package market

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Source — площадка объявлений. Каждая отвечает за свой протокол и за
// приведение цены к рублям за месяц; всё остальное — общее.
//
// Ошибка источника не рушит запрос: остальные площадки считаются, а
// упавшая попадает в ответ отдельной строкой. Один недоступный ЦИАН не
// повод не показать цифры по трём другим.
type Source interface {
	Name() string
	Fetch(ctx context.Context, q RentQuery) ([]Observation, error)
}

// Дней в месяце для пересчёта посуточной цены. Ровно 30 — не среднее по
// календарю, а то, как считают сами посуточные площадки в подписи
// «за месяц».
const daysInMonth = 30

// httpClient — общий клиент площадок. Прокси задаётся переменной
// MARKET_PROXY_URL: с адреса дата-центра площадки часто отвечают
// заглушкой, и без выхода через обычного провайдера сбор не работает.
func httpClient() *http.Client {
	tr := &http.Transport{}
	if p := os.Getenv("MARKET_PROXY_URL"); p != "" {
		if u, err := url.Parse(p); err == nil {
			tr.Proxy = http.ProxyURL(u)
		}
	}
	return &http.Client{Timeout: 10 * time.Second, Transport: tr}
}

// Заголовки обычного браузера. Без них площадки отдают страницу-заглушку
// вместо данных.
func setBrowserHeaders(r *http.Request) {
	r.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "+
		"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")
	r.Header.Set("Accept-Language", "ru-RU,ru;q=0.9")
	r.Header.Set("Accept", "application/json, text/html;q=0.9,*/*;q=0.8")
}

func fetchBody(ctx context.Context, method, addr string, body []byte) ([]byte, error) {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, addr, rd)
	if err != nil {
		return nil, err
	}
	setBrowserHeaders(req)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := httpClient().Do(req)
	if err != nil {
		// Сетевую ошибку показываем человеком, а не текстом Go: на экране
		// это читает экономист, а не разработчик.
		return nil, fmt.Errorf("площадка не отвечает")
	}
	defer resp.Body.Close()
	// Ограничение на размер: страница поиска у площадок весит мегабайты,
	// а нужны только цены.
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	switch {
	case resp.StatusCode == http.StatusOK:
	case resp.StatusCode == http.StatusUnauthorized,
		resp.StatusCode == http.StatusForbidden,
		resp.StatusCode == http.StatusTooManyRequests:
		return nil, errClosed
	case resp.StatusCode == http.StatusNotFound:
		return nil, fmt.Errorf("адрес выдачи площадки изменился — сбор не работает")
	default:
		return nil, fmt.Errorf("площадка ответила %d", resp.StatusCode)
	}
	return data, nil
}

// ── ЦИАН ──────────────────────────────────────────────────────────

type cianSource struct{}

func (cianSource) Name() string { return "ЦИАН" }

// Регионы, известные без обращения к площадке: два города, где проект
// снимает квартиры чаще всего. Остальные ищутся подсказчиком ЦИАН.
var cianRegions = map[string]int{
	"москва":          1,
	"санкт-петербург": 2,
}

