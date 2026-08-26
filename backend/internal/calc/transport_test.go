package calc

import (
	"math"
	"strings"
	"testing"
)

// Эталонные данные листа 4.3 из calc_sheets_ibcon-project-russia.xlsm
// (единственный из трёх файлов, где заполнена и аренда авто, и покупка,
// и гараж). Длительность проекта 2.Бюджет!D8 = 6 месяцев.
//
// Что стоит в форме:
//
//	аренда авто   4.3!B6  = 70 000, кол-во 4.3!C5:BJ5  = 1, 2, 3, 3, 3, 6
//	покупка авто  4.3!A17:C18 = мес.2 × 1 шт × 2 500 000
//	                            мес.5 × 2 шт × 1 950 000
//	аренда гаража 4.3!B11 = 50 000, кол-во 4.3!C10:BJ10 = 1, 1, 1, 2, 3, 3
//
// Ввод платформы ложится на форму один в один: таблица покупок — строка в
// строку, аренда — цена за единицу плюс количество в каждом месяце.
func referenceTransportInput() *InputTransport {
	return &InputTransport{
		// 4.3!A17:C18
		CarPurchases: []ItemPurchase{
			{Name: "Авто 1", Month: 2, Count: 1, Price: 2_500_000},
			{Name: "Авто 2", Month: 5, Count: 2, Price: 1_950_000},
		},
		// 4.3!B6 и 4.3!C5:BJ5
		CarRentals: []RentedItem{
			{Name: "Аренда авто", Price: 70_000, Counts: []int{1, 2, 3, 3, 3, 6}},
		},
		// 4.3!B11 и 4.3!C10:BJ10
		GarageRentals: []RentedItem{
			{Name: "Гараж", Price: 50_000, Counts: []int{1, 1, 1, 2, 3, 3}},
		},
	}
}

// TestCalcTransport_Reference — численная сверка с эталоном.
//
// Ожидаемые значения взяты из calc_sheets_ibcon-project-russia.xlsm:
// строка 4.3!C6:BJ6 (она же 2.Бюджет!H180:BO180) и 4.3!C11:BJ11
// (она же 2.Бюджет!H210:BO210). Месяц 6 в форме лежит в колонке BJ.
func TestCalcTransport_Reference(t *testing.T) {
	transport, garage := calcTransport(referenceTransportInput(), 6)

	// 4.3!C6, D6, E6, F6, G6, BJ6
	wantTransport := []float64{
		70_000,    // 1×70 000
		2_640_000, // 2×70 000 + 2 500 000
		210_000,   // 3×70 000
		210_000,   // 3×70 000
		4_110_000, // 3×70 000 + 2×1 950 000
		420_000,   // 6×70 000
	}
	// 4.3!C11, D11, E11, F11, G11, BJ11
	wantGarage := []float64{50_000, 50_000, 50_000, 100_000, 150_000, 150_000}

	for i := range wantTransport {
		if math.Abs(transport[i]-wantTransport[i]) > 0.01 {
			t.Errorf("транспорт (180), месяц %d: want %.2f, got %.2f",
				i+1, wantTransport[i], transport[i])
		}
		if math.Abs(garage[i]-wantGarage[i]) > 0.01 {
			t.Errorf("гараж (210), месяц %d: want %.2f, got %.2f",
				i+1, wantGarage[i], garage[i])
		}
	}

	// Итоги за проект: 4.3!BK6 = 2.Бюджет!G180, 4.3!BK11 = 2.Бюджет!G210
	if got := sumFloats(transport); math.Abs(got-7_660_000) > 0.01 {
		t.Errorf("итого транспорт (4.3!BK6): want 7 660 000, got %.2f", got)
	}
	if got := sumFloats(garage); math.Abs(got-550_000) > 0.01 {
		t.Errorf("итого гараж (4.3!BK11): want 550 000, got %.2f", got)
	}
}

// TestCalcTransport_InBudget — те же эталонные суммы, но через полный
// расчёт: проверяем, что 4.3 попадает именно в строки 180 и 210
// (индексы 2 и 32 массива Overhead) и ничего по дороге не теряется.
func TestCalcTransport_InBudget(t *testing.T) {
	res := Run(&BudgetInputs{
		DurationMonths: 6,
		ExecutorName:   ExecutorAibiconProject,
		Transport:      referenceTransportInput(),
	})

	wantTransport := []float64{70_000, 2_640_000, 210_000, 210_000, 4_110_000, 420_000}
	wantGarage := []float64{50_000, 50_000, 50_000, 100_000, 150_000, 150_000}

	for m := 0; m < 6; m++ {
		if got := res.Monthly[m].Overhead[2]; math.Abs(got-wantTransport[m]) > 0.01 {
			t.Errorf("2.Бюджет!180, месяц %d: want %.2f, got %.2f", m+1, wantTransport[m], got)
		}
		if got := res.Monthly[m].Overhead[32]; math.Abs(got-wantGarage[m]) > 0.01 {
			t.Errorf("2.Бюджет!210, месяц %d: want %.2f, got %.2f", m+1, wantGarage[m], got)
		}
	}
}

