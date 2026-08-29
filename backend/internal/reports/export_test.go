package reports

import (
	"bytes"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"ibcon-budget/internal/calc"
)

// Выгрузка: два листа, шапка, месяцы и итоги на своих местах.
func TestBuildExport(t *testing.T) {
	start := date(2027, time.January)
	res := resWith(3, func(m *calc.MonthlyResult) {
		m.Revenue = 1_000
		m.Overhead[182-178] = 100 // аренда офиса
	})
	p := rf(start)

	data, err := BuildExport(
		Meta{ProjectName: "Тестовый проект", Customer: "Заказчик", VersionNo: 2},
		Build(KindBDR, res, p), Build(KindBDDS, res, p),
	)
	if err != nil {
		t.Fatalf("сборка книги: %v", err)
	}

	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("книга не читается: %v", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) != 2 || sheets[0] != "БДР" || sheets[1] != "БДДС" {
		t.Fatalf("листы книги: %v, ожидались [БДР БДДС]", sheets)
	}

	// Заголовок и подзаголовок.
	if v, _ := f.GetCellValue("БДР", "A1"); v != "БДР — Тестовый проект" {
		t.Errorf("заголовок листа: %q", v)
	}
	if v, _ := f.GetCellValue("БДР", "A2"); v != "Заказчик · Версия 2" {
		t.Errorf("подзаголовок: %q", v)
	}

	// Шапка таблицы: три месяца и колонка «Итого».
	if v, _ := f.GetCellValue("БДР", "C4"); v != "янв. 27" {
		t.Errorf("первый месяц: %q", v)
	}
	if v, _ := f.GetCellValue("БДР", "E4"); v != "мар. 27" {
		t.Errorf("третий месяц: %q", v)
	}
	if v, _ := f.GetCellValue("БДР", "F4"); v != "Итого" {
		t.Errorf("колонка итога: %q", v)
	}

	// Первая строка кодификатора — «1 Выручка», её итог 3 000.
	if v, _ := f.GetCellValue("БДР", "A5"); v != "1" {
		t.Errorf("код первой строки: %q", v)
	}
	// GetCellValue отдаёт значение уже по формату ячейки.
	if v, _ := f.GetCellValue("БДР", "F5"); v != "3,000.00" {
		t.Errorf("итог выручки: %q, ожидалось 3,000.00", v)
	}
	// Нулевая строка (1.2 «Прочие поступления») остаётся пустой: стена
	// нулей на сотне незаполненных статей нечитаема.
	if v, _ := f.GetCellValue("БДР", "F7"); v != "" {
		t.Errorf("пустая статья: %q, ожидалась пустая ячейка", v)
	}

	// В БДДС столько же месяцев.
	if v, _ := f.GetCellValue("БДДС", "F4"); v != "Итого" {
		t.Errorf("БДДС, колонка итога: %q", v)
	}
}

// Имя файла: русское, без запрещённых символов.
func TestFileName(t *testing.T) {
	got := FileName(Meta{ProjectName: `Мост / А-1`, VersionNo: 3})
	want := "БДР_БДДС_Мост _ А-1_в3.xlsx"
	if got != want {
		t.Errorf("имя файла: got %q, want %q", got, want)
	}
	if got := FileName(Meta{}); got != "БДР_БДДС.xlsx" {
		t.Errorf("без проекта и версии: %q", got)
	}
}
