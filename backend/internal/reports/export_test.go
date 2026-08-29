package reports

import (
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"ibcon-budget/internal/calc"
)

// Выгрузка: листы отчётов, месяцы и итоги на своих местах.
func TestWriteSheets(t *testing.T) {
	start := date(2027, time.January)
	res := resWith(3, func(m *calc.MonthlyResult) {
		m.Revenue = 1_000
		m.Overhead[182-178] = 100 // аренда офиса
	})
	p := rf(start)

	f := excelize.NewFile()
	defer f.Close()

	if err := WriteSheets(f, Build(KindBDR, res, p), Build(KindBDDS, res, p)); err != nil {
		t.Fatalf("сборка листов: %v", err)
	}

	sheets := f.GetSheetList()
	// Первым идёт пустой лист самой книги: в выгрузке его заменяет
	// «Бюджет», который добавляет пакет budgets.
	if len(sheets) != 3 || sheets[1] != "БДР" || sheets[2] != "БДДС" {
		t.Fatalf("листы книги: %v, ожидались [… БДР БДДС]", sheets)
	}

	// Лист начинается сразу с таблицы: шапки с названием проекта нет,
	// она осталась только в имени файла.
	if v, _ := f.GetCellValue("БДР", "A1"); v != "Кодификатор" {
		t.Errorf("A1 должен быть заголовком таблицы, got %q", v)
	}
	if v, _ := f.GetCellValue("БДР", "B1"); v != "Статья оборотов" {
		t.Errorf("B1: %q", v)
	}
	if v, _ := f.GetCellValue("БДР", "C1"); v != "янв. 27" {
		t.Errorf("первый месяц: %q", v)
	}
	if v, _ := f.GetCellValue("БДР", "E1"); v != "мар. 27" {
		t.Errorf("третий месяц: %q", v)
	}
	if v, _ := f.GetCellValue("БДР", "F1"); v != "Итого" {
		t.Errorf("колонка итога: %q", v)
	}

	// Первая строка кодификатора — «1 Выручка», её итог 3 000.
	if v, _ := f.GetCellValue("БДР", "A2"); v != "1" {
		t.Errorf("код первой строки: %q", v)
	}
	// GetCellValue отдаёт значение уже по формату ячейки.
	if v, _ := f.GetCellValue("БДР", "F2"); v != "3,000.00" {
		t.Errorf("итог выручки: %q, ожидалось 3,000.00", v)
	}
	// Нулевая строка (1.2 «Прочие поступления») остаётся пустой: стена
	// нулей на сотне незаполненных статей нечитаема.
	if v, _ := f.GetCellValue("БДР", "F4"); v != "" {
		t.Errorf("пустая статья: %q, ожидалась пустая ячейка", v)
	}

	// Шрифт книги — Aptos Narrow, у шапки таблицы он жирный.
	sid, err := f.GetCellStyle("БДР", "A1")
	if err != nil {
		t.Fatalf("стиль шапки: %v", err)
	}
	st, err := f.GetStyle(sid)
	if err != nil {
		t.Fatalf("чтение стиля: %v", err)
	}
	if st.Font == nil || st.Font.Family != FontName || !st.Font.Bold {
		t.Errorf("шапка: ожидался жирный %s, got %+v", FontName, st.Font)
	}

	// В БДДС столько же месяцев.
	if v, _ := f.GetCellValue("БДДС", "F1"); v != "Итого" {
		t.Errorf("БДДС, колонка итога: %q", v)
	}
}
