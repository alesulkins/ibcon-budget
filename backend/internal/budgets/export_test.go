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
	mustHave := []string{"Итого накладные расходы", "Аренда офиса"}
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

// Маржинальность и справочная строка 249 стоят в итоговых показателях,
// а не в помесячной разбивке: их смотрят как итог за проект.
func TestBuildExport_TotalsRows(t *testing.T) {
	res := testResult()
	res.OperatingMargin = 600
	res.RefRateAmount = 150

	data, err := BuildExport(testMeta(false), res)
	if err != nil {
		t.Fatalf("сборка книги: %v", err)
	}
	f := openBook(t, data)
	defer f.Close()

	want := map[string]string{
		"Операционная маржинальность":         "600.00",
		"Стоимость + ставка рефинансирования": "150.00",
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
				t.Errorf("%s: значение %q, ожидалось %q", r[0], r[1], v)
			}
		}
	}
	for label := range want {
		if !found[label] {
			t.Errorf("строка %q не найдена в итоговых показателях", label)
		}
	}
}

// Помесячная таблица: итог стоит ПЕРЕД месяцами, а месяцы идут подряд.
// AddDate переполнял короткие месяцы, и проект с 31 августа терял
// сентябрь: шапка шла «08, 10, 10, 12, 12».
func TestBuildExport_MonthlyLayout(t *testing.T) {
	res := testResult()
	res.Monthly = append(res.Monthly, res.Monthly[0], res.Monthly[0])
	for i := range res.Monthly {
		res.Monthly[i].Month = i + 1
		res.Monthly[i].TotalFOT = 100
	}

	meta := testMeta(false)
	meta.StartDate = time.Date(2026, time.August, 31, 0, 0, 0, 0, time.UTC)
	meta.DurationMonths = len(res.Monthly)

	data, err := BuildExport(meta, res)
	if err != nil {
		t.Fatalf("сборка книги: %v", err)
	}
	f := openBook(t, data)
	defer f.Close()

	rows, _ := f.GetRows("Бюджет")
	var header, fot []string
	for _, r := range rows {
		if len(r) > 1 && r[0] == "Статья" && r[1] == "Итого" {
			header = r
		}
		if len(r) > 1 && r[0] == "ФОТ вкл. взносы" {
			fot = r
		}
	}
	if header == nil {
		t.Fatal("шапка помесячной таблицы не найдена")
	}
	wantMonths := []string{"08.2026", "09.2026", "10.2026", "11.2026", "12.2026"}
	for i, w := range wantMonths {
		if got := safeAt(header, i+2); got != w {
			t.Errorf("месяц %d: got %q, want %q", i+1, got, w)
		}
	}
	// Итог за проект — сразу после названия статьи.
	if fot == nil {
		t.Fatal("строка «ФОТ вкл. взносы» не найдена")
	}
	if got := safeAt(fot, 1); got != "500.00" {
		t.Errorf("итог строки ФОТ: got %q, want 500.00", got)
	}
}

// Номера строк формы в подписях не пишем: пользователю они не нужны.
func TestBuildExport_NoRowNumbers(t *testing.T) {
	res := testResult()
	data, err := BuildExport(testMeta(false), res)
	if err != nil {
		t.Fatalf("сборка книги: %v", err)
	}
	f := openBook(t, data)
	defer f.Close()

	rows, _ := f.GetRows("Бюджет")
	for _, r := range rows {
		if len(r) == 0 {
			continue
		}
		label := r[0]
		if len(label) > 3 && label[0] >= '1' && label[0] <= '2' &&
			label[1] >= '0' && label[1] <= '9' && label[3] == ' ' {
			t.Errorf("в подписи остался номер строки формы: %q", label)
		}
	}
}

