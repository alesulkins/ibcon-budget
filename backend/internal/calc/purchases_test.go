package calc

import (
	"strings"
	"testing"
	"time"
)

func purchasesInputs(duration int) *BudgetInputs {
	return &BudgetInputs{
		ProjectStartDate: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
		DurationMonths:   duration,
		ExecutorName:     ExecutorAibicon,
	}
}

// Ввод листа 4.7 тестового проекта: три позиции приборов, включая покупку
// в ПОСЛЕДНЕМ месяце проекта — она обязана попасть в расчёт (в отличие от
// вагончиков).
func referenceEquipmentInput() *InputPurchases {
	return &InputPurchases{Items: []ItemPurchase{
		{Name: "Нивелир оптический", Month: 1, Count: 2, Price: 45000},
		{Name: "Толщиномер покрытий", Month: 3, Count: 1, Price: 120000},
		{Name: "Тепловизор", Month: 6, Count: 1, Price: 210000},
	}}
}

func TestCalcPurchasesGroupsByMonth(t *testing.T) {
	got := calcPurchases(referenceEquipmentInput(), 6)
	want := []float64{90000, 0, 120000, 0, 0, 210000}
	for m := range want {
		if got[m] != want[m] {
			t.Errorf("месяц %d: получено %.2f, ожидалось %.2f", m+1, got[m], want[m])
		}
	}
}

// Несколько покупок в одном месяце складываются, а не затирают друг друга.
func TestCalcPurchasesSumsSameMonth(t *testing.T) {
	in := &InputPurchases{Items: []ItemPurchase{
		{Name: "A", Month: 2, Count: 3, Price: 1000},
		{Name: "B", Month: 2, Count: 1, Price: 500},
	}}
	got := calcPurchases(in, 3)
	if got[1] != 3500 {
		t.Errorf("месяц 2: получено %.2f, ожидалось 3500", got[1])
	}
}

// Ключевое отличие от вагончиков: последние два месяца проекта открыты.
func TestCalcPurchasesAllowsLastTwoMonths(t *testing.T) {
	in := &InputPurchases{Items: []ItemPurchase{
		{Name: "предпоследний", Month: 5, Count: 1, Price: 100},
		{Name: "последний", Month: 6, Count: 1, Price: 200},
	}}
	got := calcPurchases(in, 6)
	if got[4] != 100 {
		t.Errorf("месяц 5: получено %.2f, ожидалось 100", got[4])
	}
	if got[5] != 200 {
		t.Errorf("месяц 6: получено %.2f, ожидалось 200", got[5])
	}
}

// Строка без месяца — незаполненная, в расчёт не идёт.
func TestCalcPurchasesIgnoresRowWithoutMonth(t *testing.T) {
	in := &InputPurchases{Items: []ItemPurchase{
		{Name: "не заполнена", Month: 0, Count: 5, Price: 1000},
		{Name: "заполнена", Month: 1, Count: 1, Price: 300},
	}}
	got := calcPurchases(in, 2)
	if got[0] != 300 {
		t.Errorf("месяц 1: получено %.2f, ожидалось 300", got[0])
	}
	if got[1] != 0 {
		t.Errorf("месяц 2: получено %.2f, ожидался 0", got[1])
	}
}

// Длительность проекта сократили после сохранения — покупка за пределами
// проекта отбрасывается, а не паникует по индексу.
func TestCalcPurchasesDropsMonthBeyondProject(t *testing.T) {
	in := &InputPurchases{Items: []ItemPurchase{
		{Name: "за пределами", Month: 9, Count: 1, Price: 1000},
	}}
	got := calcPurchases(in, 3)
	for m, v := range got {
		if v != 0 {
			t.Errorf("месяц %d: получено %.2f, ожидался 0", m+1, v)
		}
	}
}

func TestCalcPurchasesNilAndEmpty(t *testing.T) {
	if got := calcPurchases(nil, 3); len(got) != 3 || got[0] != 0 {
		t.Errorf("nil-ввод: получено %v", got)
	}
	if got := calcPurchases(&InputPurchases{}, 3); len(got) != 3 || got[0] != 0 {
		t.Errorf("пустая таблица: получено %v", got)
	}
}

func TestValidatePurchasesRejectsMonthBeyondProject(t *testing.T) {
	in := &InputPurchases{Items: []ItemPurchase{{Month: 7, Count: 1, Price: 100}}}
	err := ValidatePurchases("приборы строительного контроля", in, 6)
	if err == nil {
		t.Fatal("месяц за пределами проекта должен отклоняться")
	}
	t.Log(err)
}

// Последний месяц проекта валиден — это и отличает 4.7 от 4.4.
func TestValidatePurchasesAcceptsLastMonth(t *testing.T) {
	in := &InputPurchases{Items: []ItemPurchase{{Month: 6, Count: 1, Price: 100}}}
	if err := ValidatePurchases("приборы строительного контроля", in, 6); err != nil {
		t.Errorf("последний месяц отклонён: %v", err)
	}
}

