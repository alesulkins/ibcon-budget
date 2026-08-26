package calc

import (
	"math"
	"strings"
	"testing"
)

// Эталонные данные листа 4.5. Формулы во всех трёх файлах идентичны, а
// данные различаются — и это важно: они закрывают обе ветки листа.
// Длительность проекта 2.Бюджет!D8 = 6 во всех файлах.
//
//	цены офисов   4.5!B15:B17
//	количества    4.5!C15:BJ17
//	уборка        4.5!B11 = 20 000 (стоимость ОДНОГО офиса за месяц)

// один офис за 125 000 все шесть месяцев — файлы «Айбикон» и «Айбикон Киргизия»
func referenceOfficeSingle() *InputOffice {
	return &InputOffice{
		Offices: []RentedItem{
			{Name: "Офис 1", Price: 125_000, Counts: []int{1, 1, 1, 1, 1, 1}},
		},
		CleaningPrice: 20_000,
	}
}

// два офиса с пятого месяца — файл «Айбикон-Проект»
func referenceOfficeTwo() *InputOffice {
	return &InputOffice{
		Offices: []RentedItem{
			{Name: "Офис 1", Price: 125_000, Counts: []int{1, 1, 1, 1, 1, 1}},
			{Name: "Офис 2", Price: 90_000, Counts: []int{0, 0, 0, 0, 1, 1}},
		},
		CleaningPrice: 20_000,
	}
}

// TestCalcOffice_Aibicon — российский исполнитель: аренда делится на 0.87.
// Эталон: calc_sheets_ibcon-russia.xlsm, 4.5!C5:BJ5 и 4.5!C11:BJ11.
func TestCalcOffice_Aibicon(t *testing.T) {
	rent, cleaning := calcOffice(referenceOfficeSingle(), ExecutorAibicon, 6)

	// 125 000 / 0.87 в каждом из шести месяцев
	const want = 143_678.16091954024
	for m := 0; m < 6; m++ {
		if math.Abs(rent[m]-want) > 0.01 {
			t.Errorf("аренда (182), месяц %d: want %.4f, got %.4f", m+1, want, rent[m])
		}
		if cleaning[m] != 20_000 {
			t.Errorf("уборка (183), месяц %d: want 20 000, got %.2f", m+1, cleaning[m])
		}
	}
	// 4.5!BK5 = 2.Бюджет!G182, 4.5!BK11 = 2.Бюджет!G183
	if s := sumFloats(rent); math.Abs(s-862_068.9655172414) > 0.01 {
		t.Errorf("итого аренда (4.5!BK5): want 862 068.97, got %.4f", s)
	}
	if s := sumFloats(cleaning); math.Abs(s-120_000) > 0.01 {
		t.Errorf("итого уборка (4.5!BK11): want 120 000, got %.2f", s)
	}
}

// TestCalcOffice_Kirgizia — киргизский филиал: gross-up НЕ применяется.
// Тот же ввод, что у «Айбикон», результат отличается ровно на 0.87.
// Эталон: calc_sheets_ibcon-kirgizia.xlsm.
func TestCalcOffice_Kirgizia(t *testing.T) {
	rent, cleaning := calcOffice(referenceOfficeSingle(), ExecutorAibiconKG, 6)

	for m := 0; m < 6; m++ {
		if rent[m] != 125_000 {
			t.Errorf("аренда (182), месяц %d: want 125 000 без gross-up, got %.4f", m+1, rent[m])
		}
	}
	if s := sumFloats(rent); math.Abs(s-750_000) > 0.01 {
		t.Errorf("итого аренда (4.5!BK5): want 750 000, got %.4f", s)
	}
	// Уборка одинакова для всех исполнителей — в форме её на 0.87 не делят.
	if s := sumFloats(cleaning); math.Abs(s-120_000) > 0.01 {
		t.Errorf("итого уборка (4.5!BK11): want 120 000, got %.2f", s)
	}
}

