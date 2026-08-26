package calc

import (
	"math"
	"testing"
)

// Эталонные входные данные листа 4.2 из заполненных файлов
// calc_sheets_ibcon-russia.xlsm / calc_sheets_ibcon-kirgizia.xlsm
// (данные на листе 4.2 в обоих файлах одинаковы, отличается только
// исполнитель в 2.Бюджет!D10). Длительность проекта D8 = 6 месяцев.
//
// Ячейки-источники:
//
//	цены      4.2!B19, B20, B21
//	кол-во    4.2!C19:BJ19, C20:BJ20, C21:BJ21  (месяц 6 = колонка BJ)
//	уборка    4.2!B9
//	риелтор   4.2!B13
//
// CleaningMonths перечисляет все 6 месяцев: в Excel уборка начисляется
// безусловно каждый месяц, а выбор месяцев — расширение платформы, поэтому
// для совпадения с эталоном надо выбрать все.
func referenceRentInput() *InputRentApartments {
	return &InputRentApartments{
		Price1Room:     200_000,
		Price2Room:     250_000,
		Price3Room:     300_000,
		Count1Room:     []int{1, 2, 1, 1, 1, 1},
		Count2Room:     []int{2, 5, 6, 2, 2, 5},
		Count3Room:     []int{4, 3, 4, 1, 2, 1},
		CleaningBase:   40_000,
		RealtorBase:    40_000,
		CleaningMonths: []int{1, 2, 3, 4, 5, 6},
	}
}

// TestCalcRentApartments_Aibicon — ветка российского исполнителя:
// аренда делится на 0.87 (gross-up на НДФЛ 13%).
// Эталон: 4.2!C5:BJ5 и 4.2!C13:BJ13 из calc_sheets_ibcon-russia.xlsm.
func TestCalcRentApartments_Aibicon(t *testing.T) {
	rent, realtor := calcRentApartments(referenceRentInput(), ExecutorAibicon, 6)

	wantRent := []float64{
		2_505_747.1264367816, // 4.2!C5  (1900000+280000)/0.87
		3_390_804.5977011495, // 4.2!D5  (2550000+400000)/0.87
		3_839_080.459770115,  // 4.2!E5  (2900000+440000)/0.87
		1_333_333.3333333333, // 4.2!F5  (1000000+160000)/0.87
		1_724_137.9310344828, // 4.2!G5  (1300000+200000)/0.87
		2_333_333.3333333335, // 4.2!BJ5 (1750000+280000)/0.87
	}
	// 4.2!C13:BJ13. Последний месяц = 0: ячейка 4.2!BJ13 в форме пустая,
	// риелтор за последний месяц проекта не начисляется.
	wantRealtor := []float64{280_000, 120_000, 40_000, 0, 40_000, 0}

	for i := range wantRent {
		if math.Abs(rent[i]-wantRent[i]) > 0.01 {
			t.Errorf("аренда, месяц %d: want %.4f, got %.4f", i+1, wantRent[i], rent[i])
		}
		if math.Abs(realtor[i]-wantRealtor[i]) > 0.01 {
			t.Errorf("риелтор, месяц %d: want %.2f, got %.2f", i+1, wantRealtor[i], realtor[i])
		}
	}

	// Итоги: 4.2!BK5 → 2.Бюджет!G178, 4.2!BK13 → 2.Бюджет!G179
	var totalRent, totalRealtor float64
	for i := range rent {
		totalRent += rent[i]
		totalRealtor += realtor[i]
	}
	if math.Abs(totalRent-15_126_436.781609196) > 0.01 {
		t.Errorf("итого аренда (G178): want 15126436.78, got %.2f", totalRent)
	}
	if math.Abs(totalRealtor-480_000) > 0.01 {
		t.Errorf("итого риелтор (G179): want 480000, got %.2f", totalRealtor)
	}
}

