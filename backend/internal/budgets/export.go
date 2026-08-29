package budgets

import (
	"bytes"
	"fmt"
	"time"

	"github.com/xuri/excelize/v2"

	"ibcon-budget/internal/calc"
	"ibcon-budget/internal/reports"
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

	// Limited — урезанная выгрузка администратора проекта: строки
	// 178-212 листа «2.Бюджет», то есть накладные расходы по статьям и
	// их итог. Ни ФОТ, ни выручки, ни прибыли, ни рентабельности в такой
	// книге быть не должно; непредвиденные (214) и АУП (215) — тоже, они
	// за границей 212 (решение владельца 2026-08-29).
	//
	// Такая книга состоит из ОДНОГО листа: БДР и БДДС администратору
	// проекта не выгружаются вовсе.
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
//
// reps — листы БДР и БДДС, которые лягут в ту же книгу следом за
// «Бюджетом»: экономисту нужны три листа сразу. У администратора проекта
// их нет — его книга состоит из одного урезанного листа (см. Limited).
func BuildExport(m ExportMeta, r *calc.CalcResult, reps ...*reports.Report) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	const sheet = "Бюджет"
	idx, err := f.NewSheet(sheet)
	if err != nil {
		return nil, err
	}
	f.SetActiveSheet(idx)
	_ = f.DeleteSheet("Sheet1")

	// Шрифт книги — тот же, что на листах БДР и БДДС: они лежат рядом в
	// одном файле, и разнобой был бы заметен сразу.
	font := func(bold bool, size float64) *excelize.Font {
		return &excelize.Font{Family: reports.FontName, Bold: bold, Size: size}
	}
	const baseSize = 11

	money, err := f.NewStyle(&excelize.Style{
		Font:   font(false, baseSize),
		NumFmt: 4, // #,##0.00
	})
	if err != nil {
		return nil, err
	}
	// moneyBold — денежные ячейки строк, которые владелец просил выделить:
	// итоги стоимости и прибыли, а также блок ФОТ.
	moneyBold, err := f.NewStyle(&excelize.Style{
		Font:   font(true, baseSize),
		NumFmt: 4,
	})
	if err != nil {
		return nil, err
	}
	text, err := f.NewStyle(&excelize.Style{Font: font(false, baseSize)})
	if err != nil {
		return nil, err
	}
	textBold, err := f.NewStyle(&excelize.Style{Font: font(true, baseSize)})
	if err != nil {
		return nil, err
	}
	head, err := f.NewStyle(&excelize.Style{
		Font: font(true, baseSize),
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"E8EEF0"}},
	})
	if err != nil {
		return nil, err
	}
	title, err := f.NewStyle(&excelize.Style{Font: font(true, 13)})
	if err != nil {
		return nil, err
	}

	_ = f.SetColWidth(sheet, "A", "A", 42)
	_ = f.SetColWidth(sheet, "B", "N", 18)

	set := func(cell string, v any) { _ = f.SetCellValue(sheet, cell, v) }
	// styleRow — один стиль на диапазон ячеек строки.
	styleRow := func(from, to string, st int) { _ = f.SetCellStyle(sheet, from, to, st) }
	row := 1
	put := func(label string, v any) {
		set(fmt.Sprintf("A%d", row), label)
		styleRow(fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), text)
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
		set(fmt.Sprintf("A%d", row), "НАКЛАДНЫЕ РАСХОДЫ (строки 178-212 листа «2.Бюджет»)")
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

		putLine := func(label string, bold bool, value func(calc.MonthlyResult) float64) {
			set(fmt.Sprintf("A%d", row), label)
			nameStyle, numStyle := text, money
			if bold {
				nameStyle, numStyle = textBold, moneyBold
			}
			styleRow(fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), nameStyle)
			sum := 0.0
			for i, mr := range r.Monthly {
				col, _ := excelize.ColumnNumberToName(i + 2)
				set(fmt.Sprintf("%s%d", col, row), value(mr))
				sum += value(mr)
			}
			set(fmt.Sprintf("%s%d", totalCol, row), sum)
			firstNum, _ := excelize.ColumnNumberToName(2)
			styleRow(fmt.Sprintf("%s%d", firstNum, row), fmt.Sprintf("%s%d", totalCol, row), numStyle)
			row++
		}

		for i, label := range overheadTitles {
			putLine(label, false, func(mr calc.MonthlyResult) float64 { return mr.Overhead[i] })
		}
		// Строка 212 — граница выгрузки администратора проекта. Всё, что
		// дальше (непредвиденные 214, АУП 215), ему не показывается.
		putLine("212 Итого накладные расходы", true,
			func(mr calc.MonthlyResult) float64 { return mr.ProjectCostsExFOT })

		// Листы БДР и БДДС сюда не добавляются: администратору проекта их
		// не выгружают вовсе.
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

	// bold — показатели, которые владелец просил выделить: по ним читают
	// бюджет, остальные строки блока справочные.
	totals := []struct {
		label string
		value float64
		bold  bool
	}{
		{"ФОТ (вкл. взносы и НДФЛ)", r.TotalFOT, true},
		{"Итого расходы без НДС", r.TotalCosts, false},
		{"Итого стоимость работ без НДС", r.TotalRevenue, true},
		{"Выручка с НДС", r.TotalRevenueWithVAT, false},
		{"Операционная прибыль", r.OperatingProfit, false},
		{"Налог на прибыль", r.Tax, true},
		{"Чистая прибыль", r.NetProfit, true},
	}
	for _, t := range totals {
		labelCell := fmt.Sprintf("A%d", row)
		valueCell := fmt.Sprintf("B%d", row)
		put(t.label, t.value)
		if t.bold {
			styleRow(labelCell, labelCell, textBold)
			styleRow(valueCell, valueCell, moneyBold)
		} else {
			styleRow(valueCell, valueCell, money)
		}
	}
	rentLabel, rentValue := fmt.Sprintf("A%d", row), fmt.Sprintf("B%d", row)
	put("Рентабельность, %", r.Profitability)
	styleRow(rentLabel, rentLabel, textBold)
	styleRow(rentValue, rentValue, moneyBold)
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

	// Блок ФОТ (строки 168-176 формы) выделен целиком: по нему сверяют
	// зарплатную часть, и владелец просил его выделить.
	lines := []struct {
		label string
		bold  bool
		pick  func(calc.MonthlyResult) float64
	}{
		{"168 ФОТ", true, func(x calc.MonthlyResult) float64 { return x.FOT }},
		{"169 Премии и компенсации", true, func(x calc.MonthlyResult) float64 { return x.Bonuses }},
		{"170-171 Переработки", true, func(x calc.MonthlyResult) float64 { return x.OvertimeRF + x.OvertimeKG }},
		{"172 НДФЛ", true, func(x calc.MonthlyResult) float64 { return x.NDFL }},
		{"173-174 Взносы", true, func(x calc.MonthlyResult) float64 { return x.InsuranceRF + x.InsuranceKG }},
		{"176 ФОТ вкл. взносы", true, func(x calc.MonthlyResult) float64 { return x.TotalFOT }},
		{"212 Накладные расходы", false, func(x calc.MonthlyResult) float64 { return x.ProjectCostsExFOT }},
		{"214 Непредвиденные", false, func(x calc.MonthlyResult) float64 { return x.Unpredictables }},
		{"215 АУП", false, func(x calc.MonthlyResult) float64 { return x.AUP }},
		{"222 БГ на исполнение", false, func(x calc.MonthlyResult) float64 { return x.BGExecution }},
		{"226 БГ на гарантийный период", false, func(x calc.MonthlyResult) float64 { return x.BGWarranty }},
		{"230 БГ на аванс", false, func(x calc.MonthlyResult) float64 { return x.BGAdvance }},
		{"232 Итого расходы", false, func(x calc.MonthlyResult) float64 { return x.TotalCosts }},
		{"236 Выручка", false, func(x calc.MonthlyResult) float64 { return x.Revenue }},
	}
	monthsLastCol, _ := excelize.ColumnNumberToName(len(r.Monthly) + 1)
	for _, l := range lines {
		set(fmt.Sprintf("A%d", row), l.label)
		nameStyle, numStyle := text, money
		if l.bold {
			nameStyle, numStyle = textBold, moneyBold
		}
		styleRow(fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), nameStyle)
		for i, mr := range r.Monthly {
			col, _ := excelize.ColumnNumberToName(i + 2)
			set(fmt.Sprintf("%s%d", col, row), l.pick(mr))
		}
		styleRow(fmt.Sprintf("B%d", row), fmt.Sprintf("%s%d", monthsLastCol, row), numStyle)
		row++
	}

	if len(reps) > 0 {
		if err := reports.WriteSheets(f, reps...); err != nil {
			return nil, err
		}
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
