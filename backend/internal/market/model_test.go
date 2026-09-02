package market

import (
	"context"
	"math"
	"math/rand"
	"strings"
	"testing"
)

// Цена придуманного рынка: 20 000 базы, 900 за метр, 6 000 за комнату,
// минус 500 за минуту до метро. По ней проверяем, что модели ловят
// зависимость, а не запоминают выборку.
func syntheticPrice(rooms int, area float64, metro int) float64 {
	return 20000 + 900*area + 6000*float64(rooms) - 500*float64(metro)
}

func syntheticSample(n int, noise float64, seed int64) []Observation {
	rnd := rand.New(rand.NewSource(seed))
	out := make([]Observation, n)
	for i := range out {
		rooms := 1 + rnd.Intn(3)
		area := 30 + float64(rooms)*10 + rnd.Float64()*15
		metro := 3 + rnd.Intn(20)
		out[i] = Observation{
			Source:       "тест",
			Rooms:        rooms,
			Area:         area,
			MetroMinutes: metro,
			PriceMonth:   syntheticPrice(rooms, area, metro) * (1 + (rnd.Float64()-0.5)*noise),
		}
	}
	return out
}

func TestChooseModelBySampleSize(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{20, "linear_regression"},
		{forestFrom, "random_forest"},
		{boostFrom + 20, "gradient_boosting"},
	}
	for _, c := range cases {
		// Модель выбирается по обучающей части (четыре пятых выборки),
		// поэтому на вход даём с запасом.
		obs := syntheticSample(c.n*5/4+1, 0.1, 7)
		f := fitModel(obs, 1)
		if f == nil {
			t.Fatalf("n=%d: модель не построилась", c.n)
		}
		if f.name != c.want {
			t.Errorf("n=%d: выбрана %s, ожидалась %s", c.n, f.name, c.want)
		}
	}
}

func TestModelsRecoverPriceLevel(t *testing.T) {
	q := RentQuery{City: "Тестбург", Rooms: 2, Area: 55, MetroMinutes: 10}
	want := syntheticPrice(2, 55, 10)

	for _, n := range []int{30, 120, 400} {
		obs := syntheticSample(n, 0.1, 42)
		f := fitModel(obs, 1)
		if f == nil {
			t.Fatalf("n=%d: модель не построилась", n)
		}
		got := f.model.predict(f.im.apply(queryFeatures(q)))
		if rel := math.Abs(got-want) / want; rel > 0.2 {
			t.Errorf("n=%d (%s): прогноз %.0f, ожидалось около %.0f (отклонение %.0f%%)",
				n, f.name, got, want, rel*100)
		}
		if f.mae <= 0 || f.mae > want {
			t.Errorf("n=%d (%s): ошибка на отложенной выборке %.0f неправдоподобна", n, f.name, f.mae)
		}
	}
}

func TestFitModelNeedsData(t *testing.T) {
	if fitModel(syntheticSample(5, 0.1, 1), 1) != nil {
		t.Error("на пяти объявлениях модель строиться не должна")
	}
}

func TestPercentile(t *testing.T) {
	v := []float64{10, 20, 30, 40, 50}
	if got := percentile(v, 50); got != 30 {
		t.Errorf("медиана: %v", got)
	}
	if got := percentile(v, 95); math.Abs(got-48) > 1e-9 {
		t.Errorf("95-й перцентиль: %v", got)
	}
	if got := percentile(nil, 95); got != 0 {
		t.Errorf("пустая выборка: %v", got)
	}
}

func TestTrimOutliers(t *testing.T) {
	obs := make([]Observation, 0, 21)
	for i := 0; i < 20; i++ {
		obs = append(obs, Observation{PriceMonth: 50000 + float64(i)*500})
	}
	// «Квартира за рубль» и цена продажи, попавшая в аренду.
	obs = append(obs, Observation{PriceMonth: 1}, Observation{PriceMonth: 9_000_000})

	got := trimOutliers(obs)
	if len(got) != 20 {
		t.Fatalf("после отбраковки осталось %d объявлений, ожидалось 20", len(got))
	}
	for _, o := range got {
		if o.PriceMonth < 50000 || o.PriceMonth > 60000 {
			t.Errorf("выброс %v остался в выборке", o.PriceMonth)
		}
	}
}

// ── Сборка ответа ─────────────────────────────────────────────────

type stubSource struct {
	name string
	obs  []Observation
	err  error
}

func (s stubSource) Name() string { return s.name }

func (s stubSource) Fetch(context.Context, RentQuery) ([]Observation, error) {
	return s.obs, s.err
}

