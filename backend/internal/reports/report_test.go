package reports

import (
	"math"
	"testing"

	"ibcon-budget/internal/calc"
)

// resWith — результат расчёта на n месяцев, где каждый месяц заполнен
// одним и тем же набором значений.
func resWith(n int, fill func(m *calc.MonthlyResult)) *calc.CalcResult {
	res := &calc.CalcResult{DurationMonths: n, Monthly: make([]calc.MonthlyResult, n)}
	for i := range res.Monthly {
		res.Monthly[i].Month = i + 1
		fill(&res.Monthly[i])
	}
	return res
}

// Горизонт отчёта — длительность проекта, а не жёсткие 12 месяцев формы.
func TestBuild_HorizonFollowsProject(t *testing.T) {
	for _, n := range []int{3, 12, 18} {
		rep := Build(KindBDR, resWith(n, func(m *calc.MonthlyResult) {}))
		if rep.Months != n {
			t.Errorf("проект на %d мес.: Months = %d", n, rep.Months)
		}
		for _, r := range rep.Rows {
			if len(r.Monthly) != n {
				t.Fatalf("строка %s: %d месяцев вместо %d", r.Code, len(r.Monthly), n)
			}
		}
	}
}

func row(t *testing.T, rep *Report, code string) Row {
	t.Helper()
	for _, r := range rep.Rows {
		if r.Code == code {
			return r
		}
	}
	t.Fatalf("строка %s не найдена", code)
	return Row{}
}

// Групповые строки собирают сумму вложенных, а не имеют своего источника.
func TestBuild_RollUp(t *testing.T) {
	rep := Build(KindBDR, resWith(2, func(m *calc.MonthlyResult) {
		m.Overhead[182-178] = 100 // аренда офиса   → 2.2.1.01
		m.Overhead[196-178] = 20  // содержание     → 2.2.1.02
		m.Overhead[208-178] = 3   // коммунальные   → 2.2.1.04
		m.Overhead[183-178] = 7   // уборка офиса   → 2.2.1.06
	}))

	g := row(t, rep, "2.2.1")
	if !g.Group {
		t.Error("2.2.1 должна быть групповой строкой")
	}
	if g.Monthly[0] != 130 {
		t.Errorf("2.2.1, месяц 1: want 130, got %.2f", g.Monthly[0])
	}
	if g.Total != 260 {
		t.Errorf("2.2.1, итого за 2 месяца: want 260, got %.2f", g.Total)
	}

	// Внуки не должны попасть в «2» дважды — через себя и через родителя.
	// Кроме 2.2.1 в этом наборе ничего не заполнено, поэтому «2» = 260.
	if s := row(t, rep, "2").Total; s != 260 {
		t.Errorf("2 «Себестоимость»: want 260, got %.2f", s)
	}
	if s := row(t, rep, "2.2").Total; s != 260 {
		t.Errorf("2.2 «Накладные»: want 260, got %.2f", s)
	}
}

// Взносы Киргизии не теряются: в форме строка 174 не учтена нигде,
// здесь она входит в статью взносов тем же месяцем, что и в 2.Бюджет.
func TestBDR_KGInsuranceNotLost(t *testing.T) {
	rep := Build(KindBDR, resWith(3, func(m *calc.MonthlyResult) {
		m.InsuranceRF = 1_000
		m.InsuranceKG = 500
	}))
	r := row(t, rep, "2.1.3")
	for m := 0; m < 3; m++ {
		if r.Monthly[m] != 1_500 {
			t.Errorf("взносы, месяц %d: want 1 500, got %.2f", m+1, r.Monthly[m])
		}
	}
}

// Охрана объекта (строка 209) в форме потеряна — здесь она на месте, и
// мебель (195) при этом учтена ровно один раз.
func TestBDR_SecurityAndFurnitureFixed(t *testing.T) {
	rep := Build(KindBDR, resWith(1, func(m *calc.MonthlyResult) {
		m.Overhead[195-178] = 274_500 // мебель
		m.Overhead[209-178] = 108_000 // охрана объекта
	}))

	if v := row(t, rep, "2.2.4.07").Total; v != 274_500 {
		t.Errorf("мебель: want 274 500, got %.2f", v)
	}
	if v := row(t, rep, "2.2.7.09").Total; v != 108_000 {
		t.Errorf("охрана объекта: want 108 000, got %.2f", v)
	}
	// Обе статьи должны войти в себестоимость ровно по одному разу.
	if v := row(t, rep, "2").Total; v != 274_500+108_000 {
		t.Errorf("себестоимость: want 382 500, got %.2f", v)
	}
}

// Субподряд — сумма четырёх строк 2.Бюджет (200-203) в одной статье.
func TestBDR_SubcontractSum(t *testing.T) {
	rep := Build(KindBDR, resWith(1, func(m *calc.MonthlyResult) {
		m.Overhead[200-178] = 1
		m.Overhead[201-178] = 2
		m.Overhead[202-178] = 4
		m.Overhead[203-178] = 8
	}))
	if v := row(t, rep, "2.1.1.05").Total; v != 15 {
		t.Errorf("субподрядные работы: want 15, got %.2f", v)
	}
}

// ФОТ оплаты труда — оклад плюс переработки РФ плюс НДФЛ (168+170+172).
func TestBDR_PayrollComposition(t *testing.T) {
	rep := Build(KindBDR, resWith(1, func(m *calc.MonthlyResult) {
		m.FOT = 100
		m.OvertimeRF = 10
		m.OvertimeKG = 5 // в эту статью не входит
		m.NDFL = 20
	}))
	if v := row(t, rep, "2.1.2").Total; v != 130 {
		t.Errorf("ФОТ оплаты труда: want 130, got %.2f", v)
	}
}

// Выручка попадает в статью 1.1 и поднимается в группу «1».
func TestBDR_Revenue(t *testing.T) {
	rep := Build(KindBDR, resWith(2, func(m *calc.MonthlyResult) { m.Revenue = 1_000 }))
	if v := row(t, rep, "1.1").Total; v != 2_000 {
		t.Errorf("выручка: want 2 000, got %.2f", v)
	}
	if v := row(t, rep, "1").Total; v != 2_000 {
		t.Errorf("группа «Выручка»: want 2 000, got %.2f", v)
	}
}

// Каждая заполняемая строка 2.Бюджет должна попасть ровно в одну статью
// БДР — иначе расход потеряется или задвоится. Проверяем сплошняком:
// подставляем в одну строку 178-211 уникальное значение и смотрим итог.
func TestBDR_EveryOverheadRowMappedOnce(t *testing.T) {
	// 203 отдельной статьи не имеет — входит в субподряд вместе с 200-202.
	for row180 := 178; row180 <= 211; row180++ {
		idx := row180 - 178
		rep := Build(KindBDR, resWith(1, func(m *calc.MonthlyResult) {
			m.Overhead[idx] = 1_000
		}))
		if got := rowByCode(rep, "2").Total; math.Abs(got-1_000) > 0.01 {
			t.Errorf("строка 2.Бюджет!%d: в себестоимость попало %.2f вместо 1 000",
				row180, got)
		}
	}
}

func rowByCode(rep *Report, code string) Row {
	for _, r := range rep.Rows {
		if r.Code == code {
			return r
		}
	}
	return Row{}
}
