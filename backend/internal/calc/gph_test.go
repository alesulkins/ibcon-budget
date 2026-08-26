package calc

import "testing"

func TestCalcGphEmployeesSameEveryMonth(t *testing.T) {
	in := &InputGphEmployees{AvgCount: 3, AvgCost: 80000}
	got := calcGphEmployees(in, 6)
	if len(got) != 6 {
		t.Fatalf("длина массива %d, ожидалось 6", len(got))
	}
	for m, v := range got {
		if v != 240000 {
			t.Errorf("месяц %d: получено %.2f, ожидалось 240000", m+1, v)
		}
	}
}

// Среднее количество — именно среднее, поэтому дробное значение допустимо
// и не должно округляться до целого.
func TestCalcGphEmployeesFractionalCount(t *testing.T) {
	in := &InputGphEmployees{AvgCount: 2.5, AvgCost: 100000}
	got := calcGphEmployees(in, 2)
	for m, v := range got {
		if v != 250000 {
			t.Errorf("месяц %d: получено %.2f, ожидалось 250000", m+1, v)
		}
	}
}

// Одно из полей не заполнено — расход нулевой, а не «ошибка ввода».
func TestCalcGphEmployeesZeroField(t *testing.T) {
	for _, in := range []*InputGphEmployees{
		{AvgCount: 0, AvgCost: 100000},
		{AvgCount: 3, AvgCost: 0},
	} {
		for m, v := range calcGphEmployees(in, 3) {
			if v != 0 {
				t.Errorf("%+v, месяц %d: получено %.2f, ожидался 0", in, m+1, v)
			}
		}
	}
}

func TestCalcGphEmployeesNil(t *testing.T) {
	got := calcGphEmployees(nil, 4)
	if len(got) != 4 {
		t.Fatalf("длина массива %d, ожидалось 4", len(got))
	}
	for m, v := range got {
		if v != 0 {
			t.Errorf("месяц %d: получено %.2f, ожидался 0", m+1, v)
		}
	}
}

func TestGphEmployeesReachesOverheadRow201(t *testing.T) {
	inp := purchasesInputs(6)
	inp.GphEmployees = &InputGphEmployees{AvgCount: 4, AvgCost: 65000}

	res := Run(inp)
	for m := range res.Monthly {
		if got := res.Monthly[m].Overhead[23]; got != 260000 {
			t.Errorf("месяц %d, строка 201: получено %.2f, ожидалось 260000", m+1, got)
		}
		// 200 (4.9) и 202 (4.11) — соседние строки субподряда, их лист 4.10
		// задевать не должен.
		if got := res.Monthly[m].Overhead[22]; got != 0 {
			t.Errorf("месяц %d, строка 200 не должна меняться, получено %.2f", m+1, got)
		}
		if got := res.Monthly[m].Overhead[24]; got != 0 {
			t.Errorf("месяц %d, строка 202 не должна меняться, получено %.2f", m+1, got)
		}
	}
}

func TestGphEmployeesLegacyFallback(t *testing.T) {
	inp := purchasesInputs(3)
	inp.SubcontractEmp = []float64{100, 200, 300}

	res := Run(inp)
	for m, w := range []float64{100, 200, 300} {
		if got := res.Monthly[m].Overhead[23]; got != w {
			t.Errorf("месяц %d, строка 201: получено %.2f, ожидалось %.2f", m+1, got, w)
		}
	}
}

func TestValidateGphEmployeesRejectsNegative(t *testing.T) {
	if err := ValidateGphEmployees(&InputGphEmployees{AvgCount: -1, AvgCost: 100}); err == nil {
		t.Error("отрицательное количество должно отклоняться")
	}
	if err := ValidateGphEmployees(&InputGphEmployees{AvgCount: 1, AvgCost: -100}); err == nil {
		t.Error("отрицательная стоимость должна отклоняться")
	}
	if err := ValidateGphEmployees(&InputGphEmployees{}); err != nil {
		t.Errorf("нулевой ввод отклонён: %v", err)
	}
}
