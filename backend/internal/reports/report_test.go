package reports

import (
	"math"
	"testing"
	"time"

	"ibcon-budget/internal/calc"
)

func date(y int, m time.Month) time.Time {
	return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
}

// resWith — результат расчёта на n месяцев с одинаковым заполнением.
func resWith(n int, fill func(m *calc.MonthlyResult)) *calc.CalcResult {
	res := &calc.CalcResult{DurationMonths: n, Monthly: make([]calc.MonthlyResult, n)}
	for i := range res.Monthly {
		res.Monthly[i].Month = i + 1
		fill(&res.Monthly[i])
	}
	return res
}

func rf(start time.Time) Params {
	return Params{StartDate: start, ExecutorName: calc.ExecutorAibicon}
}

func row(t *testing.T, rep *Report, code string) Row {
	t.Helper()
	for _, r := range rep.Rows {
		if r.Code == code {
			return r
		}
	}
	t.Fatalf("строка %s не найдена в отчёте %s", code, rep.Kind)
	return Row{}
}

// Горизонт отчёта — длительность проекта, а не жёсткие 12 месяцев формы.
func TestBuild_HorizonFollowsProject(t *testing.T) {
	for _, kind := range []Kind{KindBDR, KindBDDS} {
		for _, n := range []int{3, 12, 18} {
			rep := Build(kind, resWith(n, func(m *calc.MonthlyResult) {}), rf(date(2027, time.January)))
			if rep.Months != n || len(rep.MonthLabels) != n {
				t.Errorf("%s, проект на %d мес.: Months=%d, подписей %d",
					kind, n, rep.Months, len(rep.MonthLabels))
			}
			for _, r := range rep.Rows {
				if len(r.Monthly) != n {
					t.Fatalf("%s, строка %s: %d месяцев вместо %d", kind, r.Code, len(r.Monthly), n)
				}
			}
		}
	}
}

// Групповые строки собирают сумму вложенных, внуки не задваиваются.
func TestBuild_RollUp(t *testing.T) {
	rep := Build(KindBDR, resWith(2, func(m *calc.MonthlyResult) {
		m.Overhead[182-178] = 100 // аренда офиса → 2.2.1.01
		m.Overhead[196-178] = 20  // канцтовары   → 2.2.1.02
		m.Overhead[208-178] = 3   // коммунальные → 2.2.1.04
		m.Overhead[183-178] = 7   // уборка офиса → 2.2.1.06
	}), rf(date(2027, time.January)))

	g := row(t, rep, "2.2.1")
	if !g.Group {
		t.Error("2.2.1 должна быть групповой строкой")
	}
	if g.Monthly[0] != 130 || g.Total != 260 {
		t.Errorf("2.2.1: месяц 1 = %.2f (want 130), итого = %.2f (want 260)", g.Monthly[0], g.Total)
	}
	if s := row(t, rep, "2").Total; s != 260 {
		t.Errorf("2 «Себестоимость»: want 260, got %.2f", s)
	}
	if s := row(t, rep, "2.2").Total; s != 260 {
		t.Errorf("2.2 «Накладные»: want 260, got %.2f", s)
	}
}

// ── Исправления багов формы ─────────────────────────────────────────────

// Взносы Киргизии не теряются: в форме строка 174 не учтена нигде.
func TestKGInsuranceNotLost(t *testing.T) {
	fill := func(m *calc.MonthlyResult) {
		m.InsuranceRF = 1_000
		m.InsuranceKG = 500
	}
	start := date(2027, time.January)

	// БДР — своим месяцем.
	bdr := Build(KindBDR, resWith(3, fill), rf(start))
	for m := 0; m < 3; m++ {
		if v := row(t, bdr, "2.1.3").Monthly[m]; v != 1_500 {
			t.Errorf("БДР, взносы, месяц %d: want 1 500, got %.2f", m+1, v)
		}
	}

	// БДДС — со сдвигом на месяц: в первом месяце ничего.
	bdds := Build(KindBDDS, resWith(3, fill), rf(start))
	got := row(t, bdds, "2.1.1.06").Monthly
	if got[0] != 0 || got[1] != 1_500 || got[2] != 1_500 {
		t.Errorf("БДДС, взносы: want [0 1500 1500], got %v", got)
	}
}

