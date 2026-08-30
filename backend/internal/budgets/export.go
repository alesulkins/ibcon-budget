package budgets

import (
	"bytes"
	"fmt"
	"strings"
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

	// Employees — сотрудники версии и ITRPositions — названия должностей
	// с признаком ИТР. Нужны сводке по ИТР: она считает, у скольких
	// инженеров в каком месяце была начислена зарплата, а в итогах
	// расчёта лежит только сумма по всем сразу.
	Employees    []calc.Employee
	ITRPositions map[string]bool

	// Params — параметры версии: проценты и сроки банковских гарантий.
	// В CalcResult их нет — там только посчитанные суммы, — а в книге
	// нужно видеть, из чего сумма получилась. nil допустим: у версии,
	// которую ещё не заполняли, параметров нет.
	Params *calc.InputBudgetParams

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
	"Аренда квартир (вкл. уборку)",
	"Услуги риелтора",
	"Аренда транспорта и покупка авто",
	"Обустройство строительной площадки",
	"Аренда офиса",
	"Уборка офиса",
	"Билеты",
	"Командировочные расходы",
	"Интернет",
	"Мобильная связь",
	"Лабораторные исследования",
	"Приборы строительного контроля",
	"Обучение персонала",
	"Медицинский осмотр",
	"Спецодежда",
	"Приобретение ПО",
	"Приобретение ПК и оргтехники",
	"Приобретение мебели",
	"Содержание офиса",
	"Почтовые расходы",
	"ГСМ",
	"Транспортные услуги",
	"Субподряд, ГПХ внешний",
	"Субподряд, ГПХ сотрудников",
	"Субподрядные работы",
	"Субподряд (организационные улучшения)",
	"Представительские расходы",
	"Корпоративные мероприятия",
	"Услуги банков",
	"Страхование ответственности",
	"Коммунальные расходы",
	"Охрана объекта",
	"Аренда гаража",
	"Страхование КАСКО и ОСАГО",
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

	// Раскладка помесячных таблиц: A — статья, B — итог за проект,
	// дальше месяцы. Итог стоит ПЕРЕД месяцами: его смотрят первым, а в
	// конце строки на длинном проекте до него пришлось бы доскроллить.
	const (
		colLabel = 1
		colTotal = 2
		colFirst = 3 // первый месяц
	)
	cellAt := func(col, row int) string {
		name, _ := excelize.CoordinatesToCellName(col, row)
		return name
	}
	lastMonthCol := colFirst + len(r.Monthly) - 1

	// monthLabel — подпись месяца проекта. День фиксируем первым:
	// AddDate переполняет короткие месяцы, и проект с 31 августа давал
	// «31 сентября» → 1 октября, то есть сентябрь выпадал, а октябрь шёл
	// дважды.
	monthLabel := func(i int) string {
		d := time.Date(m.StartDate.Year(), m.StartDate.Month()+time.Month(i), 1,
			0, 0, 0, 0, m.StartDate.Location())
		return d.Format("01.2006")
	}

	// monthlyHeader — шапка помесячной таблицы.
	monthlyHeader := func() {
		set(cellAt(colLabel, row), "Статья")
		set(cellAt(colTotal, row), "Итого")
		for i := range r.Monthly {
			set(cellAt(colFirst+i, row), monthLabel(i))
		}
		styleRow(cellAt(colLabel, row), cellAt(lastMonthCol, row), head)
		row++
	}

	// monthlyLine — строка помесячной таблицы: итог слева, месяцы правее.
	//
	// Статья, у которой не заполнен ни один месяц, в книгу не попадает:
	// иначе выгрузка на треть состоит из строк нулей, а искать в ней
	// приходится те несколько статей, которые в проекте есть. Правило
	// общее для всех, включая урезанную книгу администратора проекта.
	monthlyLine := func(label string, bold bool, value func(calc.MonthlyResult) float64) {
		values := make([]float64, len(r.Monthly))
		var sum float64
		empty := true
		for i, mr := range r.Monthly {
			values[i] = value(mr)
			sum += values[i]
			if values[i] != 0 {
				empty = false
			}
		}
		if empty {
			return
		}

		set(cellAt(colLabel, row), label)
		nameStyle, numStyle := text, money
		if bold {
			nameStyle, numStyle = textBold, moneyBold
		}
		styleRow(cellAt(colLabel, row), cellAt(colLabel, row), nameStyle)

		for i, v := range values {
			set(cellAt(colFirst+i, row), v)
		}
		set(cellAt(colTotal, row), sum)
		styleRow(cellAt(colTotal, row), cellAt(lastMonthCol, row), numStyle)
		row++
	}
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

	// Вердикт по рентабельности — первым делом, до карточки проекта:
	// книгу открывают, чтобы понять, каков бюджет, а не чей он.
	// Администратору проекта его не показываем: рентабельности он не
	// видит ни на экране, ни в этой книге.
	if !m.Limited {
		grade := profitabilityGrade(r.Profitability)
		verdict, vErr := f.NewStyle(&excelize.Style{
			Font: &excelize.Font{Family: reports.FontName, Bold: true, Size: 12, Color: "FFFFFF"},
			Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{grade.Color}},
		})
		if vErr != nil {
			return nil, vErr
		}
		set(cellAt(colLabel, row), fmt.Sprintf("%s — рентабельность %.2f %%",
			grade.Label, r.Profitability))
		styleRow(cellAt(colLabel, row), cellAt(colTotal, row), verdict)
		row += 2
	}

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

		monthlyHeader()
		for i, label := range overheadTitles {
			monthlyLine(label, false, func(mr calc.MonthlyResult) float64 { return mr.Overhead[i] })
		}
		// Строка 212 — граница выгрузки администратора проекта. Всё, что
		// дальше (непредвиденные 214, АУП 215), ему не показывается.
		monthlyLine("Итого накладные расходы", true,
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
		{"Операционная маржинальность", r.OperatingMargin, false},
		{"Операционная прибыль", r.OperatingProfit, false},
		{"Налог на прибыль", r.Tax, true},
		{"Чистая прибыль", r.NetProfit, true},
		// Справочный показатель: начисляется только за первые четыре
		// месяца и ни на что в расчёте не влияет.
		{"Стоимость + ставка рефинансирования", r.RefRateAmount, false},
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

	// ── Сводка по ИТР ────────────────────────────────────────────────
	//
	// Явочно и списочно — два взгляда на одну команду. Списочно: человек
	// числится на проекте, то есть в этом месяце ему вообще начислено.
	// Явочно: он ещё и работал, а не сидел на межвахтовом отдыхе — там
	// платят фиксированные 30 000, поэтому порог стоит чуть выше.
	//
	// «В среднем в месяц» — количество человеко-месяцев, делённое на
	// длительность проекта: если инженер отработал половину срока, он
	// добавляет половину человека.
	if len(m.Employees) > 0 && len(r.Monthly) > 0 {
		const attendedThreshold = 30_001 // выше выплаты за межвахтовый отдых

		var attended, listed float64
		for i := range m.Employees {
			emp := &m.Employees[i]
			if !m.ITRPositions[strings.ToLower(strings.TrimSpace(emp.Position))] {
				continue
			}
			for month := 1; month <= len(r.Monthly); month++ {
				fot := calc.EmployeeFOTAt(emp, month, m.StartDate)
				if fot > 0 {
					listed++
				}
				if fot > attendedThreshold {
					attended++
				}
			}
		}
		months := float64(len(r.Monthly))
		attendedAvg := attended / months
		listedAvg := listed / months

		// Стоимость человеко-месяца: сколько денег проект приносит в
		// месяц на одного инженера. База — стоимость работ без НДС,
		// делённая на длительность.
		revenuePerMonth := r.TotalRevenue / months
		perPerson := func(avg float64) float64 {
			if avg == 0 {
				return 0 // инженеров нет — делить не на что
			}
			return revenuePerMonth / avg
		}

		row++
		set(cellAt(colLabel, row), "СВОДКА ПО ИТР")
		styleRow(cellAt(colLabel, row), cellAt(3, row), head)
		row++

		set(cellAt(2, row), "явочно")
		set(cellAt(3, row), "списочно")
		styleRow(cellAt(colLabel, row), cellAt(3, row), head)
		row++

		itrRows := []struct {
			label            string
			attended, listed float64
		}{
			{"Кол-во ИТР в среднем в мес.", attendedAvg, listedAvg},
			{"Средняя стоимость чел/мес", perPerson(attendedAvg), perPerson(listedAvg)},
		}
		for _, ir := range itrRows {
			set(cellAt(colLabel, row), ir.label)
			set(cellAt(2, row), ir.attended)
			set(cellAt(3, row), ir.listed)
			styleRow(cellAt(colLabel, row), cellAt(colLabel, row), text)
			styleRow(cellAt(2, row), cellAt(3, row), money)
			row++
		}
	}

	// ── Банковские гарантии ──────────────────────────────────────────
	// Суммы БГ уже стоят в помесячной разбивке, но по одной сумме не
	// понять, из чего она вышла: процент от договора, ставка, режим
	// ставки и срок задаются отдельно и в книге были не видны.
	//
	// Незаполненная гарантия в книгу не идёт, а если не заполнена ни
	// одна — не пишем и сам раздел: пустая таблица из трёх нулевых
	// строк только сбивает с толку.
	bgFilled := func(b calc.BankGuarantee, total func(calc.MonthlyResult) float64) bool {
		if b.Pct != 0 || b.RatePct != 0 || b.DurationMos != 0 {
			return true
		}
		for _, mr := range r.Monthly {
			if total(mr) != 0 {
				return true
			}
		}
		return false
	}

	if m.Params != nil {
		all := []struct {
			label string
			bg    calc.BankGuarantee
			total func(calc.MonthlyResult) float64
		}{
			{"На исполнение обязательств", m.Params.BGExecution,
				func(x calc.MonthlyResult) float64 { return x.BGExecution }},
			{"На гарантийный период", m.Params.BGWarranty,
				func(x calc.MonthlyResult) float64 { return x.BGWarranty }},
			{"На аванс", m.Params.BGAdvance,
				func(x calc.MonthlyResult) float64 { return x.BGAdvance }},
		}
		bgs := all[:0:0]
		for _, b := range all {
			if bgFilled(b.bg, b.total) {
				bgs = append(bgs, b)
			}
		}

		if len(bgs) > 0 {
			row++
			set(fmt.Sprintf("A%d", row), "БАНКОВСКИЕ ГАРАНТИИ")
			styleRow(fmt.Sprintf("A%d", row), fmt.Sprintf("F%d", row), head)
			row++

			bgHeader := []string{
				"Вид гарантии", "% от договора", "Ставка, %",
				"Режим ставки", "Срок, мес.", "Сумма за проект",
			}
			for i, h := range bgHeader {
				col, _ := excelize.ColumnNumberToName(i + 1)
				set(fmt.Sprintf("%s%d", col, row), h)
			}
			styleRow(fmt.Sprintf("A%d", row), fmt.Sprintf("F%d", row), head)
			row++
		}

		for _, b := range bgs {
			var sum float64
			for _, mr := range r.Monthly {
				sum += b.total(mr)
			}
			set(fmt.Sprintf("A%d", row), b.label)
			set(fmt.Sprintf("B%d", row), b.bg.Pct)
			set(fmt.Sprintf("C%d", row), b.bg.RatePct)
			// Режим ставки пустой у незаполненной гарантии — не пишем
			// «%/год» там, где гарантии нет вовсе.
			set(fmt.Sprintf("D%d", row), b.bg.RateType)
			set(fmt.Sprintf("E%d", row), b.bg.DurationMos)
			set(fmt.Sprintf("F%d", row), sum)
			styleRow(fmt.Sprintf("A%d", row), fmt.Sprintf("E%d", row), text)
			styleRow(fmt.Sprintf("F%d", row), fmt.Sprintf("F%d", row), money)
			row++
		}
	}

	// ── Накладные расходы по статьям ─────────────────────────────────
	// Тот же разрез, что видит администратор проекта: в помесячной
	// разбивке накладные идут одной строкой, а сверяют их по статьям.
	row++
	set(cellAt(colLabel, row), "НАКЛАДНЫЕ РАСХОДЫ ПО СТАТЬЯМ")
	styleRow(cellAt(colLabel, row), cellAt(colTotal, row), head)
	row++

	monthlyHeader()
	for i, label := range overheadTitles {
		monthlyLine(label, false, func(mr calc.MonthlyResult) float64 { return mr.Overhead[i] })
	}
	monthlyLine("Итого накладные расходы", true,
		func(mr calc.MonthlyResult) float64 { return mr.ProjectCostsExFOT })

	// ── Зарплаты сотрудников ─────────────────────────────────────────
	// Помесячный ФОТ по каждому человеку: в остальной книге он только
	// суммой, а зарплатную часть сверяют пофамильно.
	if len(m.Employees) > 0 {
		row++
		set(cellAt(colLabel, row), "ЗАРПЛАТЫ СОТРУДНИКОВ")
		styleRow(cellAt(colLabel, row), cellAt(colTotal, row), head)
		row++

		// У этой таблицы своя шапка: перед месяцами идут четыре колонки
		// описания сотрудника, а не одна «Статья».
		const (
			empPosition = 1
			empName     = 2
			empCountry  = 3
			empSalary   = 4
			empTotal    = 5
			empFirst    = 6
		)
		empLastCol := empFirst + len(r.Monthly) - 1

		set(cellAt(empPosition, row), "Должность")
		set(cellAt(empName, row), "ФИО")
		set(cellAt(empCountry, row), "Страна НО")
		set(cellAt(empSalary, row), "План ФОТ на руки, ₽")
		set(cellAt(empTotal, row), "Итого за проект")
		for i := range r.Monthly {
			set(cellAt(empFirst+i, row), monthLabel(i))
		}
		styleRow(cellAt(empPosition, row), cellAt(empLastCol, row), head)
		row++

		for i := range m.Employees {
			emp := &m.Employees[i]
			set(cellAt(empPosition, row), emp.Position)
			set(cellAt(empName, row), emp.FullName)
			set(cellAt(empCountry, row), emp.Country)
			set(cellAt(empSalary, row), emp.SalaryNet)
			styleRow(cellAt(empPosition, row), cellAt(empCountry, row), text)

			var sum float64
			for month := 1; month <= len(r.Monthly); month++ {
				v := calc.EmployeeFOTAt(emp, month, m.StartDate)
				sum += v
				set(cellAt(empFirst+month-1, row), v)
			}
			set(cellAt(empTotal, row), sum)
			styleRow(cellAt(empSalary, row), cellAt(empLastCol, row), money)
			row++
		}
	}

	// ── Помесячная разбивка ──────────────────────────────────────────
	set(fmt.Sprintf("A%d", row), "ПОМЕСЯЧНАЯ РАЗБИВКА")
	_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), head)
	row++

	monthlyHeader()

	// Блок ФОТ (строки 168-176 формы) выделен целиком: по нему сверяют
	// зарплатную часть.
	lines := []struct {
		label string
		bold  bool
		pick  func(calc.MonthlyResult) float64
	}{
		{"ФОТ", true, func(x calc.MonthlyResult) float64 { return x.FOT }},
		{"Премии и компенсации", true, func(x calc.MonthlyResult) float64 { return x.Bonuses }},
		{"Переработки", true, func(x calc.MonthlyResult) float64 { return x.OvertimeRF + x.OvertimeKG }},
		{"НДФЛ", true, func(x calc.MonthlyResult) float64 { return x.NDFL }},
		{"Взносы", true, func(x calc.MonthlyResult) float64 { return x.InsuranceRF + x.InsuranceKG }},
		{"ФОТ вкл. взносы", true, func(x calc.MonthlyResult) float64 { return x.TotalFOT }},
		{"Накладные расходы", false, func(x calc.MonthlyResult) float64 { return x.ProjectCostsExFOT }},
		{"Непредвиденные", false, func(x calc.MonthlyResult) float64 { return x.Unpredictables }},
		{"АУП", false, func(x calc.MonthlyResult) float64 { return x.AUP }},
		{"БГ на исполнение", false, func(x calc.MonthlyResult) float64 { return x.BGExecution }},
		{"БГ на гарантийный период", false, func(x calc.MonthlyResult) float64 { return x.BGWarranty }},
		{"БГ на аванс", false, func(x calc.MonthlyResult) float64 { return x.BGAdvance }},
		{"Итого расходы", false, func(x calc.MonthlyResult) float64 { return x.TotalCosts }},
		{"Выручка", false, func(x calc.MonthlyResult) float64 { return x.Revenue }},
	}
	for _, l := range lines {
		monthlyLine(l.label, l.bold, l.pick)
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
