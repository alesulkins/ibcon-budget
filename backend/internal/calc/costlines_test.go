package calc

import (
	"strings"
	"testing"
	"time"
)

// Минимальный проект без сотрудников и параметров: нужен только чтобы
// прогнать лист через движок и посмотреть строку накладных.
func costLinesInputs(duration int) *BudgetInputs {
	return &BudgetInputs{
		ProjectStartDate: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
		DurationMonths:   duration,
		ExecutorName:     ExecutorAibicon,
	}
}

// Ввод листа 4.8 тестового проекта: три позиции ПО, у каждой своя
// раскладка по 6 месяцам.
func referenceSoftwareInput() *InputCostLines {
	return &InputCostLines{Lines: []CostLine{
		// Годовая лицензия, оплачена целиком в первый месяц.
		{Name: "AutoCAD, годовая лицензия", MonthlyAmounts: []float64{120000, 0, 0, 0, 0, 0}},
		// Подписка — равными платежами каждый месяц.
		{Name: "Офисный пакет, подписка", MonthlyAmounts: []float64{15000, 15000, 15000, 15000, 15000, 15000}},
		// Разовая покупка в середине проекта.
		{Name: "Модуль сметных расчётов", MonthlyAmounts: []float64{0, 0, 45000, 0, 0, 0}},
	}}
}

func TestCalcCostLinesSumsPositionsByMonth(t *testing.T) {
	got := calcCostLines(referenceSoftwareInput(), 6)
	want := []float64{135000, 15000, 60000, 15000, 15000, 15000}
	for m := range want {
		if got[m] != want[m] {
			t.Errorf("месяц %d: получено %.2f, ожидалось %.2f", m+1, got[m], want[m])
		}
	}
}

func TestCalcCostLinesTotal(t *testing.T) {
	var total float64
	for _, v := range calcCostLines(referenceSoftwareInput(), 6) {
		total += v
	}
	// 120 000 + 15 000×6 + 45 000
	const want = 255000.0
	if total != want {
		t.Errorf("итого за проект: получено %.2f, ожидалось %.2f", total, want)
	}
}

func TestCalcCostLinesNilInput(t *testing.T) {
	got := calcCostLines(nil, 4)
	if len(got) != 4 {
		t.Fatalf("длина массива %d, ожидалось 4", len(got))
	}
	for m, v := range got {
		if v != 0 {
			t.Errorf("месяц %d: получено %.2f, ожидался 0", m+1, v)
		}
	}
}

func TestCalcCostLinesEmptyList(t *testing.T) {
	got := calcCostLines(&InputCostLines{}, 3)
	for m, v := range got {
		if v != 0 {
			t.Errorf("месяц %d: получено %.2f, ожидался 0", m+1, v)
		}
	}
}

// Длительность проекта уменьшили после сохранения — лишние месяцы
// отбрасываются, а не сдвигают суммы и не роняют расчёт.
func TestCalcCostLinesTruncatesLongerArray(t *testing.T) {
	in := &InputCostLines{Lines: []CostLine{
		{Name: "ПО", MonthlyAmounts: []float64{100, 200, 300, 400, 500}},
	}}
	got := calcCostLines(in, 3)
	want := []float64{100, 200, 300}
	if len(got) != 3 {
		t.Fatalf("длина массива %d, ожидалось 3", len(got))
	}
	for m := range want {
		if got[m] != want[m] {
			t.Errorf("месяц %d: получено %.2f, ожидалось %.2f", m+1, got[m], want[m])
		}
	}
}

// Длительность увеличили — недостающие месяцы читаются как 0.
func TestCalcCostLinesPadsShorterArray(t *testing.T) {
	in := &InputCostLines{Lines: []CostLine{
		{Name: "ПО", MonthlyAmounts: []float64{100, 200}},
	}}
	got := calcCostLines(in, 4)
	want := []float64{100, 200, 0, 0}
	for m := range want {
		if got[m] != want[m] {
			t.Errorf("месяц %d: получено %.2f, ожидалось %.2f", m+1, got[m], want[m])
		}
	}
}

// Позиция без наименования и без сумм не должна ломать расчёт: пустые
// строки таблицы — обычное состояние формы во время заполнения.
func TestCalcCostLinesIgnoresEmptyLine(t *testing.T) {
	in := &InputCostLines{Lines: []CostLine{
		{},
		{Name: "ПО", MonthlyAmounts: []float64{500, 0, 0}},
	}}
	got := calcCostLines(in, 3)
	if got[0] != 500 {
		t.Errorf("месяц 1: получено %.2f, ожидалось 500", got[0])
	}
}

func TestCalcCostLinesZeroDuration(t *testing.T) {
	if got := calcCostLines(referenceSoftwareInput(), 0); len(got) != 0 {
		t.Errorf("длина массива %d, ожидалось 0", len(got))
	}
}

func TestValidateCostLinesRejectsNegative(t *testing.T) {
	in := &InputCostLines{Lines: []CostLine{
		{Name: "ПО", MonthlyAmounts: []float64{100, -50, 0}},
	}}
	err := ValidateCostLines("ПО и лицензии", in)
	if err == nil {
		t.Fatal("отрицательная стоимость должна отклоняться")
	}
	t.Log(err)
}

func TestValidateCostLinesAcceptsReference(t *testing.T) {
	if err := ValidateCostLines("ПО и лицензии", referenceSoftwareInput()); err != nil {
		t.Errorf("корректный ввод отклонён: %v", err)
	}
}

