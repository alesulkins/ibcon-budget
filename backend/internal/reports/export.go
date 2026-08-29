package reports

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

// Выгрузка БДР и БДДС в xlsx. Раскладка повторяет эталонные листы:
// кодификатор, статья, месяцы, итог. Оттенки заливки формы не переносим —
// иерархию показываем отступом и жирностью, как на экране.

// Meta — шапка листа: чей это отчёт.
type Meta struct {
	ProjectName  string
	Customer     string
	ExecutorName string
	VersionNo    int
	VersionLabel string
}

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

// BuildExport собирает книгу с обоими отчётами: два листа, как в форме.
func BuildExport(meta Meta, bdr, bdds *Report) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	styles, err := newStyles(f)
	if err != nil {
		return nil, err
	}

	for i, rep := range []*Report{bdr, bdds} {
		name := SheetTitle(rep.Kind)
		if i == 0 {
			// Первый лист книги уже существует под именем Sheet1.
			if err := f.SetSheetName("Sheet1", name); err != nil {
				return nil, err
			}
		} else if _, err := f.NewSheet(name); err != nil {
			return nil, err
		}
		if err := writeSheet(f, name, meta, rep, styles); err != nil {
			return nil, err
		}
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type sheetStyles struct {
	title  int
	header int
	group  [3]int // жирность по уровню: 0 — самый крупный
	leaf   int
	money  int
	moneyB int
}

func newStyles(f *excelize.File) (*sheetStyles, error) {
	s := &sheetStyles{}
	var err error

	mk := func(st *excelize.Style) (int, error) { return f.NewStyle(st) }

	if s.title, err = mk(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 13},
	}); err != nil {
		return nil, err
	}
	if s.header, err = mk(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Alignment: &excelize.Alignment{Horizontal: "center", WrapText: true},
		Border:    bottomBorder(),
	}); err != nil {
		return nil, err
	}
	// Три уровня группировки: чем выше, тем крупнее.
	sizes := [3]float64{12, 11, 11}
	for i := range s.group {
		if s.group[i], err = mk(&excelize.Style{
			Font: &excelize.Font{Bold: true, Size: sizes[i]},
		}); err != nil {
			return nil, err
		}
	}
	if s.leaf, err = mk(&excelize.Style{}); err != nil {
		return nil, err
	}
	// Денежный формат с разделителем тысяч. Запятая в коде формата — это
	// ПЛЕЙСХОЛДЕР разделителя тысяч, а не сам символ: Excel подставит
	// разделитель своей локали (в русской — неразрывный пробел).
	const numFmt = `#,##0.00`
	if s.money, err = mk(&excelize.Style{CustomNumFmt: strPtr(numFmt)}); err != nil {
		return nil, err
	}
	if s.moneyB, err = mk(&excelize.Style{
		Font:         &excelize.Font{Bold: true},
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

func writeSheet(f *excelize.File, sheet string, meta Meta, rep *Report, st *sheetStyles) error {
	cell := func(col, row int) string {
		name, _ := excelize.CoordinatesToCellName(col, row)
		return name
	}

	// ── Шапка ───────────────────────────────────────────────────────────
	var version string
	if meta.VersionNo > 0 {
		version = fmt.Sprintf("Версия %d", meta.VersionNo)
		if meta.VersionLabel != "" {
			version += " (" + meta.VersionLabel + ")"
		}
	}
	title := SheetTitle(rep.Kind) + " — " + meta.ProjectName
	_ = f.SetCellStr(sheet, cell(colCode, 1), title)
	_ = f.SetCellStyle(sheet, cell(colCode, 1), cell(colCode, 1), st.title)

	sub := strings.Join(nonEmpty(meta.Customer, meta.ExecutorName, version), " · ")
	_ = f.SetCellStr(sheet, cell(colCode, 2), sub)

	// ── Заголовок таблицы ───────────────────────────────────────────────
	const hdr = 4
	_ = f.SetCellStr(sheet, cell(colCode, hdr), "Код")
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
	_ = f.SetColWidth(sheet, "A", "A", 12)
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

func nonEmpty(vals ...string) []string {
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return out
}

// FileName — имя файла выгрузки. Русское, поэтому отдаётся с filename*.
func FileName(meta Meta) string {
	name := "БДР_БДДС"
	if meta.ProjectName != "" {
		name += "_" + meta.ProjectName
	}
	if meta.VersionNo > 0 {
		name += fmt.Sprintf("_в%d", meta.VersionNo)
	}
	// Символы, недопустимые в именах файлов.
	name = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) {
			return '_'
		}
		return r
	}, name)
	return name + ".xlsx"
}
