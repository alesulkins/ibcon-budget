package calc

import (
	"encoding/json"
	"math"
	"testing"
)

// Множитель графика (4.6!BU16) обычно считается формулой, но экономист
// может задать его руками в любом месяце. Ручное значение перекрывает
// формулу и идёт в расчёт ФОТ как есть.

func mkEmp(schedule []string, salary float64) *Employee {
	return &Employee{
		Country:         CountryRF,
		SalaryNet:       salary,
		MonthlySchedule: schedule,
	}
}

// Без ручных значений множитель считается по формуле формы.
func TestEffectiveMultiplier_Auto(t *testing.T) {
	const salary = 200_000
	emp := mkEmp([]string{ScheduleOF, ScheduleMV, ScheduleNotHired, "4/2"}, salary)

	tests := []struct {
		month int
		want  float64
	}{
		{1, 1},                      // ОФ — полный оклад
		{2, interShiftPay / salary}, // МВ — 30 000 / оклад
		{3, 0},                      // не принят
		{4, 1},                      // 4/2 — полный оклад
		{5, 0},                      // за пределами графика
	}
	for _, tt := range tests {
		if got := EffectiveMultiplier(emp, tt.month, salary); math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("месяц %d: want %.6f, got %.6f", tt.month, tt.want, got)
		}
	}
}

// Ручное значение перекрывает формулу, соседние месяцы остаются авто.
func TestEffectiveMultiplier_ManualOverridesAuto(t *testing.T) {
	const salary = 200_000
	emp := mkEmp([]string{ScheduleOF, ScheduleOF, ScheduleNotHired}, salary)
	// Месяц 2 — половина; месяц 3 — ноль, заданный руками поверх «не принят».
	emp.MultiplierOverrides = []*float64{nil, pct(0.5), pct(0)}

	if got := EffectiveMultiplier(emp, 1, salary); got != 1 {
		t.Errorf("месяц 1 остаётся авто: want 1, got %.6f", got)
	}
	if got := EffectiveMultiplier(emp, 2, salary); got != 0.5 {
		t.Errorf("месяц 2 задан руками: want 0.5, got %.6f", got)
	}
	// Ноль — законное ручное значение, а не «ячейку не трогали».
	if got := EffectiveMultiplier(emp, 3, salary); got != 0 {
		t.Errorf("месяц 3 задан руками нулём: want 0, got %.6f", got)
	}

	// Ручной ноль поверх рабочего графика тоже должен действовать.
	emp2 := mkEmp([]string{ScheduleOF}, salary)
	emp2.MultiplierOverrides = []*float64{pct(0)}
	if got := EffectiveMultiplier(emp2, 1, salary); got != 0 {
		t.Errorf("ручной ноль поверх ОФ: want 0, got %.6f", got)
	}
}

// Массив короче графика или отсутствует — недостающие месяцы авто.
func TestEffectiveMultiplier_ShortOverrides(t *testing.T) {
	const salary = 100_000
	emp := mkEmp([]string{ScheduleOF, ScheduleOF, ScheduleOF}, salary)
	emp.MultiplierOverrides = []*float64{pct(0.25)}

	if got := EffectiveMultiplier(emp, 1, salary); got != 0.25 {
		t.Errorf("месяц 1: want 0.25, got %.6f", got)
	}
	for _, m := range []int{2, 3} {
		if got := EffectiveMultiplier(emp, m, salary); got != 1 {
			t.Errorf("месяц %d должен считаться авто: want 1, got %.6f", m, got)
		}
	}
}

// Ручное значение доходит до самого ФОТ, а не только до множителя.
func TestEmployeeFOT_UsesManualMultiplier(t *testing.T) {
	const salary = 200_000
	start := mustDate(2026, 12, 1)

	emp := mkEmp([]string{ScheduleOF, ScheduleOF}, salary)
	if got := employeeFOT(emp, 2, start, false); got != salary {
		t.Errorf("без ручного значения: want %.0f, got %.2f", float64(salary), got)
	}

	emp.MultiplierOverrides = []*float64{nil, pct(0.5)}
	if got := employeeFOT(emp, 2, start, false); got != salary*0.5 {
		t.Errorf("с ручным значением: want %.0f, got %.2f", salary*0.5, got)
	}
	// Первый месяц не тронут.
	if got := employeeFOT(emp, 1, start, false); got != salary {
		t.Errorf("месяц 1 остаётся авто: want %.0f, got %.2f", float64(salary), got)
	}
}

// Автоматический множитель «МВ» зависит от проиндексированного оклада, и
// выплата остаётся ровно 30 000. Ручной множитель от индексации не зависит:
// экономист задал долю, и она применяется к окладу текущего месяца.
func TestMultiplier_WithIndexation(t *testing.T) {
	const salary = 200_000
	// Старт декабрь 2026 → апрель это 5-й месяц проекта.
	start := mustDate(2026, 12, 1)
	sched := []string{ScheduleMV, ScheduleMV, ScheduleMV, ScheduleMV, ScheduleMV}

	auto := mkEmp(sched, salary)
	if got := employeeFOT(auto, 5, start, false); math.Abs(got-interShiftPay) > 0.01 {
		t.Errorf("МВ после индексации остаётся 30 000: got %.2f", got)
	}

	manual := mkEmp(sched, salary)
	manual.MultiplierOverrides = []*float64{nil, nil, nil, nil, pct(0.5)}
	want := salary * indexationRate * 0.5
	if got := employeeFOT(manual, 5, start, false); math.Abs(got-want) > 0.01 {
		t.Errorf("ручной множитель к проиндексированному окладу: want %.2f, got %.2f", want, got)
	}
}

// В JSON ячейка без ручного значения — null, и это отличается от нуля.
func TestMultiplierOverrides_JSONRoundTrip(t *testing.T) {
	raw := []byte(`{"salary_net":100000,"monthly_schedule":["ОФ","ОФ","ОФ"],` +
		`"multiplier_overrides":[null,0,0.75]}`)

	var emp Employee
	if err := json.Unmarshal(raw, &emp); err != nil {
		t.Fatalf("разбор JSON: %v", err)
	}
	if emp.MultiplierOverrides[0] != nil {
		t.Error("null должен читаться как «считается автоматически»")
	}
	if v := emp.MultiplierOverrides[1]; v == nil || *v != 0 {
		t.Error("0 должен читаться как ручное значение «ноль», а не как null")
	}
	if v := emp.MultiplierOverrides[2]; v == nil || *v != 0.75 {
		t.Error("0.75 не прочитан")
	}

	// Обратно — тоже с null в нетронутой ячейке.
	out, err := json.Marshal(emp.MultiplierOverrides)
	if err != nil {
		t.Fatalf("сериализация: %v", err)
	}
	if string(out) != `[null,0,0.75]` {
		t.Errorf("сериализация: got %s", out)
	}
}