// Охрана объекта (209) не теряется, мебель (195) не задваивается.
func TestSecurityAndFurnitureFixed(t *testing.T) {
	fill := func(m *calc.MonthlyResult) {
		m.Overhead[195-178] = 274_500
		m.Overhead[209-178] = 108_000
	}
	start := date(2027, time.January)

	bdr := Build(KindBDR, resWith(1, fill), rf(start))
	if v := row(t, bdr, "2.2.4.07").Total; v != 274_500 {
		t.Errorf("БДР, мебель: want 274 500, got %.2f", v)
	}
	if v := row(t, bdr, "2.2.7.09").Total; v != 108_000 {
		t.Errorf("БДР, охрана объекта: want 108 000, got %.2f", v)
	}
	if v := row(t, bdr, "2").Total; v != 382_500 {
		t.Errorf("БДР, себестоимость: want 382 500, got %.2f", v)
	}

	bdds := Build(KindBDDS, resWith(1, fill), rf(start))
	if v := row(t, bdds, "2.2.1.02").Total; v != 274_500 {
		t.Errorf("БДДС, мебель: want 274 500, got %.2f", v)
	}
	if v := row(t, bdds, "2.2.7.09").Total; v != 108_000 {
		t.Errorf("БДДС, охрана объекта: want 108 000, got %.2f", v)
	}
	if v := row(t, bdds, "2").Total; v != 382_500 {
		t.Errorf("БДДС, расходы: want 382 500, got %.2f", v)
	}
}

// Корпоративы (205) в БДДС берутся с НДС и не съезжают по месяцам:
// в форме диапазон был записан без $ и при копировании вправо сдвигался.
func TestBDDS_CorporateEventsVATNoDrift(t *testing.T) {
	res := resWith(4, func(m *calc.MonthlyResult) {})
	// Разные суммы по месяцам — съехавшая ссылка сразу бы это показала.
	for i, v := range []float64{100, 200, 300, 400} {
		res.Monthly[i].Overhead[205-178] = v
	}
	rep := Build(KindBDDS, res, rf(date(2027, time.January)))
	got := row(t, rep, "2.2.2.13").Monthly
	want := []float64{122, 244, 366, 488}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 0.01 {
			t.Errorf("корпоративы, месяц %d: want %.2f, got %.2f", i+1, want[i], got[i])
		}
	}
}

// Пять статей БДДС берутся с НДС, в БДР — без него.
func TestBDDS_VATArticles(t *testing.T) {
	cases := []struct {
		budgetRow int
		bddsCode  string
		bdrCode   string
	}{
		{192, "2.2.2.08", "2.2.2.07"}, // спецодежда
		{205, "2.2.2.13", "2.2.2.09"}, // корпоративы
		{186, "2.2.3.01", "2.2.3.02"}, // интернет
		{187, "2.2.3.02", "2.2.3.04"}, // мобильная связь
		{198, "2.2.5.03", "2.2.5.04"}, // топливо / ГСМ
	}
	start := date(2027, time.January)
	for _, c := range cases {
		res := resWith(1, func(m *calc.MonthlyResult) {})
		res.Monthly[0].Overhead[c.budgetRow-178] = 1_000

		bdds := Build(KindBDDS, res, rf(start))
		if v := row(t, bdds, c.bddsCode).Total; math.Abs(v-1_220) > 0.01 {
			t.Errorf("БДДС, строка %d: want 1 220 (с НДС), got %.2f", c.budgetRow, v)
		}
		bdr := Build(KindBDR, res, rf(start))
		if v := row(t, bdr, c.bdrCode).Total; v != 1_000 {
			t.Errorf("БДР, строка %d: want 1 000 (без НДС), got %.2f", c.budgetRow, v)
		}
	}
}

// ── Сдвиги платежей в БДДС ──────────────────────────────────────────────