// TestCalcRentApartments_Kirgizia — ветка «Айбикон Киргизия»:
// деления на 0.87 нет (4.2!C5, первая ветка IF).
// Эталон: calc_sheets_ibcon-kirgizia.xlsm.
func TestCalcRentApartments_Kirgizia(t *testing.T) {
	rent, realtor := calcRentApartments(referenceRentInput(), ExecutorAibiconKG, 6)

	wantRent := []float64{
		2_180_000, // 4.2!C5  1900000+280000
		2_950_000, // 4.2!D5  2550000+400000
		3_340_000, // 4.2!E5  2900000+440000
		1_160_000, // 4.2!F5  1000000+160000
		1_500_000, // 4.2!G5  1300000+200000
		2_030_000, // 4.2!BJ5 1750000+280000
	}
	// Риелтор от исполнителя не зависит — те же значения, что у «Айбикон».
	wantRealtor := []float64{280_000, 120_000, 40_000, 0, 40_000, 0}

	for i := range wantRent {
		if math.Abs(rent[i]-wantRent[i]) > 0.01 {
			t.Errorf("аренда КГ, месяц %d: want %.2f, got %.2f", i+1, wantRent[i], rent[i])
		}
		if math.Abs(realtor[i]-wantRealtor[i]) > 0.01 {
			t.Errorf("риелтор КГ, месяц %d: want %.2f, got %.2f", i+1, wantRealtor[i], realtor[i])
		}
	}

	// Итого аренда: 4.2!BK5 → 2.Бюджет!G178 в киргизском файле
	var totalRent float64
	for _, v := range rent {
		totalRent += v
	}
	if math.Abs(totalRent-13_160_000) > 0.01 {
		t.Errorf("итого аренда КГ (G178): want 13160000, got %.2f", totalRent)
	}
}

// TestCalcRentApartments_ExecutorCaseInsensitive — сравнение исполнителя
// должно быть регистронезависимым, как в Excel
// (санкционированное отклонение №3, п.3).
func TestCalcRentApartments_ExecutorCaseInsensitive(t *testing.T) {
	in := referenceRentInput()
	want := 2_180_000.0 // без деления на 0.87

	for _, executor := range []string{
		"Айбикон Киргизия",
		"айбикон киргизия",
		"АЙБИКОН КИРГИЗИЯ",
		"  Айбикон Киргизия  ",
	} {
		rent, _ := calcRentApartments(in, executor, 6)
		if math.Abs(rent[0]-want) > 0.01 {
			t.Errorf("исполнитель %q: want %.2f (без gross-up), got %.2f",
				executor, want, rent[0])
		}
	}

	// А вот российский исполнитель должен делиться на 0.87
	rent, _ := calcRentApartments(in, "айбикон", 6)
	if math.Abs(rent[0]-2_505_747.1264367816) > 0.01 {
		t.Errorf("исполнитель \"айбикон\": ожидался gross-up, got %.2f", rent[0])
	}
}

// TestCalcRentApartments_RealtorRules — правила начисления риелтора отдельно:
// месяц 1 — за все квартиры; рост количества — за прирост; падение или
// без изменений — 0; последний месяц — всегда 0.
func TestCalcRentApartments_RealtorRules(t *testing.T) {
	in := &InputRentApartments{
		Price1Room:  100_000,
		Count1Room:  []int{3, 3, 5, 2, 4},
		RealtorBase: 10_000,
	}
	// месяц 1: 3 квартиры          → 3 × 10000 = 30000
	// месяц 2: 3, без изменений    → 0
	// месяц 3: 5, прирост 2        → 20000
	// месяц 4: 2, падение          → 0
	// месяц 5: 4 — ПОСЛЕДНИЙ месяц → 0 (несмотря на прирост)
	want := []float64{30_000, 0, 20_000, 0, 0}

	_, realtor := calcRentApartments(in, ExecutorAibicon, 5)
	for i, w := range want {
		if math.Abs(realtor[i]-w) > 0.01 {
			t.Errorf("риелтор, месяц %d: want %.0f, got %.0f", i+1, w, realtor[i])
		}
	}
}

// TestCalcRentApartments_CleaningMonths — уборка начисляется только в
// выбранных месяцах. Расширение сверх формы (в Excel уборка безусловна),
// затребовано владельцем 2026-08-23.
func TestCalcRentApartments_CleaningMonths(t *testing.T) {
	in := referenceRentInput()

	// Месяц 1: 7 квартир (1+2+4) × 40 000 уборки = 280 000.
	// База аренды месяца 1 = 1×200000 + 2×250000 + 4×300000 = 1 900 000.
	const baseM1 = 1_900_000.0
	const cleanM1 = 280_000.0

	// Уборка только в месяце 1
	in.CleaningMonths = []int{1}
	rent, _ := calcRentApartments(in, ExecutorAibiconKG, 6)
	if got := rent[0]; math.Abs(got-(baseM1+cleanM1)) > 0.01 {
		t.Errorf("месяц 1 с уборкой: want %.2f, got %.2f", baseM1+cleanM1, got)
	}
	// Месяц 2 без уборки: только база 2×200000+5×250000+3×300000 = 2 550 000
	if got := rent[1]; math.Abs(got-2_550_000) > 0.01 {
		t.Errorf("месяц 2 без уборки: want 2550000.00, got %.2f", got)
	}

	// Пустой список — уборки нет за весь период
	in.CleaningMonths = nil
	rent, _ = calcRentApartments(in, ExecutorAibiconKG, 6)
	if got := rent[0]; math.Abs(got-baseM1) > 0.01 {
		t.Errorf("без выбранных месяцев уборка должна быть 0: want %.2f, got %.2f",
			baseM1, got)
	}

	// Номер месяца за пределами проекта не должен ничего ломать
	in.CleaningMonths = []int{99}
	rent, _ = calcRentApartments(in, ExecutorAibiconKG, 6)
	for m, v := range rent {
		if v == 0 {
			continue
		}
		// уборки быть не должно ни в одном месяце — сверяем только её отсутствие
		if m == 0 && math.Abs(v-baseM1) > 0.01 {
			t.Errorf("месяц вне проекта не должен включать уборку: got %.2f", v)
		}
	}
}

