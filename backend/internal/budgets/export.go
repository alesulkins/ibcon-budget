package budgets

import (
	"bytes"
	"fmt"
	"time"

	"github.com/xuri/excelize/v2"

	"ibcon-budget/internal/calc"
)

/*
Выгрузка бюджета в xlsx.

ОБЪЁМ. Книга содержит то, что платформа считает сегодня: итоговые
показатели, помесячную разбивку и построчную раскладку расходов.
Это НЕ копия эталонной формы `calc_sheets.xlsm` со всеми листами
4.1–4.12 и внутренними контрольными ячейками — воспроизведение формы
целиком относится к отдельному этапу.

ФОРМУЛ В КНИГЕ НЕТ. Выгружаются посчитанные значения: формулы в файле
позволили бы править выгрузку и получать числа, разошедшиеся с
платформой, а книга должна быть слепком расчёта на момент выгрузки.
*/

// ExportMeta — шапка выгрузки: что за проект и какая версия.
type ExportMeta struct {
	ProjectID      int
	ProjectName    string
	Customer       string
	ExecutorName   string
	VersionNo      int
	VersionLabel   string
	Status         string
	StartDate      time.Time
	DurationMonths int

	// Limited — урезанная выгрузка администратора проекта.
	// ТЗ: он «видит информацию только по строкам 178-214 листа
	// "2.Бюджет" Формы» — это накладные расходы по статьям, их итог
	// и непредвиденные. Ни ФОТ, ни выручки, ни прибыли, ни
	// рентабельности в такой книге быть не должно.
	Limited bool
}

// overheadTitles — названия строк 178-211 листа «2.Бюджет» в том же
// порядке, в каком движок раскладывает MonthlyResult.Overhead.
// Индекс массива + 178 = номер строки формы.
var overheadTitles = [34]string{
	"178 Аренда квартир (вкл. уборку)",
	"179 Услуги риелтора",
	"180 Аренда транспорта и покупка авто",
	"181 Обустройство строительной площадки",
	"182 Аренда офиса",
	"183 Уборка офиса",
	"184 Билеты",
	"185 Командировочные расходы",
	"186 Интернет",
	"187 Мобильная связь",
	"188 Лабораторные исследования",
	"189 Приборы строительного контроля",
	"190 Обучение персонала",
	"191 Медицинский осмотр",
	"192 Спецодежда",
	"193 Приобретение ПО",
	"194 Приобретение ПК и оргтехники",
	"195 Приобретение мебели",
	"196 Содержание офиса",
	"197 Почтовые расходы",
	"198 ГСМ",
	"199 Транспортные услуги",
	"200 Субподряд, ГПХ внешний",
	"201 Субподряд, ГПХ сотрудников",
	"202 Субподрядные работы",
	"203 Субподряд (организационные улучшения)",
	"204 Представительские расходы",
	"205 Корпоративные мероприятия",
	"206 Услуги банков",
	"207 Страхование ответственности",
	"208 Коммунальные расходы",
	"209 Охрана объекта",
	"210 Аренда гаража",
	"211 Страхование КАСКО и ОСАГО",
}

// statusTitles — подписи статусов, те же, что в интерфейсе.
var statusTitles = map[string]string{
	StatusDraft:       "Черновик",
	StatusUnderReview: "На согласовании",
	StatusApproved:    "Согласован",
	StatusArchive:     "Архив",
}

// ExportFileName — имя файла выгрузки: «Бюджет 9.1 — Название.xlsx».
func ExportFileName(m ExportMeta) string {
	name := m.ProjectName
	// Символы, недопустимые в именах файлов на части систем.
	for _, bad := range []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"} {
		name = replaceAll(name, bad, " ")
	}
	return fmt.Sprintf("Бюджет %d.%d — %s.xlsx", m.ProjectID, m.VersionNo, name)
}

func replaceAll(s, old, new string) string {
	out := ""
	for _, r := range s {
		if string(r) == old {
			out += new
			continue
		}
		out += string(r)
	}
	return out
}