// Зарплата и НДФЛ платятся дважды в месяц: половина за текущий месяц,
// половина за предыдущий.
func TestBDDS_SalaryHalfAndHalf(t *testing.T) {
	res := resWith(3, func(m *calc.MonthlyResult) {})
	for i, v := range []float64{100, 200, 400} {
		res.Monthly[i].FOT = v
		res.Monthly[i].NDFL = v / 10
	}
	rep := Build(KindBDDS, res, rf(date(2027, time.January)))

	// 100/2 | 100/2+200/2 | 200/2+400/2
	wantSalary := []float64{50, 150, 300}
	got := row(t, rep, "2.1.1.01").Monthly
	for i := range wantSalary {
		if math.Abs(got[i]-wantSalary[i]) > 0.01 {
			t.Errorf("зарплата, месяц %d: want %.2f, got %.2f", i+1, wantSalary[i], got[i])
		}
	}

	wantNDFL := []float64{5, 15, 30}
	got = row(t, rep, "2.1.1.03").Monthly
	for i := range wantNDFL {
		if math.Abs(got[i]-wantNDFL[i]) > 0.01 {
			t.Errorf("НДФЛ, месяц %d: want %.2f, got %.2f", i+1, wantNDFL[i], got[i])
		}
	}

	// В БДР та же зарплата — своим месяцем и целиком (плюс НДФЛ).
	bdr := Build(KindBDR, res, rf(date(2027, time.January)))
	if v := row(t, bdr, "2.1.2").Monthly[0]; math.Abs(v-110) > 0.01 {
		t.Errorf("БДР, ФОТ месяц 1: want 110, got %.2f", v)
	}
}

// Выручка: у киргизского филиала со сдвигом на месяц, у российских — нет.
func TestBDDS_RevenueShiftOnlyForKG(t *testing.T) {
	res := resWith(3, func(m *calc.MonthlyResult) {})
	for i, v := range []float64{10, 20, 30} {
		res.Monthly[i].Revenue = v
	}
	start := date(2027, time.January)

	ru := Build(KindBDDS, res, Params{StartDate: start, ExecutorName: calc.ExecutorAibicon})
	if got := row(t, ru, "1.1").Monthly; got[0] != 10 || got[1] != 20 || got[2] != 30 {
		t.Errorf("выручка РФ: want [10 20 30], got %v", got)
	}

	kg := Build(KindBDDS, res, Params{StartDate: start, ExecutorName: calc.ExecutorAibiconKG})
	if got := row(t, kg, "1.1").Monthly; got[0] != 0 || got[1] != 10 || got[2] != 20 {
		t.Errorf("выручка КГ: want [0 10 20], got %v", got)
	}

	// В БДР сдвига нет ни у кого.
	bdr := Build(KindBDR, res, Params{StartDate: start, ExecutorName: calc.ExecutorAibiconKG})
	if got := row(t, bdr, "1.1").Monthly; got[0] != 10 || got[2] != 30 {
		t.Errorf("БДР, выручка КГ: сдвига быть не должно, got %v", got)
	}
}

// ── Налог на прибыль ────────────────────────────────────────────────────

// БДР: платёж в последнем месяце календарного квартала, база — прибыль
// этого квартала. Кварталы календарные, а не «каждые три месяца проекта».
func TestBDR_QuarterlyProfitTax(t *testing.T) {
	// Старт февраль 2027, 12 месяцев: фев…янв 2028.
	res := resWith(12, func(m *calc.MonthlyResult) { m.OperatingProfit = 100 })
	res.ProfitTaxRate = 0.25
	rep := Build(KindBDR, res, rf(date(2027, time.February)))
	got := row(t, rep, "2.2.8.01").Monthly

	// I квартал: в проекте только февраль и март → 200 × 0.25 = 50,
	// платёж в марте (индекс 1).
	if math.Abs(got[1]-50) > 0.01 {
		t.Errorf("март (I кв., 2 месяца в проекте): want 50, got %.2f", got[1])
	}
	// II квартал: апрель-июнь целиком → 300 × 0.25 = 75, платёж в июне.
	if math.Abs(got[4]-75) > 0.01 {
		t.Errorf("июнь (II кв.): want 75, got %.2f", got[4])
	}
	// III квартал — сентябрь, IV — декабрь.
	if math.Abs(got[7]-75) > 0.01 {
		t.Errorf("сентябрь (III кв.): want 75, got %.2f", got[7])
	}
	if math.Abs(got[10]-75) > 0.01 {
		t.Errorf("декабрь (IV кв.): want 75, got %.2f", got[10])
	}
	// В остальных месяцах налога нет.
	for _, i := range []int{0, 2, 3, 5, 6, 8, 9, 11} {
		if got[i] != 0 {
			t.Errorf("месяц %d должен быть без налога, got %.2f", i+1, got[i])
		}
	}
}