// TestCalcOffice_AibiconProject — два офиса с разными ценами, второй
// подключается с пятого месяца. Заодно проверяет, что уборка реагирует на
// количество офисов: 20 000 → 40 000.
// Эталон: calc_sheets_ibcon-project-russia.xlsm.
func TestCalcOffice_AibiconProject(t *testing.T) {
	rent, cleaning := calcOffice(referenceOfficeTwo(), ExecutorAibiconProject, 6)

	// 4.5!C5:BJ5 — (125 000)/0.87 и (125 000+90 000)/0.87
	wantRent := []float64{
		143_678.16091954024,
		143_678.16091954024,
		143_678.16091954024,
		143_678.16091954024,
		247_126.4367816092,
		247_126.4367816092,
	}
	// 4.5!C11:BJ11 — количество офисов × 20 000
	wantCleaning := []float64{20_000, 20_000, 20_000, 20_000, 40_000, 40_000}

	for m := range wantRent {
		if math.Abs(rent[m]-wantRent[m]) > 0.01 {
			t.Errorf("аренда (182), месяц %d: want %.4f, got %.4f", m+1, wantRent[m], rent[m])
		}
		if math.Abs(cleaning[m]-wantCleaning[m]) > 0.01 {
			t.Errorf("уборка (183), месяц %d: want %.2f, got %.2f", m+1, wantCleaning[m], cleaning[m])
		}
	}
	if s := sumFloats(rent); math.Abs(s-1_068_965.5172413792) > 0.01 {
		t.Errorf("итого аренда (4.5!BK5): want 1 068 965.52, got %.4f", s)
	}
	if s := sumFloats(cleaning); math.Abs(s-160_000) > 0.01 {
		t.Errorf("итого уборка (4.5!BK11): want 160 000, got %.2f", s)
	}
}

// TestCalcOffice_InBudget — те же суммы через полный расчёт: проверяем, что
// 4.5 попадает в строки 182 и 183 (индексы 4 и 5 массива Overhead).
func TestCalcOffice_InBudget(t *testing.T) {
	res := Run(&BudgetInputs{
		DurationMonths: 6,
		ExecutorName:   ExecutorAibiconProject,
		Office:         referenceOfficeTwo(),
	})

	wantRent := []float64{
		143_678.16091954024, 143_678.16091954024, 143_678.16091954024,
		143_678.16091954024, 247_126.4367816092, 247_126.4367816092,
	}
	wantCleaning := []float64{20_000, 20_000, 20_000, 20_000, 40_000, 40_000}

	for m := 0; m < 6; m++ {
		if got := res.Monthly[m].Overhead[4]; math.Abs(got-wantRent[m]) > 0.01 {
			t.Errorf("2.Бюджет!182, месяц %d: want %.4f, got %.4f", m+1, wantRent[m], got)
		}
		if got := res.Monthly[m].Overhead[5]; math.Abs(got-wantCleaning[m]) > 0.01 {
			t.Errorf("2.Бюджет!183, месяц %d: want %.2f, got %.2f", m+1, wantCleaning[m], got)
		}
	}
}

// TestCalcOffice_CleaningFollowsCount — уборка это НЕ фиксированная сумма,
// а «цена одного офиса × количество офисов в месяце» (4.5!C11 = C19*$B$11).
// Ключевое отличие от того, как это описано в ТЗ.
func TestCalcOffice_CleaningFollowsCount(t *testing.T) {
	_, cleaning := calcOffice(&InputOffice{
		Offices: []RentedItem{
			{Price: 1, Counts: []int{0, 1, 2}},
			{Price: 1, Counts: []int{0, 1, 1}},
		},
		CleaningPrice: 5_000,
	}, ExecutorAibicon, 3)

	// офисов в месяце: 0, 2, 3
	for m, want := range []float64{0, 10_000, 15_000} {
		if cleaning[m] != want {
			t.Errorf("месяц %d: want %.0f, got %.0f", m+1, want, cleaning[m])
		}
	}
}

// TestCalcOffice_CountsLength — массив количеств может не совпасть по длине
// с проектом: короткий добивается нулями, длинный обрезается.
func TestCalcOffice_CountsLength(t *testing.T) {
	rent, cleaning := calcOffice(&InputOffice{
		Offices: []RentedItem{
			{Price: 87, Counts: []int{1}},             // короче проекта
			{Price: 87, Counts: []int{0, 0, 0, 5, 9}}, // длиннее проекта
		},
		CleaningPrice: 100,
	}, ExecutorAibiconKG, 3)

	for m, want := range []float64{87, 0, 0} {
		if rent[m] != want {
			t.Errorf("аренда, месяц %d: want %.0f, got %.0f", m+1, want, rent[m])
		}
	}
	for m, want := range []float64{100, 0, 0} {
		if cleaning[m] != want {
			t.Errorf("уборка, месяц %d: want %.0f, got %.0f", m+1, want, cleaning[m])
		}
	}
}

