package budgets

import (
	"bytes"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"ibcon-budget/internal/calc"
	"ibcon-budget/internal/reports"
)

func testMeta(limited bool) ExportMeta {
	return ExportMeta{
		ProjectID:      7,
		ProjectName:    "Мост",
		Customer:       "Заказчик",
		ExecutorName:   calc.ExecutorAibicon,
		VersionNo:      2,
		Status:         "draft",
		StartDate:      time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC),
		DurationMonths: 3,
		Limited:        limited,
	}
}

func testResult() *calc.CalcResult {
	res := &calc.CalcResult{DurationMonths: 3, Monthly: make([]calc.MonthlyResult, 3)}
	for i := range res.Monthly {
		m := &res.Monthly[i]
		m.Month = i + 1
		m.FOT = 100
		m.NDFL = 20
		m.InsuranceRF = 30
		m.TotalFOT = 150
		m.Overhead[182-178] = 40 // аренда офиса
		m.ProjectCostsExFOT = 40
		m.Unpredictables = 5
		m.Revenue = 1_000
	}
	res.TotalRevenue = 3_000
	res.TotalFOT = 450
	return res
}

func openBook(t *testing.T, data []byte) *excelize.File {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("книга не читается: %v", err)
	}
	return f
}

// Полная выгрузка — три листа в одной книге: экономисту нужны бюджет,
// БДР и БДДС сразу, разносить их по файлам незачем.
func TestBuildExport_ThreeSheets(t *testing.T) {
	res := testResult()
	p := reports.Params{
		StartDate:    testMeta(false).StartDate,
		ExecutorName: calc.ExecutorAibicon,
	}
	data, err := BuildExport(testMeta(false), res,
		reports.Build(reports.KindBDR, res, p),
		reports.Build(reports.KindBDDS, res, p),
	)
	if err != nil {
		t.Fatalf("сборка книги: %v", err)
	}

	f := openBook(t, data)
	defer f.Close()

	sheets := f.GetSheetList()
	want := []string{"Бюджет", "БДР", "БДДС"}
	if len(sheets) != len(want) {
		t.Fatalf("листы книги: %v, ожидались %v", sheets, want)
	}
	for i := range want {
		if sheets[i] != want[i] {
			t.Errorf("лист %d: %q, ожидался %q", i, sheets[i], want[i])
		}
	}
}

// Книга администратора проекта: один лист, строки 178-212 и ничего сверх
// того — ни ФОТ, ни выручки, ни прибыли, ни отчётов.
func TestBuildExport_LimitedForAdmin(t *testing.T) {
	res := testResult()
	p := reports.Params{StartDate: testMeta(true).StartDate, ExecutorName: calc.ExecutorAibicon}

	// Отчёты передаём намеренно: урезанная книга обязана их проигнорировать.
	data, err := BuildExport(testMeta(true), res,
		reports.Build(reports.KindBDR, res, p),
		reports.Build(reports.KindBDDS, res, p),
	)
	if err != nil {
		t.Fatalf("сборка книги: %v", err)
	}

	f := openBook(t, data)
	defer f.Close()

	if sheets := f.GetSheetList(); len(sheets) != 1 || sheets[0] != "Бюджет" {
		t.Fatalf("книга администратора: листы %v, ожидался один «Бюджет»", sheets)
	}

	rows, err := f.GetRows("Бюджет")
	if err != nil {
		t.Fatalf("чтение листа: %v", err)
	}
	var text string
	for _, r := range rows {
		for _, c := range r {
			text += c + "\n"
		}
	}

	// Граница выгрузки — строка 212. Всё, что дальше, администратору не
	// показывается: и непредвиденные (214), и АУП (215).
	mustHave := []string{"212 Итого накладные расходы", "182 Аренда офиса"}
	for _, want := range mustHave {
		if !contains(text, want) {
			t.Errorf("в книге администратора нет строки %q", want)
		}
	}
	mustNotHave := []string{
		"Непредвиденные", "АУП", "Выручка", "Чистая прибыль",
		"Рентабельность", "ФОТ", "Налог на прибыль",
	}
	for _, bad := range mustNotHave {
		if contains(text, bad) {
			t.Errorf("в книге администратора не должно быть %q", bad)
		}
	}
}

// Шрифт книги — тот же Aptos Narrow, что на листах БДР и БДДС: они лежат
// рядом в одном файле, и разнобой был бы заметен сразу.
func TestBuildExport_Font(t *testing.T) {
	res := testResult()
	data, err := BuildExport(testMeta(false), res)
	if err != nil {
		t.Fatalf("сборка книги: %v", err)
	}
	f := openBook(t, data)
	defer f.Close()

	rows, err := f.GetRows("Бюджет")
	if err != nil {
		t.Fatalf("чтение листа: %v", err)
	}
	// Ищем строку «Чистая прибыль» — она должна быть жирной.
	found := false
	for i, r := range rows {
		if len(r) == 0 || r[0] != "Чистая прибыль" {
			continue
		}
		found = true
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		sid, err := f.GetCellStyle("Бюджет", cell)
		if err != nil {
			t.Fatalf("стиль ячейки: %v", err)
		}
		st, err := f.GetStyle(sid)
		if err != nil {
			t.Fatalf("чтение стиля: %v", err)
		}
		if st.Font == nil || st.Font.Family != reports.FontName || !st.Font.Bold {
			t.Errorf("«Чистая прибыль»: ожидался жирный %s, got %+v", reports.FontName, st.Font)
		}
	}
	if !found {
		t.Error("строка «Чистая прибыль» не найдена в выгрузке")
	}
}