func TestValidatePurchasesRejectsNegative(t *testing.T) {
	neg := &InputPurchases{Items: []ItemPurchase{{Month: 1, Count: 1, Price: -1}}}
	if err := ValidatePurchases("корпоративные мероприятия", neg, 6); err == nil {
		t.Error("отрицательная цена должна отклоняться")
	}
	negCount := &InputPurchases{Items: []ItemPurchase{{Month: 1, Count: -2, Price: 1}}}
	if err := ValidatePurchases("корпоративные мероприятия", negCount, 6); err == nil {
		t.Error("отрицательное количество должно отклоняться")
	}
}

// Пустая строка (месяц 0) не должна валить валидацию — таблица во время
// заполнения почти всегда содержит такую.
func TestValidatePurchasesSkipsEmptyRow(t *testing.T) {
	in := &InputPurchases{Items: []ItemPurchase{{Month: 0, Count: 0, Price: 0}}}
	if err := ValidatePurchases("приборы строительного контроля", in, 6); err != nil {
		t.Errorf("пустая строка отклонена: %v", err)
	}
}

// ── Попадание в строки бюджета ──────────────────────────────────────────────
//
// 189 (приборы) и 205 (корпоративы) разнесены далеко друг от друга, между
// ними полтора десятка чужих статей — проверяем, что каждый лист попал
// ровно в свою и не задел соседей.

func TestEquipmentReachesOverheadRow189(t *testing.T) {
	inp := purchasesInputs(6)
	inp.EquipmentItems = referenceEquipmentInput()

	res := Run(inp)
	want := []float64{90000, 0, 120000, 0, 0, 210000}
	for m, w := range want {
		if got := res.Monthly[m].Overhead[11]; got != w {
			t.Errorf("месяц %d, строка 189: получено %.2f, ожидалось %.2f", m+1, got, w)
		}
		if got := res.Monthly[m].Overhead[27]; got != 0 {
			t.Errorf("месяц %d, строка 205 не должна меняться, получено %.2f", m+1, got)
		}
	}
}

func TestCorporateEventsReachOverheadRow205(t *testing.T) {
	inp := purchasesInputs(6)
	// Корпоратива два: летний и новогодний, у каждого свои участники и цена.
	inp.CorporateEventItems = &InputPurchases{Items: []ItemPurchase{
		{Month: 2, Count: 25, Price: 4000},
		{Month: 5, Count: 30, Price: 6500},
	}}

	res := Run(inp)
	want := []float64{0, 100000, 0, 0, 195000, 0}
	for m, w := range want {
		if got := res.Monthly[m].Overhead[27]; got != w {
			t.Errorf("месяц %d, строка 205: получено %.2f, ожидалось %.2f", m+1, got, w)
		}
		if got := res.Monthly[m].Overhead[11]; got != 0 {
			t.Errorf("месяц %d, строка 189 не должна меняться, получено %.2f", m+1, got)
		}
	}
}

// Корпоративов может не быть совсем — пустая таблица даёт 0 расходов и не
// ломает расчёт.
func TestCorporateEventsEmptyIsZero(t *testing.T) {
	inp := purchasesInputs(3)
	inp.CorporateEventItems = &InputPurchases{}

	res := Run(inp)
	for m := range res.Monthly {
		if got := res.Monthly[m].Overhead[27]; got != 0 {
			t.Errorf("месяц %d, строка 205: получено %.2f, ожидался 0", m+1, got)
		}
	}
}

func TestPurchasesLegacyFallback(t *testing.T) {
	inp := purchasesInputs(3)
	inp.ControlEquipment = []float64{10, 20, 30}
	inp.CorporateEvents = []float64{40, 50, 60}

	res := Run(inp)
	for m, w := range []float64{10, 20, 30} {
		if got := res.Monthly[m].Overhead[11]; got != w {
			t.Errorf("месяц %d, строка 189: получено %.2f, ожидалось %.2f", m+1, got, w)
		}
	}
	for m, w := range []float64{40, 50, 60} {
		if got := res.Monthly[m].Overhead[27]; got != w {
			t.Errorf("месяц %d, строка 205: получено %.2f, ожидалось %.2f", m+1, got, w)
		}
	}
}

func TestValidateInputPurchasesTitles(t *testing.T) {
	bad := []byte(`{"items":[{"month":99,"count":1,"price":1}]}`)
	cases := map[string]string{
		TypeEquipmentItems:       "приборы строительного контроля",
		TypeCorporateEventsItems: "корпоративные мероприятия",
	}
	for typ, want := range cases {
		err := ValidateInput(typ, bad, ExecutorAibicon, 6)
		if err == nil {
			t.Errorf("%s: месяц вне проекта должен отклоняться", typ)
			continue
		}
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%s: в ошибке нет «%s»: %v", typ, want, err)
		}
	}
}
