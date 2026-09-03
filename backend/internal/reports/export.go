package reports

import (
	"strings"

	"github.com/xuri/excelize/v2"
)

// Выгрузка БДР и БДДС в xlsx. Раскладка повторяет эталонные листы:
// кодификатор, статья, месяцы, итог.

// FontName — шрифт всех листов книги. Тот же, что в эталонных формах.
const FontName = "Aptos Narrow"

const (
	colCode  = 1 // A
	colName  = 2 // B
	firstCol = 3 // C — первый месяц
)

// SheetTitle — название листа книги.
func SheetTitle(k Kind) string {
	if k == KindBDDS {
		return "БДДС"
	}
	return "БДР"
}

// WriteSheets добавляет листы БДР и БДДС в уже открытую книгу — рядом с
// листом «Бюджет», который собирает пакет budgets.
func WriteSheets(f *excelize.File, reps ...*Report) error {
	styles, err := newStyles(f)
	if err != nil {
		return err
	}
	for _, rep := range reps {
		name := SheetTitle(rep.Kind)
		if _, err := f.NewSheet(name); err != nil {
			return err
		}
		if err := writeSheet(f, name, rep, styles); err != nil {
			return err
		}
	}
	return nil
}

type sheetStyles struct {
	header int
	group  [3]int // жирность по уровню: 0 — самый крупный
	leaf   int
	money  int
	moneyB int
}

// font — базовый шрифт листа. Возвращаем новый объект на каждый стиль:
// excelize держит указатель, и общий на всех давал бы одну правку на всю
// книгу.
func font(bold bool, size float64) *excelize.Font {
	return &excelize.Font{Family: FontName, Bold: bold, Size: size}
}

const baseSize = 11

func newStyles(f *excelize.File) (*sheetStyles, error) {
	s := &sheetStyles{}
	var err error

	mk := func(st *excelize.Style) (int, error) { return f.NewStyle(st) }

	if s.header, err = mk(&excelize.Style{
		Font:      font(true, baseSize),
		Alignment: &excelize.Alignment{Horizontal: "center", WrapText: true},
		Border:    bottomBorder(),
	}); err != nil {
		return nil, err
	}
	// Три уровня группировки: чем выше, тем крупнее.
	sizes := [3]float64{12, 11, 11}
	for i := range s.group {
		if s.group[i], err = mk(&excelize.Style{Font: font(true, sizes[i])}); err != nil {
			return nil, err
		}
	}
	if s.leaf, err = mk(&excelize.Style{Font: font(false, baseSize)}); err != nil {
		return nil, err
	}
	// Денежный формат с разделителем тысяч. Запятая в коде формата — это
	// ПЛЕЙСХОЛДЕР разделителя тысяч, а не сам символ: Excel подставит
	// разделитель своей локали (в русской — неразрывный пробел).
	const numFmt = `#,##0.00`
	if s.money, err = mk(&excelize.Style{
		Font:         font(false, baseSize),
		CustomNumFmt: strPtr(numFmt),
	}); err != nil {
		return nil, err
	}
	if s.moneyB, err = mk(&excelize.Style{
		Font:         font(true, baseSize),
		CustomNumFmt: strPtr(numFmt),
	}); err != nil {
		return nil, err
	}
	return s, nil
}

func strPtr(s string) *string { return &s }

func bottomBorder() []excelize.Border {
	return []excelize.Border{{Type: "bottom", Color: "BFBFBF", Style: 1}}
}

// writeSheet раскладывает отчёт по листу. Лист начинается сразу с
// таблицы: название проекта и версия — в имени файла, а внутри книги
// шапка только мешала бы сводить листы формулами.
func writeSheet(f *excelize.File, sheet string, rep *Report, st *sheetStyles) error {
	cell := func(col, row int) string {
		name, _ := excelize.CoordinatesToCellName(col, row)
		return name
	}

	// ── Заголовок таблицы ───────────────────────────────────────────────
	const hdr = 1
	_ = f.SetCellStr(sheet, cell(colCode, hdr), "Кодификатор")
	_ = f.SetCellStr(sheet, cell(colName, hdr), "Статья оборотов")
	for i, label := range rep.MonthLabels {
		_ = f.SetCellStr(sheet, cell(firstCol+i, hdr), label)
	}
	totalCol := firstCol + rep.Months
	_ = f.SetCellStr(sheet, cell(totalCol, hdr), "Итого")
	_ = f.SetCellStyle(sheet, cell(colCode, hdr), cell(totalCol, hdr), st.header)

	// ── Строки ──────────────────────────────────────────────────────────
	for i, r := range rep.Rows {
		row := hdr + 1 + i

		_ = f.SetCellStr(sheet, cell(colCode, row), r.Code)
		// Отступ по уровню: иерархия читается без заливки, которую в форме
		// использовать не договаривались.
		_ = f.SetCellStr(sheet, cell(colName, row),
			strings.Repeat("    ", r.Level)+r.Name)

		nameStyle := st.leaf
		moneyStyle := st.money
		if r.Group {
			lvl := r.Level
			if lvl > 2 {
				lvl = 2
			}
			nameStyle = st.group[lvl]
			moneyStyle = st.moneyB
		}
		_ = f.SetCellStyle(sheet, cell(colCode, row), cell(colName, row), nameStyle)

		// Нули не пишем совсем: в кодификаторе больше сотни статей, и
		// заполненных из них единицы — стена нулей делает лист нечитаемым.
		// Пустая ячейка в Excel и есть ноль.
		for m, v := range r.Monthly {
			if v != 0 {
				_ = f.SetCellFloat(sheet, cell(firstCol+m, row), v, 2, 64)
			}
		}
		if r.Total != 0 {
			_ = f.SetCellFloat(sheet, cell(totalCol, row), r.Total, 2, 64)
		}
		_ = f.SetCellStyle(sheet, cell(firstCol, row), cell(totalCol, row), moneyStyle)
	}

	// ── Ширины и закрепление ────────────────────────────────────────────
	_ = f.SetColWidth(sheet, "A", "A", 14)
	_ = f.SetColWidth(sheet, "B", "B", 52)
	firstMonth, _ := excelize.ColumnNumberToName(firstCol)
	lastCol, _ := excelize.ColumnNumberToName(totalCol)
	_ = f.SetColWidth(sheet, firstMonth, lastCol, 14)

	// Кодификатор и статья остаются на месте при прокрутке вправо, шапка —
	// при прокрутке вниз: без этого в отчёте на 18 месяцев не разобраться.
	return f.SetPanes(sheet, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      colName,
		YSplit:      hdr,
		TopLeftCell: cell(firstCol, hdr+1),
		ActivePane:  "bottomRight",
	})
}