// Расчёт листа 4.8 должен попадать в строку 193 накладных (индекс 15).
func TestSoftwareLinesReachOverheadRow193(t *testing.T) {
	inp := costLinesInputs(6)
	inp.SoftwareLines = referenceSoftwareInput()

	res := Run(inp)
	want := []float64{135000, 15000, 60000, 15000, 15000, 15000}
	for m, w := range want {
		if got := res.Monthly[m].Overhead[15]; got != w {
			t.Errorf("месяц %d, строка 193: получено %.2f, ожидалось %.2f", m+1, got, w)
		}
	}
}

// Версия, сохранённая до перехода на список позиций, не должна обнулиться:
// при отсутствии SoftwareLines берётся старый ввод готовыми суммами.
func TestSoftwareLegacyFallback(t *testing.T) {
	inp := costLinesInputs(3)
	inp.Software = []float64{1000, 2000, 3000}

	res := Run(inp)
	for m, w := range []float64{1000, 2000, 3000} {
		if got := res.Monthly[m].Overhead[15]; got != w {
			t.Errorf("месяц %d, строка 193: получено %.2f, ожидалось %.2f", m+1, got, w)
		}
	}
}

// Новый ввод имеет приоритет: если список позиций задан, старые суммы
// игнорируются целиком, а не складываются с ним.
func TestSoftwareLinesOverrideLegacy(t *testing.T) {
	inp := costLinesInputs(3)
	inp.Software = []float64{1000, 2000, 3000}
	inp.SoftwareLines = &InputCostLines{Lines: []CostLine{
		{Name: "ПО", MonthlyAmounts: []float64{7000, 0, 0}},
	}}

	res := Run(inp)
	for m, w := range []float64{7000, 0, 0} {
		if got := res.Monthly[m].Overhead[15]; got != w {
			t.Errorf("месяц %d, строка 193: получено %.2f, ожидалось %.2f", m+1, got, w)
		}
	}
}

// ── 4.9 «ГПХ внешний» и 4.11 «Субподряд» ────────────────────────────────────
//
// Расчёт у них тот же самый, проверяем главное: каждый лист попадает в СВОЮ
// строку накладных и не задевает соседние. Перепутанные индексы — самая
// вероятная ошибка при добавлении листа в overheadLines.

func TestSubcontractExtLinesReachOverheadRow200(t *testing.T) {
	inp := costLinesInputs(3)
	inp.SubcontractExtLines = &InputCostLines{Lines: []CostLine{
		{Name: "ООО «Геодезия», вынос осей", MonthlyAmounts: []float64{80000, 0, 0}},
		{Name: "ИП Петров, авторский надзор", MonthlyAmounts: []float64{30000, 30000, 30000}},
	}}

	res := Run(inp)
	for m, w := range []float64{110000, 30000, 30000} {
		if got := res.Monthly[m].Overhead[22]; got != w {
			t.Errorf("месяц %d, строка 200: получено %.2f, ожидалось %.2f", m+1, got, w)
		}
	}
	// Соседние строки листа не касаются: 201 — это 4.10, 202 — это 4.11.
	for m := range res.Monthly {
		if got := res.Monthly[m].Overhead[23]; got != 0 {
			t.Errorf("месяц %d, строка 201 не должна меняться, получено %.2f", m+1, got)
		}
		if got := res.Monthly[m].Overhead[24]; got != 0 {
			t.Errorf("месяц %d, строка 202 не должна меняться, получено %.2f", m+1, got)
		}
	}
}

func TestSubcontractGenLinesReachOverheadRow202(t *testing.T) {
	inp := costLinesInputs(3)
	inp.SubcontractGenLines = &InputCostLines{Lines: []CostLine{
		{Name: "Монтаж металлоконструкций", MonthlyAmounts: []float64{0, 500000, 500000}},
	}}

	res := Run(inp)
	for m, w := range []float64{0, 500000, 500000} {
		if got := res.Monthly[m].Overhead[24]; got != w {
			t.Errorf("месяц %d, строка 202: получено %.2f, ожидалось %.2f", m+1, got, w)
		}
	}
	for m := range res.Monthly {
		if got := res.Monthly[m].Overhead[22]; got != 0 {
			t.Errorf("месяц %d, строка 200 не должна меняться, получено %.2f", m+1, got)
		}
	}
}

func TestSubcontractLegacyFallback(t *testing.T) {
	inp := costLinesInputs(3)
	inp.SubcontractExt = []float64{100, 200, 300}
	inp.SubcontractGen = []float64{400, 500, 600}

	res := Run(inp)
	for m, w := range []float64{100, 200, 300} {
		if got := res.Monthly[m].Overhead[22]; got != w {
			t.Errorf("месяц %d, строка 200: получено %.2f, ожидалось %.2f", m+1, got, w)
		}
	}
	for m, w := range []float64{400, 500, 600} {
		if got := res.Monthly[m].Overhead[24]; got != w {
			t.Errorf("месяц %d, строка 202: получено %.2f, ожидалось %.2f", m+1, got, w)
		}
	}
}

// Текст ошибки должен называть тот лист, на котором экономист сейчас стоит,
// иначе тост уводит не туда.
func TestValidateInputCostLinesTitles(t *testing.T) {
	bad := []byte(`{"lines":[{"name":"X","monthly_amounts":[-1]}]}`)
	cases := map[string]string{
		TypeSoftwareItems:       "ПО и лицензии",
		TypeSubcontractExtItems: "ГПХ внешний",
		TypeSubcontractGenItems: "субподрядные работы",
	}
	for typ, want := range cases {
		err := ValidateInput(typ, bad, ExecutorAibicon, 1)
		if err == nil {
			t.Errorf("%s: отрицательная стоимость должна отклоняться", typ)
			continue
		}
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%s: в ошибке нет «%s»: %v", typ, want, err)
		}
	}
}