// БДДС: тот же налог, но месяцем позже — апрель, июль, октябрь, а за
// IV квартал в марте СЛЕДУЮЩЕГО года.
func TestBDDS_QuarterlyProfitTaxShifted(t *testing.T) {
	// Старт январь 2027, 15 месяцев: янв 2027 … мар 2028.
	res := resWith(15, func(m *calc.MonthlyResult) { m.OperatingProfit = 100 })
	res.ProfitTaxRate = 0.25
	rep := Build(KindBDDS, res, rf(date(2027, time.January)))
	got := row(t, rep, "2.2.8.01").Monthly

	// I квартал 2027 (300) платится в апреле — индекс 3.
	if math.Abs(got[3]-75) > 0.01 {
		t.Errorf("апрель (за I кв.): want 75, got %.2f", got[3])
	}
	if math.Abs(got[6]-75) > 0.01 {
		t.Errorf("июль (за II кв.): want 75, got %.2f", got[6])
	}
	if math.Abs(got[9]-75) > 0.01 {
		t.Errorf("октябрь (за III кв.): want 75, got %.2f", got[9])
	}
	// IV квартал 2027 платится в марте 2028 — индекс 14.
	if math.Abs(got[14]-75) > 0.01 {
		t.Errorf("март следующего года (за IV кв.): want 75, got %.2f", got[14])
	}
	// В марте 2027 налога нет: IV квартала 2026 в проекте не было.
	if got[2] != 0 {
		t.Errorf("март первого года: налога быть не должно, got %.2f", got[2])
	}

	// Сдвиг переносит деньги, а не меняет их, но последний квартал в горизонт
	// уже не попадает: I квартал 2028 (янв-мар) БДР начисляет в марте 2028 —
	// последнем месяце проекта, — а БДДС платил бы в апреле, то есть после его
	// окончания.
	bdr := Build(KindBDR, res, rf(date(2027, time.January)))
	sBDR := row(t, bdr, "2.2.8.01").Total
	sBDDS := row(t, rep, "2.2.8.01").Total
	lastQuarter := 300 * 0.25 // янв-мар 2028, по 100 прибыли в месяц
	if math.Abs(sBDR-sBDDS-lastQuarter) > 0.01 {
		t.Errorf("итог налога: БДР %.2f, БДДС %.2f — разница должна быть %.2f",
			sBDR, sBDDS, lastQuarter)
	}
}

// Нулевая ставка (Айбикон-Проект) — налога нет вовсе.
func TestProfitTax_ZeroRate(t *testing.T) {
	res := resWith(12, func(m *calc.MonthlyResult) { m.OperatingProfit = 1_000 })
	res.ProfitTaxRate = 0
	for _, kind := range []Kind{KindBDR, KindBDDS} {
		rep := Build(kind, res, rf(date(2027, time.January)))
		if v := row(t, rep, "2.2.8.01").Total; v != 0 {
			t.Errorf("%s: при нулевой ставке налог должен быть 0, got %.2f", kind, v)
		}
	}
}

// ── Сплошная проверка карты статей ──────────────────────────────────────