// TestCalcTransport_LegacyFallback — версии, сохранённые до перехода 4.3 на
// расчёт по формуле, продолжают считаться по старым готовым суммам.
func TestCalcTransport_LegacyFallback(t *testing.T) {
	res := Run(&BudgetInputs{
		DurationMonths:  3,
		ExecutorName:    ExecutorAibicon,
		TransportRental: []float64{100, 200, 300},
		GarageRent:      []float64{10, 20, 30},
	})
	for m, want := range []float64{100, 200, 300} {
		if got := res.Monthly[m].Overhead[2]; got != want {
			t.Errorf("старый ввод 180, месяц %d: want %.0f, got %.0f", m+1, want, got)
		}
	}
	for m, want := range []float64{10, 20, 30} {
		if got := res.Monthly[m].Overhead[32]; got != want {
			t.Errorf("старый ввод 210, месяц %d: want %.0f, got %.0f", m+1, want, got)
		}
	}

	// Как только у версии появился новый ввод, старые суммы игнорируются.
	res = Run(&BudgetInputs{
		DurationMonths:  3,
		ExecutorName:    ExecutorAibicon,
		TransportRental: []float64{100, 200, 300},
		GarageRent:      []float64{10, 20, 30},
		Transport: &InputTransport{
			CarPurchases: []ItemPurchase{{Month: 1, Count: 1, Price: 999}},
		},
	})
	if got := res.Monthly[0].Overhead[2]; got != 999 {
		t.Errorf("новый ввод должен перекрывать старый: want 999, got %.0f", got)
	}
	if got := res.Monthly[0].Overhead[32]; got != 0 {
		t.Errorf("старый гараж не должен просачиваться: want 0, got %.0f", got)
	}
}

// TestCalcTransport_SplitFormulaFixed — исправление расщеплённой формулы.
//
// В форме проверка «месяц покупки внутри проекта» стоит только у строк
// 17–24 (`D17 = IF(A17<=$D$8, C17*B17, 0)`), а у строк 25–27 её нет
// (`D25 = C25*B25`), из-за чего результат зависел от того, в какую строку
// таблицы попал ввод. Здесь правило одно для всех строк: покупка вне
// проекта не считается, каким бы номером строка ни была.
func TestCalcTransport_SplitFormulaFixed(t *testing.T) {
	in := &InputTransport{CarPurchases: []ItemPurchase{
		{Name: "внутри проекта", Month: 3, Count: 1, Price: 1_000_000},
		{Name: "за пределами", Month: 7, Count: 1, Price: 5_000_000},
		{Name: "месяц не заполнен", Month: 0, Count: 1, Price: 3_000_000},
	}}
	transport, _ := calcTransport(in, 6)

	if got := sumFloats(transport); math.Abs(got-1_000_000) > 0.01 {
		t.Errorf("в итог должна попасть только покупка 3-го месяца: want 1 000 000, got %.2f", got)
	}
	if transport[2] != 1_000_000 {
		t.Errorf("месяц 3: want 1 000 000, got %.2f", transport[2])
	}
}

// TestCalcTransport_RentalCountsLength — массив количеств может не совпасть
// по длине с проектом: короткий добивается нулями, длинный обрезается.
// Так бывает после смены длительности проекта у уже сохранённой версии.
func TestCalcTransport_RentalCountsLength(t *testing.T) {
	transport, _ := calcTransport(&InputTransport{
		CarRentals: []RentedItem{
			{Price: 1000, Counts: []int{1}},             // короче проекта
			{Price: 1000, Counts: []int{0, 1, 0, 5, 9}}, // длиннее проекта
		},
	}, 3)

	want := []float64{1000, 1000, 0}
	for i, w := range want {
		if transport[i] != w {
			t.Errorf("месяц %d: want %.0f, got %.0f", i+1, w, transport[i])
		}
	}
}