func contains(haystack, needle string) bool {
	return bytes.Contains([]byte(haystack), []byte(needle))
}

// Помесячная разбивка содержит маржинальность и справочную строку 249.
func TestBuildExport_MonthlyRows(t *testing.T) {
	res := testResult()
	for i := range res.Monthly {
		res.Monthly[i].MarginAmount = 200
		res.Monthly[i].RefRateAmount = 50
	}
	data, err := BuildExport(testMeta(false), res)
	if err != nil {
		t.Fatalf("сборка книги: %v", err)
	}
	f := openBook(t, data)
	defer f.Close()

	// GetRows отдаёт значения уже по формату ячейки: money — два знака.
	want := map[string]string{
		"234 Операционная маржинальность":         "200.00",
		"249 Стоимость + ставка рефинансирования": "50.00",
	}
	rows, _ := f.GetRows("Бюджет")
	found := map[string]bool{}
	for _, r := range rows {
		if len(r) < 2 {
			continue
		}
		if v, ok := want[r[0]]; ok {
			found[r[0]] = true
			if r[1] != v {
				t.Errorf("%s: первый месяц %q, ожидалось %q", r[0], r[1], v)
			}
		}
	}
	for label := range want {
		if !found[label] {
			t.Errorf("строка %q не найдена в выгрузке", label)
		}
	}
}

// Блок банковских гарантий показывает условия, а не только сумму: по
// одной сумме не понять, из чего она вышла.
func TestBuildExport_BankGuaranteeDetails(t *testing.T) {
	res := testResult()
	for i := range res.Monthly {
		res.Monthly[i].BGExecution = 10
	}
	meta := testMeta(false)
	meta.Params = &calc.InputBudgetParams{
		BGExecution: calc.BankGuarantee{
			Pct: 30, RatePct: 5, RateType: calc.BGRatePerYear, DurationMos: 12,
		},
	}

	data, err := BuildExport(meta, res)
	if err != nil {
		t.Fatalf("сборка книги: %v", err)
	}
	f := openBook(t, data)
	defer f.Close()

	rows, _ := f.GetRows("Бюджет")
	var line []string
	for _, r := range rows {
		if len(r) > 0 && r[0] == "На исполнение обязательств" {
			line = r
		}
	}
	if line == nil {
		t.Fatal("строка «На исполнение обязательств» не найдена")
	}
	// % от договора, ставка, режим, срок, сумма за проект (10 × 3 месяца).
	// Сумма за проект — денежная ячейка, поэтому с двумя знаками;
	// условия гарантии пишутся как есть.
	want := []string{"На исполнение обязательств", "30", "5", "%/год", "12", "30.00"}
	for i := range want {
		if i >= len(line) || line[i] != want[i] {
			t.Errorf("колонка %d: got %q, want %q", i, safeAt(line, i), want[i])
		}
	}
}

// Финальная строка-вывод: тот же вердикт и та же заливка, что на экранах.
func TestBuildExport_Verdict(t *testing.T) {
	cases := []struct {
		profitability float64
		label         string
		color         string
	}{
		{35, "Сверхприбыльный", "2F7D3A"},
		{25, "Высокорентабельный", "4E9455"},
		{10, "Среднерентабельный", "B07A12"},
		{3, "Низкорентабельный", "A8621A"},
		{0.5, "Порог рентабельности", "8E4A2A"},
		{-4, "Убыточный", "9C2B2B"},
	}

	for _, c := range cases {
		res := testResult()
		res.Profitability = c.profitability

		data, err := BuildExport(testMeta(false), res)
		if err != nil {
			t.Fatalf("сборка книги: %v", err)
		}
		f := openBook(t, data)

		rows, _ := f.GetRows("Бюджет")
		found := -1
		for i, r := range rows {
			if len(r) > 0 && contains(r[0], c.label) {
				found = i
			}
		}
		if found < 0 {
			f.Close()
			t.Errorf("рентабельность %.1f: вердикт %q не найден", c.profitability, c.label)
			continue
		}

		cell, _ := excelize.CoordinatesToCellName(1, found+1)
		sid, _ := f.GetCellStyle("Бюджет", cell)
		st, err := f.GetStyle(sid)
		if err != nil {
			f.Close()
			t.Fatalf("чтение стиля: %v", err)
		}
		if len(st.Fill.Color) == 0 || st.Fill.Color[0] != c.color {
			t.Errorf("рентабельность %.1f: заливка %v, ожидалась %s",
				c.profitability, st.Fill.Color, c.color)
		}
		f.Close()
	}
}

// Границы шкалы: нижняя включается, «> 30 %» — строго больше.
func TestProfitabilityGrade_Boundaries(t *testing.T) {
	cases := []struct {
		n     float64
		label string
	}{
		{30.01, "Сверхприбыльный"},
		{30, "Высокорентабельный"}, // ровно 30 — ещё не «сверх»
		{20, "Высокорентабельный"},
		{19.99, "Среднерентабельный"},
		{5, "Среднерентабельный"},
		{1, "Низкорентабельный"},
		{0, "Порог рентабельности"},
		{-0.01, "Убыточный"},
	}
	for _, c := range cases {
		if got := profitabilityGrade(c.n).Label; got != c.label {
			t.Errorf("%.2f%%: got %q, want %q", c.n, got, c.label)
		}
	}
}

func safeAt(s []string, i int) string {
	if i < len(s) {
		return s[i]
	}
	return ""
}