func TestEstimateCombinesSources(t *testing.T) {
	good := syntheticSample(60, 0.1, 3)
	for i := range good {
		good[i].Source = "площадка"
	}
	svc := NewService(
		stubSource{name: "площадка", obs: good},
		stubSource{name: "молчит", err: errUnparsed},
	)

	est, err := svc.Estimate(context.Background(), RentQuery{City: "Тестбург", Rooms: 2, Area: 55})
	if err != nil {
		t.Fatal(err)
	}
	if est.Sample == 0 || est.P50 == 0 || est.P95 <= est.P50 {
		t.Fatalf("перцентили не посчитаны: %+v", est)
	}
	if est.Recommended <= 0 || est.Recommended >= est.P95 {
		t.Errorf("в бюджет предлагается %v — должно быть среднее без верхних пяти процентов, "+
			"то есть меньше 95-го перцентиля %v", est.Recommended, est.P95)
	}
	if len(est.Sources) != 2 || est.Sources[1].Error == "" {
		t.Errorf("недоступная площадка не попала в ответ: %+v", est.Sources)
	}
	if est.Model == "" || est.Predicted == 0 {
		t.Errorf("модель не построена на 60 объявлениях: %+v", est)
	}

	// Второй запрос тех же параметров идёт из кэша: площадки не должны
	// получать по обращению на каждое нажатие кнопки.
	again, err := svc.Estimate(context.Background(), RentQuery{City: "Тестбург", Rooms: 2, Area: 55})
	if err != nil || !again.Cached {
		t.Errorf("повторный запрос не взят из кэша: %+v, %v", again, err)
	}
}

func TestEstimateWithoutData(t *testing.T) {
	svc := NewService(stubSource{name: "молчит", err: errUnparsed})
	est, err := svc.Estimate(context.Background(), RentQuery{City: "Тестбург"})
	if err != nil {
		t.Fatal(err)
	}
	if est.Sample != 0 || est.Recommended != 0 || est.Predicted != 0 {
		t.Fatalf("без объявлений оценка не должна придумываться: %+v", est)
	}
	if est.ModelReason == "" {
		t.Error("человеку не объяснили, почему цифр нет")
	}
}

func TestEstimateRequiresCity(t *testing.T) {
	if _, err := NewService(stubSource{name: "x"}).Estimate(context.Background(), RentQuery{}); err == nil {
		t.Error("город обязателен")
	}
}

// Посуточные площадки приводятся к месяцу и помечаются признаком: без
// него месячная оценка уехала бы вверх вслед за суточной ценой.
func TestDailyPricesNormalized(t *testing.T) {
	body := []byte(`<span class="price-value" data-price-currency="RUB" data-price-value="3000">` +
		`3 000</span><span data-price-value="3500.50">3 500,50</span>` +
		// Цена за час или доплата: в месяц это меньше пяти тысяч — не аренда.
		`<span data-price-value="100">100</span>`)
	obs, err := parse101("101hotels.com", body)
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 2 {
		t.Fatalf("разобрано %d цен, ожидалось 2: %+v", len(obs), obs)
	}
	if obs[0].PriceMonth != 3000*daysInMonth || !obs[0].Daily {
		t.Fatalf("суточная цена не пересчитана в месяц: %+v", obs[0])
	}
}

// Разбор выдачи Яндекс Недвижимости. Фрагмент — форма записи объявления
// в состоянии страницы: цена, комнаты, площадь и этаж лежат в одном
// объекте, и признаки должны браться от него, а не «в среднем».
func TestParseYandexOffer(t *testing.T) {
	body := []byte(`{"offers":[` +
		`{"roomsTotal":3,"floorsTotal":20,"floorsOffered":[18],` +
		`"area":{"value":110,"unit":"SQUARE_METER"},` +
		`"price":{"currency":"RUR","value":240000,"period":"PER_MONTH","valuePerPart":2182}},` +
		`{"roomsTotal":1,"floorsTotal":9,"floorsOffered":[4],` +
		`"area":{"value":35,"unit":"SQUARE_METER"},` +
		`"price":{"currency":"RUR","value":45000,"period":"PER_MONTH","valuePerPart":1285}},` +
		// Цена за сутки в той же выдаче: другой период — в выборку не идёт.
		`{"roomsTotal":2,"floorsOffered":[3],"area":{"value":50},` +
		`"price":{"currency":"RUR","value":4500,"period":"PER_DAY"}}]}`)

	obs, err := parseYandex("Яндекс Недвижимость", body)
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 2 {
		t.Fatalf("разобрано %d объявлений, ожидалось 2: %+v", len(obs), obs)
	}
	if obs[0].PriceMonth != 240000 || obs[0].Rooms != 3 || obs[0].Area != 110 || obs[0].Floor != 18 {
		t.Errorf("первое объявление разобрано неверно: %+v", obs[0])
	}
	if obs[1].PriceMonth != 45000 || obs[1].Rooms != 1 || obs[1].Area != 35 || obs[1].Floor != 4 {
		t.Errorf("второе объявление разобрано неверно: %+v", obs[1])
	}
}