// BuildExport собирает книгу с результатами расчёта.
func BuildExport(m ExportMeta, r *calc.CalcResult) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	const sheet = "Бюджет"
	idx, err := f.NewSheet(sheet)
	if err != nil {
		return nil, err
	}
	f.SetActiveSheet(idx)
	_ = f.DeleteSheet("Sheet1")

	money, err := f.NewStyle(&excelize.Style{
		NumFmt: 4, // #,##0.00
	})
	if err != nil {
		return nil, err
	}
	head, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"E8EEF0"}},
	})
	if err != nil {
		return nil, err
	}
	title, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 13},
	})
	if err != nil {
		return nil, err
	}

	_ = f.SetColWidth(sheet, "A", "A", 42)
	_ = f.SetColWidth(sheet, "B", "N", 18)

	set := func(cell string, v any) { _ = f.SetCellValue(sheet, cell, v) }
	row := 1
	put := func(label string, v any) {
		set(fmt.Sprintf("A%d", row), label)
		if v != nil {
			set(fmt.Sprintf("B%d", row), v)
		}
		row++
	}

	// ── Шапка ────────────────────────────────────────────────────────
	set("A1", fmt.Sprintf("Бюджет %d.%d — %s", m.ProjectID, m.VersionNo, m.ProjectName))
	_ = f.SetCellStyle(sheet, "A1", "A1", title)
	row = 3

	put("Заказчик", m.Customer)
	put("Исполнитель", m.ExecutorName)
	status := statusTitles[m.Status]
	if m.VersionLabel != "" {
		status = fmt.Sprintf("%s (%s)", status, m.VersionLabel)
	}
	put("Статус бюджета", status)
	put("Дата начала", m.StartDate.Format("02.01.2006"))
	put("Продолжительность, мес.", m.DurationMonths)
	put("Выгружено", time.Now().Format("02.01.2006 15:04"))
	row++

	// ── Урезанная выгрузка администратора проекта ────────────────────
	// Только карточка проекта и строки 178-214: накладные расходы по
	// статьям, их итог и непредвиденные.
	if m.Limited {
		set(fmt.Sprintf("A%d", row), "НАКЛАДНЫЕ РАСХОДЫ (строки 178-214 листа «2.Бюджет»)")
		_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), head)
		row++

		headerRow := row
		set(fmt.Sprintf("A%d", row), "Статья")
		for i := range r.Monthly {
			col, _ := excelize.ColumnNumberToName(i + 2)
			set(fmt.Sprintf("%s%d", col, row), m.StartDate.AddDate(0, i, 0).Format("01.2006"))
		}
		totalCol, _ := excelize.ColumnNumberToName(len(r.Monthly) + 2)
		set(fmt.Sprintf("%s%d", totalCol, row), "Итого")
		_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", headerRow),
			fmt.Sprintf("%s%d", totalCol, headerRow), head)
		row++

		putLine := func(label string, value func(calc.MonthlyResult) float64) {
			set(fmt.Sprintf("A%d", row), label)
			sum := 0.0
			for i, mr := range r.Monthly {
				col, _ := excelize.ColumnNumberToName(i + 2)
				cell := fmt.Sprintf("%s%d", col, row)
				v := value(mr)
				sum += v
				set(cell, v)
				_ = f.SetCellStyle(sheet, cell, cell, money)
			}
			cell := fmt.Sprintf("%s%d", totalCol, row)
			set(cell, sum)
			_ = f.SetCellStyle(sheet, cell, cell, money)
			row++
		}

		for i, label := range overheadTitles {
			putLine(label, func(mr calc.MonthlyResult) float64 { return mr.Overhead[i] })
		}
		putLine("212 Итого накладные расходы",
			func(mr calc.MonthlyResult) float64 { return mr.ProjectCostsExFOT })
		putLine("214 Непредвиденные расходы",
			func(mr calc.MonthlyResult) float64 { return mr.Unpredictables })

		var buf bytes.Buffer
		if err := f.Write(&buf); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}

	// ── Итоговые показатели ──────────────────────────────────────────
	set(fmt.Sprintf("A%d", row), "ИТОГОВЫЕ ПОКАЗАТЕЛИ")
	_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("B%d", row), head)
	row++

	totals := []struct {
		label string
		value float64
	}{
		{"ФОТ (вкл. взносы и НДФЛ)", r.TotalFOT},
		{"Итого расходы без НДС", r.TotalCosts},
		{"Итого стоимость работ без НДС", r.TotalRevenue},
		{"Выручка с НДС", r.TotalRevenueWithVAT},
		{"Операционная прибыль", r.OperatingProfit},
		{"Налог на прибыль", r.Tax},
		{"Чистая прибыль", r.NetProfit},
	}
	for _, t := range totals {
		cell := fmt.Sprintf("B%d", row)
		put(t.label, t.value)
		_ = f.SetCellStyle(sheet, cell, cell, money)
	}
	put("Рентабельность, %", r.Profitability)
	row++

	// ── Помесячная разбивка ──────────────────────────────────────────
	set(fmt.Sprintf("A%d", row), "ПОМЕСЯЧНАЯ РАЗБИВКА")
	_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), head)
	row++

	headerRow := row
	set(fmt.Sprintf("A%d", row), "Статья")
	for i := range r.Monthly {
		col, _ := excelize.ColumnNumberToName(i + 2)
		set(fmt.Sprintf("%s%d", col, row), m.StartDate.AddDate(0, i, 0).Format("01.2006"))
	}
	lastCol, _ := excelize.ColumnNumberToName(len(r.Monthly) + 1)
	_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", headerRow),
		fmt.Sprintf("%s%d", lastCol, headerRow), head)
	row++

	lines := []struct {
		label string
		pick  func(calc.MonthlyResult) float64
	}{
		{"ФОТ вкл. взносы", func(x calc.MonthlyResult) float64 { return x.TotalFOT }},
		{"Накладные расходы", func(x calc.MonthlyResult) float64 { return x.ProjectCostsExFOT }},
		{"Непредвиденные", func(x calc.MonthlyResult) float64 { return x.Unpredictables }},
		{"АУП", func(x calc.MonthlyResult) float64 { return x.AUP }},
		{"БГ на исполнение", func(x calc.MonthlyResult) float64 { return x.BGExecution }},
		{"БГ на гарантийный период", func(x calc.MonthlyResult) float64 { return x.BGWarranty }},
		{"БГ на аванс", func(x calc.MonthlyResult) float64 { return x.BGAdvance }},
		{"Итого расходы", func(x calc.MonthlyResult) float64 { return x.TotalCosts }},
		{"Выручка", func(x calc.MonthlyResult) float64 { return x.Revenue }},
	}
	for _, l := range lines {
		set(fmt.Sprintf("A%d", row), l.label)
		for i, mr := range r.Monthly {
			col, _ := excelize.ColumnNumberToName(i + 2)
			cell := fmt.Sprintf("%s%d", col, row)
			set(cell, l.pick(mr))
			_ = f.SetCellStyle(sheet, cell, cell, money)
		}
		row++
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