func (c cianSource) Fetch(ctx context.Context, q RentQuery) ([]Observation, error) {
	region, err := c.region(ctx, q.City)
	if err != nil {
		return nil, err
	}
	jq := map[string]any{
		"_type":          "flatrent",
		"engine_version": map[string]any{"type": "term", "value": 2},
		"region":         map[string]any{"type": "terms", "value": []int{region}},
		// Аренда на длительный срок: посуточные объявления ЦИАН
		// смешивать с месячными нельзя.
		"for_day": map[string]any{"type": "term", "value": "!1"},
		"page":    map[string]any{"type": "term", "value": 1},
	}
	if q.Rooms > 0 {
		jq["room"] = map[string]any{"type": "terms", "value": []int{q.Rooms}}
	}
	body, _ := json.Marshal(map[string]any{"jsonQuery": jq})
	data, err := fetchBody(ctx,
		http.MethodPost,
		"https://api.cian.ru/search-offers/v2/search-offers-desktop/",
		body)
	if err != nil {
		return nil, err
	}

	var res struct {
		Data struct {
			OffersSerialized []struct {
				BargainTerms struct {
					PriceRur float64 `json:"priceRur"`
				} `json:"bargainTerms"`
				RoomsCount  int     `json:"roomsCount"`
				TotalArea   float64 `json:"totalArea"`
				FloorNumber int     `json:"floorNumber"`
				Building    struct {
					PassengerLiftsCount int `json:"passengerLiftsCount"`
				} `json:"building"`
				Geo struct {
					Undergrounds []struct {
						TimeToGet int `json:"time"`
					} `json:"undergrounds"`
				} `json:"geo"`
				FullURL string `json:"fullUrl"`
				Title   string `json:"title"`
			} `json:"offersSerialized"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, errUnparsed
	}
	var out []Observation
	for _, o := range res.Data.OffersSerialized {
		if o.BargainTerms.PriceRur <= 0 {
			continue
		}
		lift := o.Building.PassengerLiftsCount > 0
		metro := 0
		if len(o.Geo.Undergrounds) > 0 {
			metro = o.Geo.Undergrounds[0].TimeToGet
		}
		out = append(out, Observation{
			Source:       c.Name(),
			PriceMonth:   o.BargainTerms.PriceRur,
			Rooms:        o.RoomsCount,
			Area:         o.TotalArea,
			Floor:        o.FloorNumber,
			Elevator:     &lift,
			MetroMinutes: metro,
			Title:        o.Title,
			URL:          o.FullURL,
		})
	}
	if len(out) == 0 {
		return nil, errUnparsed
	}
	return out, nil
}

// region ищет числовой идентификатор города у подсказчика ЦИАН.
func (c cianSource) region(ctx context.Context, city string) (int, error) {
	if id, ok := cianRegions[strings.ToLower(strings.TrimSpace(city))]; ok {
		return id, nil
	}
	data, err := fetchBody(ctx, http.MethodGet,
		"https://api.cian.ru/geo-suggest/v1/suggest/?query="+url.QueryEscape(city), nil)
	if err != nil {
		return 0, err
	}
	var res struct {
		Items []struct {
			ID   int    `json:"id"`
			Text string `json:"text"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &res); err != nil || len(res.Items) == 0 {
		return 0, fmt.Errorf("город «%s» не найден", city)
	}
	return res.Items[0].ID, nil
}

// ── Яндекс Недвижимость ───────────────────────────────────────────

type yandexSource struct{}

func (yandexSource) Name() string { return "Яндекс Недвижимость" }

func (y yandexSource) Fetch(ctx context.Context, q RentQuery) ([]Observation, error) {
	slug := citySlug(q.City)
	if slug == "" {
		return nil, fmt.Errorf("город «%s» не поддерживается площадкой", q.City)
	}
	addr := fmt.Sprintf("https://realty.yandex.ru/%s/snyat/kvartira/", slug)
	// Число комнат у площадки в адресе словом. Больше четырёх отдельной
	// страницы не имеет — такие запросы идут по всей выдаче города.
	if s, ok := yaRooms[q.Rooms]; ok {
		addr += s + "/"
	}
	data, err := fetchBody(ctx, http.MethodGet, addr, nil)
	if err != nil {
		return nil, err
	}
	return parseYandex(y.Name(), data)
}

// parseYandex разбирает объявления из состояния приложения, вшитого в
// страницу выдачи. Цена, комнаты, площадь и этаж лежат в одном объекте
// объявления, поэтому признаки берутся не «в среднем по странице», а от
// конкретного объявления — модели есть на чём учиться.
//
// Опора на форму записи, а не на разметку: разметка у площадки меняется
// от выката к выкату, а поля объявления — редко.
func parseYandex(source string, body []byte) ([]Observation, error) {
	const priceKey = `"price":{"currency":"RUR","value":`
	var out []Observation
	for pos := 0; ; {
		i := bytes.Index(body[pos:], []byte(priceKey))
		if i < 0 {
			break
		}
		i += pos
		valAt := i + len(priceKey)
		price, ok := readNumber(body, valAt)
		pos = valAt
		// Месячная аренда: в той же выдаче попадаются цены за сутки и
		// за метр — они помечены другим периодом.
		if !ok || !bytes.Contains(window(body, valAt, 160), []byte(`"period":"PER_MONTH"`)) {
			continue
		}
		// Смотрим строго НАЗАД от цены: окно, залезающее вперёд, брало бы
		// признаки следующего объявления.
		back := body[max0(i-1200):i]
		o := Observation{Source: source, PriceMonth: price}
		if v, ok := lastNumberFor(back, `"roomsTotal":`); ok {
			o.Rooms = int(v)
		}
		if v, ok := lastNumberFor(back, `"area":{"value":`); ok {
			o.Area = v
		}
		if v, ok := lastNumberFor(back, `"floorsOffered":[`); ok {
			o.Floor = int(v)
		}
		out = append(out, o)
	}
	if len(out) == 0 {
		return nil, errUnparsed
	}
	return out, nil
}

// readNumber читает число, начинающееся в позиции i.
func readNumber(body []byte, i int) (float64, bool) {
	j := i
	for j < len(body) && (body[j] == '.' || (body[j] >= '0' && body[j] <= '9')) {
		j++
	}
	if j == i {
		return 0, false
	}
	v, err := strconv.ParseFloat(string(body[i:j]), 64)
	return v, err == nil
}

// lastNumberFor берёт число у ПОСЛЕДНЕГО вхождения ключа: объявление
// записано перед своей ценой, и ближайшее к ней вхождение — его.
func lastNumberFor(chunk []byte, key string) (float64, bool) {
	i := bytes.LastIndex(chunk, []byte(key))
	if i < 0 {
		return 0, false
	}
	return readNumber(chunk, i+len(key))
}

func window(body []byte, from, n int) []byte {
	if from < 0 {
		from = 0
	}
	to := from + n
	if to > len(body) {
		to = len(body)
	}
	return body[from:to]
}

func max0(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

var yaRooms = map[int]string{
	1: "odnokomnatnaya",
	2: "dvuhkomnatnaya",
	3: "tryohkomnatnaya",
	4: "chetyryohkomnatnaya",
}

// ── Суточно.ру ────────────────────────────────────────────────────

type sutochnoSource struct{}

func (sutochnoSource) Name() string { return "Суточно.ру" }

func (s sutochnoSource) Fetch(ctx context.Context, q RentQuery) ([]Observation, error) {
	addr := "https://sutochno.ru/search?" + url.Values{
		"q":        {q.City},
		"type":     {"apartment"},
		"occupied": {"1"},
	}.Encode()
	data, err := fetchBody(ctx, http.MethodGet, addr, nil)
	if err != nil {
		return nil, err
	}
	// Посуточная площадка: цена за сутки, пересчёт в месяц — в parseEmbedded.
	return parseEmbedded(s.Name(), data, true)
}

// ── Ostrovok ──────────────────────────────────────────────────────

type ostrovokSource struct{}

func (ostrovokSource) Name() string { return "Ostrovok" }

func (o ostrovokSource) Fetch(ctx context.Context, q RentQuery) ([]Observation, error) {
	addr := "https://ostrovok.ru/hotel/search/?" + url.Values{
		"q":        {q.City},
		"dates":    {""},
		"guests":   {"2"},
		"category": {"apartment"},
	}.Encode()
	data, err := fetchBody(ctx, http.MethodGet, addr, nil)
	if err != nil {
		return nil, err
	}
	return parseEmbedded(o.Name(), data, true)
}

// ── Разбор встроенных данных ──────────────────────────────────────

// errUnparsed — площадка ответила, но данными это не является: обычно
// вместо выдачи приходит страница проверки на робота. Отдельная ошибка,
// чтобы на экране это читалось не как «сеть недоступна».
var errUnparsed = fmt.Errorf("площадка не отдала объявления автоматическому запросу")

// errClosed — площадка отвечает отказом на запрос без ключа приложения
// или без браузерной сессии. Лечится ключом партнёрского API или
// выходом через MARKET_PROXY_URL, а не правкой кода.
var errClosed = fmt.Errorf("площадка закрыта для автоматических запросов — нужен ключ или прокси")

// Цены в страницах площадок лежат внутри встроенного JSON состояния
// приложения. Разметка у всех разная и меняется, а форма записи цены —
// нет: числовое поле с ценой рядом с площадью и числом комнат.
var (
	rePrice = regexp.MustCompile(`"(?:priceRur|price|priceValue|value)"\s*:\s*(\d{3,9})`)
	reArea  = regexp.MustCompile(`"(?:totalArea|area|square)"\s*:\s*(\d{1,3}(?:\.\d+)?)`)
	reRooms = regexp.MustCompile(`"(?:roomsCount|rooms|roomsTotal)"\s*:\s*(\d{1,2})`)
)

// parseEmbedded вытаскивает цены из страницы. Площадь и комнаты берутся
// «в среднем по странице»: привязать их к конкретному объявлению без
// разбора всей разметки нельзя, а для оценки уровня цен этого хватает —
// признаки всё равно усредняются пропусками.
func parseEmbedded(source string, body []byte, daily bool) ([]Observation, error) {
	prices := numbers(rePrice, body)
	if len(prices) == 0 {
		return nil, errUnparsed
	}
	area := median(numbers(reArea, body))
	rooms := int(median(numbers(reRooms, body)))

	out := make([]Observation, 0, len(prices))
	for _, p := range prices {
		if daily {
			// Суточная цена ниже месячной по абсолютной величине, но в
			// пересчёте на месяц заметно выше: признак Daily сообщает
			// модели, что это другая цена, а не выброс.
			p *= daysInMonth
		}
		if p < 5000 || p > 5_000_000 {
			// Явно не аренда квартиры: цена продажи, цена за час,
			// идентификатор, попавший под то же имя поля.
			continue
		}
		out = append(out, Observation{
			Source:     source,
			PriceMonth: p,
			Area:       area,
			Rooms:      rooms,
			Daily:      daily,
		})
	}
	if len(out) == 0 {
		return nil, errUnparsed
	}
	return out, nil
}

func numbers(re *regexp.Regexp, body []byte) []float64 {
	var out []float64
	for _, m := range re.FindAllSubmatch(body, 400) {
		v, err := strconv.ParseFloat(string(m[1]), 64)
		if err == nil && v > 0 {
			out = append(out, v)
		}
	}
	return out
}

// citySlug — адрес города в Яндекс Недвижимости. Список короткий
// намеренно: у площадки свои написания, и угаданный транслитом адрес
// отдал бы страницу другого города, а не ошибку.
var yaSlugs = map[string]string{
	"москва":          "moskva",
	"санкт-петербург": "sankt-peterburg",
	"екатеринбург":    "ekaterinburg",
	"новосибирск":     "novosibirsk",
	"казань":          "kazan",
	"нижний новгород": "nizhniy-novgorod",
	"краснодар":       "krasnodar",
	"ростов-на-дону":  "rostov-na-donu",
	"самара":          "samara",
	"уфа":             "ufa",
	"тюмень":          "tyumen",
	"мурманск":        "murmansk",
	"владивосток":     "vladivostok",
}

func citySlug(city string) string {
	return yaSlugs[strings.ToLower(strings.TrimSpace(city))]
}

func median(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	c := append([]float64(nil), v...)
	sortFloats(c)
	return percentile(c, 50)
}
