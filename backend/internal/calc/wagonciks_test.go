package calc

import (
	"math"
	"strings"
	"testing"
)

// Эталонные данные листа 4.4.
func referenceWagonciksInput() *InputWagonciks {
	return &InputWagonciks{
		Rental: MonthlyQty{Price: 5_000, Counts: []int{0, 2, 1, 1, 0, 0}},
		// 4.4!A12:C12 — строка в строку с таблицей формы
		Purchases: []ItemPurchase{{Name: "Вагончик 1", Month: 1, Count: 2, Price: 55_500}},
	}
}

// TestCalcWagonciks_Reference — численная сверка с эталоном.
// Ожидаемые значения — 4.4!C6:BJ6 (они же 2.Бюджет!H181:BO181) и 4.4!BK5.
func TestCalcWagonciks_Reference(t *testing.T) {
	got := calcWagonciks(referenceWagonciksInput(), 6)

	// 4.4!C6, D6, E6, F6, G6, BJ6
	want := []float64{
		111_000, // 2×55 500 покупка, аренды нет
		10_000,  // 2×5 000
		5_000,   // 1×5 000
		5_000,   // 1×5 000
		0,
		0,
	}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 0.01 {
			t.Errorf("вагончики (181), месяц %d: want %.2f, got %.2f", i+1, want[i], got[i])
		}
	}
	// 4.4!BK5 = 2.Бюджет!G181
	if s := sumFloats(got); math.Abs(s-131_000) > 0.01 {
		t.Errorf("итого (4.4!BK5): want 131 000, got %.2f", s)
	}
}

// TestCalcWagonciks_ReferenceAP — сверка с calc_sheets_ibcon-project-
// russia.xlsm после того, как владелец дозаполнил лист 2026-08-26.
func TestCalcWagonciks_ReferenceAP(t *testing.T) {
	got := calcWagonciks(&InputWagonciks{
		Rental: MonthlyQty{Price: 5_000, Counts: []int{0, 2, 1, 1, 2, 2}},
		// 4.4!A12:C18 — семь строк таблицы формы, у каждой своя цена
		Purchases: []ItemPurchase{
			{Name: "Вагончик 1", Month: 1, Count: 2, Price: 55_500},
			{Name: "Вагончик 2", Month: 1, Count: 3, Price: 10_000},
			{Name: "Вагончик 3", Month: 2, Count: 3, Price: 20_000},
			{Name: "Вагончик 4", Month: 3, Count: 4, Price: 10_000},
			{Name: "Вагончик 5", Month: 4, Count: 5, Price: 30_000},
			{Name: "Вагончик 6", Month: 5, Count: 6, Price: 40_000},
			{Name: "Вагончик 7", Month: 6, Count: 7, Price: 50_000},
		},
	}, 6)

	// 4.4!C6:BJ6 = 2.Бюджет!H181:BO181
	want := []float64{141_000, 70_000, 45_000, 155_000, 250_000, 360_000}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 0.01 {
			t.Errorf("месяц %d: want %.2f, got %.2f", i+1, want[i], got[i])
		}
	}
	if s := sumFloats(got); math.Abs(s-1_021_000) > 0.01 {
		t.Errorf("итого (4.4!BK5 = G181): want 1 021 000, got %.2f", s)
	}
}

// TestCalcWagonciks_InBudget — те же суммы через полный расчёт: проверяем,
// что 4.4 попадает именно в строку 181 (индекс 3 массива Overhead).
func TestCalcWagonciks_InBudget(t *testing.T) {
	res := Run(&BudgetInputs{
		DurationMonths: 6,
		ExecutorName:   ExecutorAibiconProject,
		Wagonciks:      referenceWagonciksInput(),
	})

	want := []float64{111_000, 10_000, 5_000, 5_000, 0, 0}
	for m := range want {
		if got := res.Monthly[m].Overhead[3]; math.Abs(got-want[m]) > 0.01 {
			t.Errorf("2.Бюджет!181, месяц %d: want %.2f, got %.2f", m+1, want[m], got)
		}
	}
}

// TestCalcWagonciks_SplitFormulaFixed — исправление расщеплённой формулы.
func TestCalcWagonciks_SplitFormulaFixed(t *testing.T) {
	// Покупка в 4-м месяце проекта длиной 6. По форме этот месяц лежит в
	// колонке F, то есть в рабочей ветке, но проверяем именно «не только
	// первый месяц».
	got := calcWagonciks(&InputWagonciks{
		Purchases: []ItemPurchase{{Month: 4, Count: 3, Price: 100_000}},
	}, 6)

	if got[3] != 300_000 {
		t.Errorf("покупка в 4-м месяце: want 300 000, got %.2f", got[3])
	}
	if s := sumFloats(got); s != 300_000 {
		t.Errorf("больше нигде начисляться не должно: want 300 000, got %.2f", s)
	}

	// И покупка в разных месяцах сразу — форма считала бы только первую
	// строку таблицы ($D$12), здесь считаются все месяцы.
	got = calcWagonciks(&InputWagonciks{
		Purchases: []ItemPurchase{
			{Month: 1, Count: 1, Price: 10},
			{Month: 2, Count: 2, Price: 10},
			{Month: 3, Count: 3, Price: 10},
			{Month: 4, Count: 4, Price: 10},
		},
	}, 6)
	for i, w := range []float64{10, 20, 30, 40, 0, 0} {
		if got[i] != w {
			t.Errorf("месяц %d: want %.0f, got %.0f", i+1, w, got[i])
		}
	}
}

