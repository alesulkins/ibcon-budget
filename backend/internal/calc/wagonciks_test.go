package calc

import (
	"math"
	"strings"
	"testing"
)

// Эталонные данные листа 4.4. В calc_sheets_ibcon-project-russia.xlsm,
// calc_sheets_ibcon-russia.xlsm и calc_sheets_ibcon-kirgizia.xlsm они
// одинаковы (проверено 2026-08-26), формулы тоже. Длительность D8 = 6.
//
// Что стоит в форме:
//
//	аренда  4.4!B6  = 5 000, кол-во 4.4!C5:BJ5 = 0, 2, 1, 1, 0, 0
//	покупка 4.4!A12:C12 = мес.1 × 2 шт × 55 500
func referenceWagonciksInput() *InputWagonciks {
	return &InputWagonciks{
		Rental:   MonthlyQty{Price: 5_000, Counts: []int{0, 2, 1, 1, 0, 0}},
		Purchase: MonthlyQty{Price: 55_500, Counts: []int{2, 0, 0, 0, 0, 0}},
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

// TestCalcWagonciks_ReferenceAP — сверка с calc_sheets_ibcon-project-russia.xlsm
// после того, как владелец дозаполнил лист 2026-08-26. Этот файл — самый
// интересный: в нём семь покупок в разных месяцах, включая последние два,
// и заполнена аренда во всех месяцах.
//
// Данные формы:
//
//	аренда  4.4!B6 = 5 000, кол-во 4.4!C5:BJ5 = 0, 2, 1, 1, 2, 2
//	покупки 4.4!A12:C18 — мес.1: 2×55 500 и 3×10 000; мес.2: 3×20 000;
//	                      мес.3: 4×10 000; мес.4: 5×30 000;
//	                      мес.5: 6×40 000; мес.6: 7×50 000
//
// ОГОВОРКА про представление. Согласованная модель ввода держит ОДНУ цену
// покупки на весь блок, а в форме у каждой из семи строк своя цена. Чтобы
// сверить суммы, представляем месячные суммы покупок как 1 000 × количество:
// 141 000 = 1 000×141 и так далее. Числа от этого не меняются, но сам факт,
// что данные формы в модель не укладываются, вынесен владельцу отдельно.
func TestCalcWagonciks_ReferenceAP(t *testing.T) {
	got := calcWagonciks(&InputWagonciks{
		Rental:   MonthlyQty{Price: 5_000, Counts: []int{0, 2, 1, 1, 2, 2}},
		Purchase: MonthlyQty{Price: 1_000, Counts: []int{141, 60, 40, 150, 240, 350}},
	}, 6)

	// 4.4!C6:BJ6 = 2.Бюджет!H181:BO181.
	// В месяцах 5-6 покупка обнулена: правилом «последние два месяца» у нас,
	// вырожденной веткой IF(месяц=1) — в форме. Итог совпадает.
	want := []float64{141_000, 70_000, 45_000, 155_000, 10_000, 10_000}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 0.01 {
			t.Errorf("месяц %d: want %.2f, got %.2f", i+1, want[i], got[i])
		}
	}
	if s := sumFloats(got); math.Abs(s-431_000) > 0.01 {
		t.Errorf("итого (4.4!BK5 = G181): want 431 000, got %.2f", s)
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
//
// В форме полноценная формула с SUMPRODUCT стоит только в колонках C:K, а в
// L:BJ — вырожденная IF(месяц=1, $D$12, 0). С учётом редиректа колонок это
// значит, что покупка вне первого месяца в поздних колонках терялась.
// Здесь формула одна на все месяцы: покупка считается в том месяце, в
// котором указана.
func TestCalcWagonciks_SplitFormulaFixed(t *testing.T) {
	// Покупка в 4-м месяце проекта длиной 6 — допустимый месяц (D−2 = 4).
	// По форме этот месяц лежит в колонке F, то есть в рабочей ветке, но
	// проверяем именно «не только первый месяц».
	got := calcWagonciks(&InputWagonciks{
		Purchase: MonthlyQty{Price: 100_000, Counts: []int{0, 0, 0, 3, 0, 0}},
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
		Purchase: MonthlyQty{Price: 10, Counts: []int{1, 2, 3, 4, 0, 0}},
	}, 6)
	for i, w := range []float64{10, 20, 30, 40, 0, 0} {
		if got[i] != w {
			t.Errorf("месяц %d: want %.0f, got %.0f", i+1, w, got[i])
		}
	}
}

// TestPurchaseAllowedMonths — покупка недоступна в последние два месяца
// проекта (правило владельца 2026-08-26) с оговорённым исключением D=1.
func TestPurchaseAllowedMonths(t *testing.T) {
	cases := map[int]int{
		0:  0,
		1:  1, // краевой случай: единственный месяц остаётся открытым
		2:  0, // оба месяца — последние два, покупка недоступна вовсе
		3:  1,
		6:  4,
		12: 10,
	}
	for duration, want := range cases {
		if got := PurchaseAllowedMonths(duration); got != want {
			t.Errorf("длительность %d: want %d открытых месяцев, got %d", duration, want, got)
		}
	}
}

// TestCalcWagonciks_PurchaseBlockedTail — количество покупки в закрытых
// месяцах обнуляется, аренда в них считается как обычно.
func TestCalcWagonciks_PurchaseBlockedTail(t *testing.T) {
	got := calcWagonciks(&InputWagonciks{
		Rental:   MonthlyQty{Price: 1_000, Counts: []int{1, 1, 1, 1, 1, 1}},
		Purchase: MonthlyQty{Price: 100_000, Counts: []int{1, 1, 1, 1, 1, 1}},
	}, 6)

	// месяцы 1-4 — аренда + покупка, месяцы 5-6 — только аренда
	want := []float64{101_000, 101_000, 101_000, 101_000, 1_000, 1_000}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("месяц %d: want %.0f, got %.0f", i+1, want[i], got[i])
		}
	}

	// Проект в 1 месяц: покупка разрешена.
	got = calcWagonciks(&InputWagonciks{
		Rental:   MonthlyQty{Price: 1_000, Counts: []int{1}},
		Purchase: MonthlyQty{Price: 100_000, Counts: []int{1}},
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
	if err := ValidateWagonciks(referenceWagonciksInput()); err != nil {
		t.Errorf("эталонный ввод должен проходить валидацию: %v", err)
	}

	cases := []struct {
		name string
		in   *InputWagonciks
		want string
	}{
		{
			name: "покупка в закрытом месяце — не ошибка, расчёт её обнулит",
			in: &InputWagonciks{
				Purchase: MonthlyQty{Price: 1, Counts: []int{0, 0, 0, 0, 5, 5}},
			},
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
			in:   &InputWagonciks{Purchase: MonthlyQty{Price: 1, Counts: []int{-1}}},
			want: "покупка вагончиков",
		},
	}

	for _, c := range cases {
		err := ValidateWagonciks(c.in)
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
	ok := []byte(`{"rental":{"price":5000,"counts":[0,2,1]},"purchase":{"price":55500,"counts":[2,0,0]}}`)
	if err := ValidateInput(TypeWagonciks, ok, ExecutorAibicon, 6); err != nil {
		t.Errorf("корректный ввод отклонён: %v", err)
	}

	bad := []byte(`{"rental":{"price":-1,"counts":[]}}`)
	if err := ValidateInput(TypeWagonciks, bad, ExecutorAibicon, 6); err == nil {
		t.Error("отрицательная цена должна отклоняться")
	}

	if err := ValidateInput(TypeWagonciks, []byte(`{"rental":`), ExecutorAibicon, 6); err == nil {
		t.Error("битый JSON должен отклоняться")
	}
}