// TestCalcOffice_LegacyFallback — версии, сохранённые до перехода 4.5 на
// расчёт по формуле, продолжают считаться по старым готовым суммам.
func TestCalcOffice_LegacyFallback(t *testing.T) {
	res := Run(&BudgetInputs{
		DurationMonths: 3,
		ExecutorName:   ExecutorAibicon,
		OfficeRent:     []float64{100, 200, 300},
		OfficeCleaning: []float64{10, 20, 30},
	})
	for m, want := range []float64{100, 200, 300} {
		if got := res.Monthly[m].Overhead[4]; got != want {
			t.Errorf("старый ввод 182, месяц %d: want %.0f, got %.0f", m+1, want, got)
		}
	}
	for m, want := range []float64{10, 20, 30} {
		if got := res.Monthly[m].Overhead[5]; got != want {
			t.Errorf("старый ввод 183, месяц %d: want %.0f, got %.0f", m+1, want, got)
		}
	}

	// Появился новый ввод — старые суммы игнорируются.
	res = Run(&BudgetInputs{
		DurationMonths: 3,
		ExecutorName:   ExecutorAibiconKG,
		OfficeRent:     []float64{100, 200, 300},
		OfficeCleaning: []float64{10, 20, 30},
		Office: &InputOffice{
			Offices:       []RentedItem{{Price: 7, Counts: []int{1, 0, 0}}},
			CleaningPrice: 3,
		},
	})
	if got := res.Monthly[0].Overhead[4]; got != 7 {
		t.Errorf("новый ввод должен перекрывать старый: want 7, got %.0f", got)
	}
	if got := res.Monthly[1].Overhead[5]; got != 0 {
		t.Errorf("старая уборка не должна просачиваться: want 0, got %.0f", got)
	}
}

// TestCalcOffice_Empty — пустой ввод и нулевая длительность не роняют расчёт.
func TestCalcOffice_Empty(t *testing.T) {
	rent, cleaning := calcOffice(nil, ExecutorAibicon, 4)
	if len(rent) != 4 || len(cleaning) != 4 {
		t.Fatalf("длина массивов: want 4/4, got %d/%d", len(rent), len(cleaning))
	}
	if sumFloats(rent) != 0 || sumFloats(cleaning) != 0 {
		t.Error("пустой ввод должен давать нули")
	}
	if rent, cleaning = calcOffice(referenceOfficeTwo(), ExecutorAibicon, 0); len(rent) != 0 || len(cleaning) != 0 {
		t.Error("нулевая длительность должна давать пустые массивы")
	}
}

// TestValidateOffice — отрицательные цены и количества отклоняем.
func TestValidateOffice(t *testing.T) {
	if err := ValidateOffice(referenceOfficeTwo()); err != nil {
		t.Errorf("эталонный ввод должен проходить валидацию: %v", err)
	}

	cases := []struct {
		name string
		in   *InputOffice
		want string
	}{
		{
			name: "нулевые количества допустимы",
			in:   &InputOffice{Offices: []RentedItem{{Price: 1, Counts: []int{0, 0}}}},
		},
		{
			name: "отрицательная стоимость уборки",
			in:   &InputOffice{CleaningPrice: -1},
			want: "уборка офиса: стоимость не может быть отрицательной",
		},
		{
			name: "отрицательная цена офиса",
			in:   &InputOffice{Offices: []RentedItem{{Name: "Офис", Price: -1}}},
			want: "аренда офиса, строка 1 (Офис): цена не может быть отрицательной",
		},
		{
			name: "отрицательное количество офисов",
			in:   &InputOffice{Offices: []RentedItem{{Price: 1, Counts: []int{1, -2}}}},
			want: "количество в месяце 2 не может быть отрицательным",
		},
	}

	for _, c := range cases {
		err := ValidateOffice(c.in)
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

// TestValidateInput_Office — тот же контроль через диспетчер сохранения.
func TestValidateInput_Office(t *testing.T) {
	ok := []byte(`{"offices":[{"name":"Офис 1","price":125000,"counts":[1,1,1]}],"cleaning_price":20000}`)
	if err := ValidateInput(TypeOffice, ok, ExecutorAibicon, 6); err != nil {
		t.Errorf("корректный ввод отклонён: %v", err)
	}

	bad := []byte(`{"cleaning_price":-1}`)
	if err := ValidateInput(TypeOffice, bad, ExecutorAibicon, 6); err == nil {
		t.Error("отрицательная стоимость уборки должна отклоняться")
	}

	if err := ValidateInput(TypeOffice, []byte(`{"offices":`), ExecutorAibicon, 6); err == nil {
		t.Error("битый JSON должен отклоняться")
	}
}