// TestCalcWagonciks_PurchaseAnyMonth — покупка разрешена в любом месяце
// проекта, включая последние два. Ограничение «1…D−2» отменено владельцем
// 2026-08-28; тест держит отмену, чтобы правило не вернулось молча.
func TestCalcWagonciks_PurchaseAnyMonth(t *testing.T) {
	for _, month := range []int{1, 5, 6} {
		got := calcWagonciks(&InputWagonciks{
			Purchases: []ItemPurchase{{Month: month, Count: 2, Price: 1_000}},
		}, 6)
		if got[month-1] != 2_000 {
			t.Errorf("покупка в месяце %d: want 2 000, got %.0f", month, got[month-1])
		}
		if s := sumFloats(got); s != 2_000 {
			t.Errorf("покупка в месяце %d: больше нигде начисляться не должно, got %.0f", month, s)
		}
	}

	// Проект в один месяц — покупка в нём доступна.
	got := calcWagonciks(&InputWagonciks{
		Purchases: []ItemPurchase{{Month: 1, Count: 1, Price: 500}},
	}, 1)
	if got[0] != 500 {
		t.Errorf("проект в 1 месяц: want 500, got %.0f", got[0])
	}
}

// TestCalcWagonciks_RentalAndPurchaseTogether — аренда и покупка в одном
// месяце складываются в одну строку бюджета (4.4!строка 6), и так во всех
// месяцах проекта без изъятий.
func TestCalcWagonciks_RentalAndPurchaseTogether(t *testing.T) {
	got := calcWagonciks(&InputWagonciks{
		Rental: MonthlyQty{Price: 1_000, Counts: []int{1, 1, 1, 1, 1, 1}},
		Purchases: []ItemPurchase{
			{Month: 1, Count: 1, Price: 100_000},
			{Month: 2, Count: 1, Price: 100_000},
			{Month: 3, Count: 1, Price: 100_000},
			{Month: 4, Count: 1, Price: 100_000},
			{Month: 5, Count: 1, Price: 100_000},
			{Month: 6, Count: 1, Price: 100_000},
		},
	}, 6)

	for i := 0; i < 6; i++ {
		if got[i] != 101_000 {
			t.Errorf("месяц %d: want 101 000, got %.0f", i+1, got[i])
		}
	}

	// Проект в 1 месяц: покупка разрешена.
	got = calcWagonciks(&InputWagonciks{
		Rental:    MonthlyQty{Price: 1_000, Counts: []int{1}},
		Purchases: []ItemPurchase{{Month: 1, Count: 1, Price: 100_000}},
	}, 1)
	if got[0] != 101_000 {
		t.Errorf("проект в 1 месяц: want 101 000, got %.0f", got[0])
	}
}

// TestCalcWagonciks_CountsLength — массив количеств может не совпасть по
// длине с проектом: короткий добивается нулями, длинный обрезается.
func TestCalcWagonciks_CountsLength(t *testing.T) {
	got := calcWagonciks(&InputWagonciks{
		Rental: MonthlyQty{Price: 1_000, Counts: []int{1}},
	}, 3)
	for i, w := range []float64{1_000, 0, 0} {
		if got[i] != w {
			t.Errorf("короткий массив, месяц %d: want %.0f, got %.0f", i+1, w, got[i])
		}
	}

	got = calcWagonciks(&InputWagonciks{
		Rental: MonthlyQty{Price: 1_000, Counts: []int{1, 1, 1, 9, 9}},
	}, 3)
	if s := sumFloats(got); s != 3_000 {
		t.Errorf("длинный массив должен обрезаться: want 3 000, got %.0f", s)
	}
}

// TestCalcWagonciks_LegacyFallback — версии, сохранённые до перехода 4.4 на
// расчёт по формуле, продолжают считаться по старым готовым суммам.
func TestCalcWagonciks_LegacyFallback(t *testing.T) {
	res := Run(&BudgetInputs{
		DurationMonths: 3,
		ExecutorName:   ExecutorAibicon,
		SiteSetup:      []float64{100, 200, 300},
	})
	for m, want := range []float64{100, 200, 300} {
		if got := res.Monthly[m].Overhead[3]; got != want {
			t.Errorf("старый ввод 181, месяц %d: want %.0f, got %.0f", m+1, want, got)
		}
	}

	// Появился новый ввод — старые суммы игнорируются.
	res = Run(&BudgetInputs{
		DurationMonths: 3,
		ExecutorName:   ExecutorAibicon,
		SiteSetup:      []float64{100, 200, 300},
		Wagonciks: &InputWagonciks{
			Rental: MonthlyQty{Price: 7, Counts: []int{1, 0, 0}},
		},
	})
	if got := res.Monthly[0].Overhead[3]; got != 7 {
		t.Errorf("новый ввод должен перекрывать старый: want 7, got %.0f", got)
	}
	if got := res.Monthly[1].Overhead[3]; got != 0 {
		t.Errorf("старые суммы не должны просачиваться: want 0, got %.0f", got)
	}
}