// Сводка по ИТР: человеко-месяцы делятся на длительность проекта.
// Явочно — только месяцы с зарплатой выше выплаты за межвахтовый отдых.
func TestBuildExport_ITRSummary(t *testing.T) {
	res := testResult()
	res.TotalRevenue = 3_000_000

	meta := testMeta(false)
	// Инженер работает все три месяца полностью, техник — на межвахтовом
	// отдыхе (выплата 30 000, ниже порога явки), рабочий не ИТР вовсе.
	meta.Employees = []calc.Employee{
		{Position: "Инженер ПТО", SalaryNet: 200_000,
			MonthlySchedule: []string{calc.ScheduleOF, calc.ScheduleOF, calc.ScheduleOF}},
		{Position: "Техник ПТО", SalaryNet: 200_000,
			MonthlySchedule: []string{calc.ScheduleMV, calc.ScheduleMV, calc.ScheduleMV}},
		{Position: "Разнорабочий", SalaryNet: 100_000,
			MonthlySchedule: []string{calc.ScheduleOF, calc.ScheduleOF, calc.ScheduleOF}},
	}
	meta.ITRPositions = map[string]bool{"инженер пто": true, "техник пто": true}

	data, err := BuildExport(meta, res)
	if err != nil {
		t.Fatalf("сборка книги: %v", err)
	}
	f := openBook(t, data)
	defer f.Close()

	rows, _ := f.GetRows("Бюджет")
	var count, cost []string
	for _, r := range rows {
		if len(r) > 2 && r[0] == "Кол-во ИТР в среднем в мес." {
			count = r
		}
		if len(r) > 2 && r[0] == "Средняя стоимость чел/мес" {
			cost = r
		}
	}
	if count == nil || cost == nil {
		t.Fatal("сводка по ИТР не найдена")
	}
	// Явочно — только инженер (3 месяца / 3 = 1);
	// списочно — инженер и техник (6 / 3 = 2). Разнорабочий не ИТР.
	if got := safeAt(count, 1); got != "1.00" {
		t.Errorf("явочно: got %q, want 1.00", got)
	}
	if got := safeAt(count, 2); got != "2.00" {
		t.Errorf("списочно: got %q, want 2.00", got)
	}
	// Выручка 3 000 000 за 3 месяца = 1 000 000 в месяц.
	if got := safeAt(cost, 1); got != "1,000,000.00" {
		t.Errorf("стоимость чел/мес явочно: got %q, want 1,000,000.00", got)
	}
	if got := safeAt(cost, 2); got != "500,000.00" {
		t.Errorf("стоимость чел/мес списочно: got %q, want 500,000.00", got)
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

// Полная книга содержит и накладные по статьям (как у администратора),
// и помесячные зарплаты каждого сотрудника: в остальной книге ФОТ идёт
// одной суммой, а сверяют его пофамильно.
func TestBuildExport_OverheadAndSalaries(t *testing.T) {
	res := testResult()
	meta := testMeta(false)
	meta.Employees = []calc.Employee{{
		Position: "Инженер ПТО", FullName: "Тестов Т.Т.",
		Country: calc.CountryRF, SalaryNet: 200_000,
		MonthlySchedule: []string{calc.ScheduleOF, calc.ScheduleMV, calc.ScheduleOF},
	}}

	data, err := BuildExport(meta, res)
	if err != nil {
		t.Fatalf("сборка книги: %v", err)
	}
	f := openBook(t, data)
	defer f.Close()

	rows, _ := f.GetRows("Бюджет")
	var salary []string
	sections := map[string]bool{}
	for _, r := range rows {
		if len(r) == 0 {
			continue
		}
		sections[r[0]] = true
		if r[0] == "Инженер ПТО" {
			salary = r
		}
	}

	for _, want := range []string{
		"НАКЛАДНЫЕ РАСХОДЫ ПО СТАТЬЯМ", "ЗАРПЛАТЫ СОТРУДНИКОВ И ВЗНОСЫ",
		"Аренда офиса", "Итого накладные расходы",
	} {
		if !sections[want] {
			t.Errorf("в книге нет блока или строки %q", want)
		}
	}

	if salary == nil {
		t.Fatal("строка сотрудника не найдена")
	}
	// Должность, ФИО, страна, оклад, итог, дальше месяцы.
	// Второй месяц — межвахтовый отдых: фиксированные 30 000.
	want := []string{
		"Инженер ПТО", "Тестов Т.Т.", "россия", "200,000.00", "430,000.00",
		"200,000.00", "30,000.00", "200,000.00",
	}
	for i := range want {
		if got := safeAt(salary, i); got != want[i] {
			t.Errorf("колонка %d: got %q, want %q", i, got, want[i])
		}
	}
}

// Порядок блоков листа «Бюджет» и отступы между ними — решение владельца:
// карточка со всем, что в ней, сводка по ИТР, помесячная разбивка, зарплаты
// со взносами, накладные по статьям.
func TestBuildExport_BlockOrderAndSpacing(t *testing.T) {
	res := testResult()
	meta := testMeta(false)
	meta.Employees = []calc.Employee{{
		Position: "Инженер ПТО", FullName: "Тестов Т.Т.",
		Country: calc.CountryRF, SalaryNet: 200_000,
		MonthlySchedule: []string{calc.ScheduleOF, calc.ScheduleMV, calc.ScheduleOF},
	}}
	meta.ITRPositions = map[string]bool{"инженер пто": true}
	meta.Params = &calc.InputBudgetParams{
		BGExecution: calc.BankGuarantee{Pct: 5, RatePct: 3, DurationMos: 12},
	}

	data, err := BuildExport(meta, res)
	if err != nil {
		t.Fatalf("сборка книги: %v", err)
	}
	f := openBook(t, data)
	defer f.Close()

	rows, err := f.GetRows("Бюджет")
	if err != nil {
		t.Fatal(err)
	}
	at := func(i int) string {
		if i < 0 || i >= len(rows) || len(rows[i]) == 0 {
			return ""
		}
		return rows[i][0]
	}

	want := []string{
		"ИТОГОВЫЕ ПОКАЗАТЕЛИ",
		"БАНКОВСКИЕ ГАРАНТИИ",
		"СВОДКА ПО ИТР",
		"ПОМЕСЯЧНАЯ РАЗБИВКА",
		"ЗАРПЛАТЫ СОТРУДНИКОВ И ВЗНОСЫ",
		"НАКЛАДНЫЕ РАСХОДЫ ПО СТАТЬЯМ",
	}
	prev := -1
	for _, title := range want {
		idx := -1
		for i := range rows {
			if at(i) == title {
				idx = i
				break
			}
		}
		if idx < 0 {
			t.Fatalf("в книге нет блока %q", title)
		}
		if idx <= prev {
			t.Errorf("блок %q стоит выше предыдущего (строка %d, предыдущий %d)",
				title, idx+1, prev+1)
		}
		if at(idx-1) != "" {
			t.Errorf("перед блоком %q нет пустой строки", title)
		}
		if at(idx-2) == "" {
			t.Errorf("перед блоком %q две пустые строки подряд", title)
		}
		prev = idx
	}
}