// Каждая заполняемая строка 2.Бюджет 178-211 должна попасть в расходы
// ровно один раз — иначе сумма потеряется или задвоится. Проверяем оба
// отчёта: подставляем в одну строку значение и смотрим итог «2».
func TestEveryOverheadRowMappedOnce(t *testing.T) {
	start := date(2027, time.January)
	// Пять статей БДДС берутся с НДС — для них ожидаем 1 220.
	vatRows := map[int]bool{186: true, 187: true, 192: true, 198: true, 205: true}

	for budgetRow := 178; budgetRow <= 211; budgetRow++ {
		idx := budgetRow - 178
		res := resWith(1, func(m *calc.MonthlyResult) {})
		res.Monthly[0].Overhead[idx] = 1_000

		if got := row(t, Build(KindBDR, res, rf(start)), "2").Total; math.Abs(got-1_000) > 0.01 {
			t.Errorf("БДР, строка 2.Бюджет!%d: в расходы попало %.2f вместо 1 000", budgetRow, got)
		}

		want := 1_000.0
		if vatRows[budgetRow] {
			want = 1_220
		}
		if got := row(t, Build(KindBDDS, res, rf(start)), "2").Total; math.Abs(got-want) > 0.01 {
			t.Errorf("БДДС, строка 2.Бюджет!%d: в расходы попало %.2f вместо %.0f",
				budgetRow, got, want)
		}
	}
}

// Коды статей внутри одного отчёта не повторяются: дубль тихо ломает
// сборку групповых сумм.
func TestCodesAreUnique(t *testing.T) {
	for _, arts := range [][]article{bdrArticles(), bddsArticles()} {
		seen := map[string]bool{}
		for _, a := range arts {
			if seen[a.code] {
				t.Errorf("код %s (%s) встречается дважды", a.code, a.name)
			}
			seen[a.code] = true
		}
	}
}

// У каждой статьи с источником должен быть родитель в кодификаторе,
// иначе её сумма не поднимется в итог отчёта и потеряется.
func TestEveryFilledArticleHasParent(t *testing.T) {
	for _, arts := range [][]article{bdrArticles(), bddsArticles()} {
		codes := map[string]bool{}
		for _, a := range arts {
			codes[a.code] = true
		}
		for _, a := range arts {
			if a.src == nil || codeLevel(a.code) == 0 {
				continue
			}
			parent := a.code[:lastDot(a.code)]
			if !codes[parent] {
				t.Errorf("у статьи %s (%s) нет родителя %s", a.code, a.name, parent)
			}
		}
	}
}

func lastDot(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return i
		}
	}
	return 0
}

// ── Календарь ───────────────────────────────────────────────────────────

// Проект, начатый в конце месяца, не должен терять месяцы: AddDate
// переполняет короткие месяцы, и старт 31 августа давал «31 сентября» →
// 1 октября, то есть сентябрь выпадал из отчёта.
func TestBuild_MonthLabelsNoOverflow(t *testing.T) {
	start := time.Date(2026, time.August, 31, 0, 0, 0, 0, time.UTC)
	rep := Build(KindBDR, resWith(6, func(m *calc.MonthlyResult) {}),
		Params{StartDate: start, ExecutorName: calc.ExecutorAibicon})

	want := []string{"авг. 26", "сен. 26", "окт. 26", "ноя. 26", "дек. 26", "янв. 27"}
	for i := range want {
		if rep.MonthLabels[i] != want[i] {
			t.Errorf("месяц %d: want %q, got %q", i+1, want[i], rep.MonthLabels[i])
		}
	}
}

// Тот же перекос ломал и квартальный налог: месяцы уезжали в чужие
// кварталы. Старт 31 января — налог платится в марте.
func TestProfitTax_NoMonthOverflow(t *testing.T) {
	start := time.Date(2027, time.January, 31, 0, 0, 0, 0, time.UTC)
	res := resWith(6, func(m *calc.MonthlyResult) { m.OperatingProfit = 100 })
	res.ProfitTaxRate = 0.25

	rep := Build(KindBDR, res, Params{StartDate: start, ExecutorName: calc.ExecutorAibicon})
	got := row(t, rep, "2.2.8.01").Monthly
	// Январь-март в проекте — три месяца, платёж в марте (индекс 2).
	if math.Abs(got[2]-75) > 0.01 {
		t.Errorf("март: want 75, got %.2f", got[2])
	}
	if got[0] != 0 || got[1] != 0 {
		t.Errorf("январь и февраль должны быть без налога, got %v", got[:2])
	}
}

// ── Ручные статьи ───────────────────────────────────────────────────────