// TestCalcWagonciks_Empty — пустой ввод и нулевая длительность не роняют расчёт.
func TestCalcWagonciks_Empty(t *testing.T) {
	if got := calcWagonciks(nil, 4); len(got) != 4 || sumFloats(got) != 0 {
		t.Errorf("пустой ввод: want 4 нуля, got %v", got)
	}
	if got := calcWagonciks(referenceWagonciksInput(), 0); len(got) != 0 {
		t.Errorf("нулевая длительность: want пустой массив, got %v", got)
	}
}

// TestValidateWagonciks — отрицательные цены и количества отклоняем,
// количество покупки в закрытом месяце ошибкой не считаем.
func TestValidateWagonciks(t *testing.T) {
	if err := ValidateWagonciks(referenceWagonciksInput(), 6); err != nil {
		t.Errorf("эталонный ввод должен проходить валидацию: %v", err)
	}

	cases := []struct {
		name string
		in   *InputWagonciks
		want string
	}{
		{
			name: "покупка без месяца — строка просто игнорируется",
			in:   &InputWagonciks{Purchases: []ItemPurchase{{Count: 1, Price: 100}}},
		},
		{
			name: "покупка в последнем месяце проекта — разрешена",
			in: &InputWagonciks{Purchases: []ItemPurchase{
				{Name: "Бытовка", Month: 6, Count: 1, Price: 100},
			}},
		},
		{
			name: "покупка за пределами проекта",
			in:   &InputWagonciks{Purchases: []ItemPurchase{{Month: 9, Count: 1, Price: 100}}},
			want: "допустимо от 1 до 6",
		},
		{
			name: "отрицательная цена аренды",
			in:   &InputWagonciks{Rental: MonthlyQty{Price: -1}},
			want: "аренда вагончиков: цена не может быть отрицательной",
		},
		{
			name: "отрицательное количество аренды",
			in:   &InputWagonciks{Rental: MonthlyQty{Price: 1, Counts: []int{1, -2}}},
			want: "количество в месяце 2 не может быть отрицательным",
		},
		{
			name: "отрицательное количество покупки",
			in:   &InputWagonciks{Purchases: []ItemPurchase{{Month: 1, Count: -1, Price: 1}}},
			want: "покупка вагончиков",
		},
	}

	for _, c := range cases {
		err := ValidateWagonciks(c.in, 6)
		switch {
		case c.want == "" && err != nil:
			t.Errorf("%s: ожидалось без ошибки, получено %v", c.name, err)
		case c.want != "" && err == nil:
			t.Errorf("%s: ожидалась ошибка со словами «%s», ошибки нет", c.name, c.want)
		case c.want != "" && err != nil && !strings.Contains(err.Error(), c.want):
			t.Errorf("%s: ожидалась ошибка со словами «%s», получено %v", c.name, c.want, err)
		}
	}
}

// TestValidateInput_Wagonciks — тот же контроль через диспетчер сохранения.
func TestValidateInput_Wagonciks(t *testing.T) {
	ok := []byte(`{"rental":{"price":5000,"counts":[0,2,1]},` +
		`"purchases":[{"name":"Бытовка","month":1,"count":2,"price":55500}]}`)
	if err := ValidateInput(TypeWagonciks, ok, ExecutorAibicon, 6); err != nil {
		t.Errorf("корректный ввод отклонён: %v", err)
	}

	bad := []byte(`{"rental":{"price":-1,"counts":[]}}`)
	if err := ValidateInput(TypeWagonciks, bad, ExecutorAibicon, 6); err == nil {
		t.Error("отрицательная цена должна отклоняться")
	}

	// Месяц 6 при длительности 6 — последний месяц, покупка разрешена.
	tail := []byte(`{"purchases":[{"month":6,"count":1,"price":100}]}`)
	if err := ValidateInput(TypeWagonciks, tail, ExecutorAibicon, 6); err != nil {
		t.Errorf("покупка в последнем месяце должна проходить: %v", err)
	}

	// Месяц 7 при длительности 6 — за пределами проекта.
	out := []byte(`{"purchases":[{"month":7,"count":1,"price":100}]}`)
	if err := ValidateInput(TypeWagonciks, out, ExecutorAibicon, 6); err == nil {
		t.Error("покупка вне проекта должна отклоняться")
	}

	if err := ValidateInput(TypeWagonciks, []byte(`{"rental":`), ExecutorAibicon, 6); err == nil {
		t.Error("битый JSON должен отклоняться")
	}
}