func TestParseYandexEmpty(t *testing.T) {
	if _, err := parseYandex("Яндекс Недвижимость", []byte("<html>проверка робота</html>")); err == nil {
		t.Error("страница без объявлений должна давать ошибку, а не пустую выборку")
	}
}

// Посуточные объявления в расчёт не идут вовсе: суточная цена, даже
// пересчитанная в месяц, не ставка по договору найма.
func TestDailyExcludedFromEstimate(t *testing.T) {
	var obs []Observation
	for i := 0; i < 20; i++ {
		obs = append(obs, Observation{Source: "аренда", PriceMonth: 40000 + float64(i)*500})
	}
	for i := 0; i < 20; i++ {
		obs = append(obs, Observation{Source: "посуточно", PriceMonth: 130000, Daily: true})
	}
	est := build(RentQuery{City: "Тестбург"}, obs, nil)
	if est.Sample != 20 {
		t.Fatalf("в расчёт попало %d объявлений, ожидалось 20 помесячных", est.Sample)
	}
	if est.P95 > 50000 {
		t.Errorf("посуточные цены попали в перцентили: p95 = %v", est.P95)
	}
}

func TestOnlyDailyMeansNoEstimate(t *testing.T) {
	var obs []Observation
	for i := 0; i < 20; i++ {
		obs = append(obs, Observation{Source: "посуточно", PriceMonth: 130000, Daily: true})
	}
	est := build(RentQuery{City: "Тестбург"}, obs, nil)
	if est.Sample != 0 || est.Recommended != 0 {
		t.Fatalf("оценка построена на одних посуточных: %+v", est)
	}
	if !strings.Contains(est.ModelReason, "помесячной") {
		t.Errorf("человеку не объяснили, почему цифр нет: %q", est.ModelReason)
	}
}

// В бюджет идёт среднее по выборке без верхних пяти процентов, а не сам
// 95-й перцентиль: тот — почти самое дорогое предложение рынка.
func TestRecommendedIsMeanBelowP95(t *testing.T) {
	// Ровный ряд от 30 000 до 49 000: 95-й перцентиль около 49 000,
	// среднее по выборке без верхушки — около 39 000.
	var obs []Observation
	for i := 0; i < 20; i++ {
		obs = append(obs, Observation{Source: "аренда", PriceMonth: 30000 + float64(i)*1000})
	}

	est := build(RentQuery{City: "Тестбург"}, obs, nil)
	if est.Recommended >= est.P95 {
		t.Errorf("в бюджет предлагается %v — не меньше 95-го перцентиля %v",
			est.Recommended, est.P95)
	}
	if est.Recommended < 38000 || est.Recommended > 40000 {
		t.Errorf("среднее по выборке без верхушки: %v, ожидалось около 39 000", est.Recommended)
	}
}

// График распределения: столбики покрывают всю выборку и ничего не
// теряют — самое дорогое объявление попадает в последний столбик.
func TestHistogramCoversSample(t *testing.T) {
	var obs []Observation
	for i := 0; i < 50; i++ {
		obs = append(obs, Observation{Source: "аренда", PriceMonth: 30000 + float64(i)*1000})
	}
	est := build(RentQuery{City: "Тестбург"}, obs, nil)
	total := 0
	for _, b := range est.Histogram {
		total += b.Count
		if b.To < b.From {
			t.Errorf("диапазон столбика перевёрнут: %+v", b)
		}
	}
	if total != est.Sample {
		t.Errorf("в столбиках %d объявлений, в выборке %d", total, est.Sample)
	}
}

// В примерах — только объявления, где заполнено всё, что в них выведено.
func TestExamplesAreComplete(t *testing.T) {
	obs := []Observation{
		{Source: "аренда", PriceMonth: 40000, Rooms: 1, Area: 35},
		{Source: "аренда", PriceMonth: 45000},
		{Source: "аренда", PriceMonth: 50000, Rooms: 2},
	}
	for _, e := range examples(obs) {
		if e.Rooms == 0 || e.Area == 0 || e.PriceMonth == 0 {
			t.Errorf("в примерах объявление с пустыми полями: %+v", e)
		}
	}
}