// Статью, которую платформа не считает, заполняют руками прямо в отчёте.
// Её значение попадает в отчёт и поднимается в групповые суммы наравне с
// расчётными.
func TestBuild_ManualValues(t *testing.T) {
	start := date(2027, time.January)
	res := resWith(3, func(m *calc.MonthlyResult) {})

	p := rf(start)
	// 2.2.1.05 «Ремонт офиса» — ручная статья группы «Аренда и содержание
	// офиса»; 2.2.1.01 «Аренда (офис)» в той же группе считается.
	p.Manual = ManualValues{BDR: map[string][]float64{
		"2.2.1.05": {10, 20, 30},
	}}
	rep := Build(KindBDR, res, p)

	r := row(t, rep, "2.2.1.05")
	if !r.Manual {
		t.Error("статья без источника должна быть помечена как ручная")
	}
	if r.Total != 60 {
		t.Errorf("ручная статья: want 60, got %.2f", r.Total)
	}
	// Поднялась в группу и в итог отчёта.
	if v := row(t, rep, "2.2.1").Total; v != 60 {
		t.Errorf("группа: want 60, got %.2f", v)
	}
	if v := row(t, rep, "2").Total; v != 60 {
		t.Errorf("себестоимость: want 60, got %.2f", v)
	}
}

// Ручные значения одного отчёта не протекают в другой: одна и та же
// статья может быть расчётной в БДР и ручной в БДДС.
func TestBuild_ManualValuesPerKind(t *testing.T) {
	start := date(2027, time.January)
	res := resWith(2, func(m *calc.MonthlyResult) {})

	p := rf(start)
	p.Manual = ManualValues{BDDS: map[string][]float64{"2.2.1.03": {5, 5}}}

	if v := row(t, Build(KindBDDS, res, p), "2.2.1.03").Total; v != 10 {
		t.Errorf("БДДС: want 10, got %.2f", v)
	}
	// В БДР код 2.2.1.03 — это другая статья, и ручных значений ей не
	// задавали.
	if v := row(t, Build(KindBDR, res, p), "2.2.1.03").Total; v != 0 {
		t.Errorf("БДР не должен видеть ручные значения БДДС, got %.2f", v)
	}
}

// Массив короче горизонта (проект продлили после ввода) не должен ронять
// сборку — недостающие месяцы остаются нулями, лишние отбрасываются.
func TestBuild_ManualValuesLengthMismatch(t *testing.T) {
	start := date(2027, time.January)
	res := resWith(3, func(m *calc.MonthlyResult) {})

	p := rf(start)
	p.Manual = ManualValues{BDR: map[string][]float64{
		"2.2.1.05": {7},             // короче горизонта
		"2.2.1.03": {1, 2, 3, 4, 5}, // длиннее
	}}
	rep := Build(KindBDR, res, p)

	if got := row(t, rep, "2.2.1.05").Monthly; got[0] != 7 || got[1] != 0 || got[2] != 0 {
		t.Errorf("короткий массив: want [7 0 0], got %v", got)
	}
	if v := row(t, rep, "2.2.1.03").Total; v != 6 {
		t.Errorf("длинный массив: лишние месяцы должны отбрасываться, got %.2f", v)
	}
}

// Групповая строка — та, у которой есть потомки, а не любая строка без
// источника: ручные статьи тоже без источника, но заполняются, а не
// собираются снизу.
func TestBuild_GroupVsManual(t *testing.T) {
	rep := Build(KindBDR, resWith(1, func(m *calc.MonthlyResult) {}), rf(date(2027, time.January)))

	g := row(t, rep, "2.2.1") // есть потомки
	if !g.Group || g.Manual {
		t.Errorf("2.2.1: want group, got group=%v manual=%v", g.Group, g.Manual)
	}
	m := row(t, rep, "2.2.1.05") // потомков нет, источника нет
	if m.Group || !m.Manual {
		t.Errorf("2.2.1.05: want manual, got group=%v manual=%v", m.Group, m.Manual)
	}
	c := row(t, rep, "2.2.1.01") // считается
	if c.Group || c.Manual {
		t.Errorf("2.2.1.01: want calculated, got group=%v manual=%v", c.Group, c.Manual)
	}
}