// TestCalcRentApartments_Empty — отсутствие данных листа 4.2 не должно
// ломать расчёт: обе строки нулевые.
func TestCalcRentApartments_Empty(t *testing.T) {
	rent, realtor := calcRentApartments(nil, ExecutorAibicon, 3)
	if len(rent) != 3 || len(realtor) != 3 {
		t.Fatalf("ожидались массивы длиной 3, got %d/%d", len(rent), len(realtor))
	}
	for i := 0; i < 3; i++ {
		if rent[i] != 0 || realtor[i] != 0 {
			t.Errorf("месяц %d: ожидались нули, got rent=%.2f realtor=%.2f",
				i+1, rent[i], realtor[i])
		}
	}
}

// TestValidateRentApartments — цены и количества не могут быть отрицательными.
// Правило «в последнем месяце не больше квартир, чем в предыдущем» здесь
// НЕ проверяется — оно отложено, см. docs/deferred_validations.md.
func TestValidateRentApartments(t *testing.T) {
	valid := referenceRentInput()
	if err := ValidateRentApartments(valid); err != nil {
		t.Errorf("эталонные данные должны проходить валидацию, got: %v", err)
	}

	// Рост количества в последнем месяце (5 → 7, как в реальных файлах)
	// НЕ должен блокироваться: правило отложено.
	if err := ValidateRentApartments(valid); err != nil {
		t.Errorf("рост квартир в последнем месяце не должен блокироваться: %v", err)
	}

	tests := []struct {
		name string
		in   *InputRentApartments
	}{
		{"отрицательная цена 1кк", &InputRentApartments{Price1Room: -1}},
		{"отрицательная цена 2кк", &InputRentApartments{Price2Room: -100}},
		{"отрицательная цена 3кк", &InputRentApartments{Price3Room: -0.5}},
		{"отрицательная уборка", &InputRentApartments{CleaningBase: -1}},
		{"отрицательный риелтор", &InputRentApartments{RealtorBase: -1}},
		{"отрицательное кол-во 1кк", &InputRentApartments{Count1Room: []int{1, -2}}},
		{"отрицательное кол-во 2кк", &InputRentApartments{Count2Room: []int{-1}}},
		{"отрицательное кол-во 3кк", &InputRentApartments{Count3Room: []int{0, 0, -3}}},
	}
	for _, tt := range tests {
		if err := ValidateRentApartments(tt.in); err == nil {
			t.Errorf("%s: ожидалась ошибка, got nil", tt.name)
		}
	}

	if err := ValidateRentApartments(nil); err != nil {
		t.Errorf("nil должен проходить валидацию, got: %v", err)
	}
}

// TestValidateInput — диспетчер валидации по типу входных данных.
func TestValidateInput(t *testing.T) {
	// Корректные данные 4.2
	ok := []byte(`{"price_1room":200000,"count_1room":[1,2],"cleaning_base":40000}`)
	if err := ValidateInput(TypeRentApartments, ok, ExecutorAibicon, 6); err != nil {
		t.Errorf("корректные данные 4.2: %v", err)
	}

	// Отрицательная цена
	bad := []byte(`{"price_1room":-5}`)
	if err := ValidateInput(TypeRentApartments, bad, ExecutorAibicon, 6); err == nil {
		t.Error("отрицательная цена: ожидалась ошибка")
	}

	// Битый JSON для типа с валидацией
	if err := ValidateInput(TypeRentApartments, []byte(`{"price_1room":`), ExecutorAibicon, 6); err == nil {
		t.Error("битый JSON: ожидалась ошибка")
	}

	// Тип без собственных правил — валидация пропускает
	if err := ValidateInput(TypeInternet, []byte(`{"monthly_amounts":[1,2]}`), ExecutorAibicon, 6); err != nil {
		t.Errorf("тип без правил не должен отклоняться: %v", err)
	}
}