// TestCalcTransport_Count — количество умножает и аренду, и покупку;
// ноль в месяце — допустимое значение, ничего не начисляется.
func TestCalcTransport_Count(t *testing.T) {
	transport, garage := calcTransport(&InputTransport{
		CarPurchases: []ItemPurchase{{Month: 1, Count: 3, Price: 100}},
		CarRentals: []RentedItem{
			{Price: 10, Counts: []int{0, 4, 4}},
			{Price: 10, Counts: []int{0, 0, 0}}, // не арендуем ни в одном месяце
		},
		GarageRentals: []RentedItem{{Price: 50, Counts: []int{0, 0, 2}}},
	}, 3)

	want := []float64{300, 40, 40}
	for i, w := range want {
		if transport[i] != w {
			t.Errorf("транспорт, месяц %d: want %.0f, got %.0f", i+1, w, transport[i])
		}
	}
	if garage[2] != 100 {
		t.Errorf("гараж, месяц 3: want 100, got %.0f", garage[2])
	}
}

// TestCalcTransport_Empty — пустой ввод и нулевая длительность не роняют расчёт.
func TestCalcTransport_Empty(t *testing.T) {
	transport, garage := calcTransport(nil, 4)
	if len(transport) != 4 || len(garage) != 4 {
		t.Fatalf("длина массивов: want 4/4, got %d/%d", len(transport), len(garage))
	}
	if sumFloats(transport) != 0 || sumFloats(garage) != 0 {
		t.Error("пустой ввод должен давать нули")
	}
	if transport, garage = calcTransport(referenceTransportInput(), 0); len(transport) != 0 || len(garage) != 0 {
		t.Error("нулевая длительность должна давать пустые массивы")
	}
}

// TestValidateTransport — месяц вне проекта отклоняем, пустой месяц пропускаем.
func TestValidateTransport(t *testing.T) {
	if err := ValidateTransport(referenceTransportInput(), 6); err != nil {
		t.Errorf("эталонный ввод должен проходить валидацию: %v", err)
	}

	cases := []struct {
		name string
		in   *InputTransport
		want string // подстрока ожидаемой ошибки, "" — ошибки быть не должно
	}{
		{
			name: "покупка без месяца — строка просто игнорируется",
			in:   &InputTransport{CarPurchases: []ItemPurchase{{Price: 100}}},
		},
		{
			name: "покупка за пределами проекта",
			in:   &InputTransport{CarPurchases: []ItemPurchase{{Name: "Газель", Month: 7, Price: 100}}},
			want: "месяц покупки 7 вне проекта",
		},
		{
			name: "отрицательный месяц покупки",
			in:   &InputTransport{CarPurchases: []ItemPurchase{{Month: -1, Price: 100}}},
			want: "вне проекта",
		},
		{
			name: "отрицательная цена покупки",
			in:   &InputTransport{CarPurchases: []ItemPurchase{{Month: 1, Price: -5}}},
			want: "цена не может быть отрицательной",
		},
		{
			name: "отрицательная цена аренды авто",
			in:   &InputTransport{CarRentals: []RentedItem{{Price: -1, Counts: []int{1}}}},
			want: "аренда авто",
		},
		{
			name: "нулевые количества допустимы",
			in:   &InputTransport{CarRentals: []RentedItem{{Price: 1, Counts: []int{0, 0}}}},
		},
		{
			name: "отрицательное количество в покупке",
			in:   &InputTransport{CarPurchases: []ItemPurchase{{Month: 1, Count: -2, Price: 100}}},
			want: "количество не может быть отрицательным",
		},
		{
			name: "отрицательное количество в аренде авто",
			in:   &InputTransport{CarRentals: []RentedItem{{Price: 1, Counts: []int{1, -1}}}},
			want: "количество в месяце 2 не может быть отрицательным",
		},
		{
			name: "отрицательное количество в аренде гаража",
			in:   &InputTransport{GarageRentals: []RentedItem{{Price: 1, Counts: []int{-3}}}},
			want: "аренда гаража",
		},
	}

	for _, c := range cases {
		err := ValidateTransport(c.in, 6)
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

// TestValidateInput_Transport — тот же контроль через диспетчер сохранения.
func TestValidateInput_Transport(t *testing.T) {
	ok := []byte(`{"car_purchases":[{"name":"Газель","month":2,"count":1,"price":100}]}`)
	if err := ValidateInput(TypeTransport, ok, ExecutorAibicon, 6); err != nil {
		t.Errorf("корректный ввод отклонён: %v", err)
	}

	bad := []byte(`{"car_purchases":[{"name":"Газель","month":8,"count":1,"price":100}]}`)
	if err := ValidateInput(TypeTransport, bad, ExecutorAibicon, 6); err == nil {
		t.Error("месяц 8 при длительности 6 должен отклоняться")
	}

	if err := ValidateInput(TypeTransport, []byte(`{"car_purchases":`), ExecutorAibicon, 6); err == nil {
		t.Error("битый JSON должен отклоняться")
	}
}

func sumFloats(a []float64) float64 {
	var s float64
	for _, v := range a {
		s += v
	}
	return s
}
