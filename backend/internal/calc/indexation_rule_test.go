package calc

import (
	"math"
	"testing"
)

// Правило индексации в формулировке владельца (2026-08-29): апрель
// повышает оклад тем, кто работал в компании ещё в марте того же года.
// Принятый в апреле этот апрель пропускает.
//
//	прошло пар «март + апрель» → множитель: 0 → 1, 1 → 1.1, 2 → 1.21

// empFrom — сотрудник, принятый в месяце hire и работающий до конца.
func empFrom(hire, duration int) *Employee {
	sched := make([]string, duration)
	for m := 0; m < duration; m++ {
		if m+1 < hire {
			sched[m] = ScheduleNotHired
		} else {
			sched[m] = ScheduleOF
		}
	}
	return &Employee{Country: CountryRF, SalaryNet: 100_000, MonthlySchedule: sched}
}

func TestIndexation_MarchAprilPairs(t *testing.T) {
	// Проект стартует в январе 2027 и идёт три года:
	// апрели — месяцы 4, 16, 28; марты — 3, 15, 27.
	start := mustDate(2027, 1, 1)
	const duration = 36

	tests := []struct {
		name  string
		hire  int // месяц проекта
		month int // на какой месяц смотрим
		want  float64
	}{
		// Принят в январе: март 2027 застал → апрель 2027 индексирует.
		{"принят в январе, до апреля", 1, 3, 1},
		{"принят в январе, апрель", 1, 4, 1.1},
		{"принят в январе, второй апрель", 1, 16, 1.1 * 1.1},
		{"принят в январе, третий апрель", 1, 28, 1.1 * 1.1 * 1.1},

		// Принят в марте: март застал → апрель того же года индексирует.
		{"принят в марте, апрель", 3, 4, 1.1},

		// Принят в апреле: марта в компании не было — этот апрель мимо.
		{"принят в апреле, тот же апрель", 4, 4, 1},
		{"принят в апреле, до следующего апреля", 4, 15, 1},
		{"принят в апреле, следующий апрель", 4, 16, 1.1},

		// Принят в мае: пара «март + апрель» будет только в следующем году.
		{"принят в мае, до апреля", 5, 15, 1},
		{"принят в мае, апрель", 5, 16, 1.1},
		{"принят в мае, второй апрель", 5, 28, 1.1 * 1.1},

		// Принят в феврале — как и все, кто «с мая по февраль».
		{"принят в феврале, апрель", 2, 4, 1.1},
	}

	for _, tt := range tests {
		emp := empFrom(tt.hire, duration)
		got := aprilIndexation(emp, tt.month, start)
		if math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("%s: want %.4f, got %.4f", tt.name, tt.want, got)
		}
	}
}

// Индексация — свойство сотрудника, а не проекта: два человека с разными
// месяцами приёма в одном и том же месяце могут иметь разные коэффициенты.
func TestIndexation_IsPerEmployee(t *testing.T) {
	start := mustDate(2027, 1, 1)
	const duration = 18

	early := empFrom(1, duration) // застал март 2027
	late := empFrom(4, duration)  // принят в апреле 2027

	if got := aprilIndexation(early, 4, start); math.Abs(got-1.1) > 1e-9 {
		t.Errorf("принятый в январе, апрель: want 1.1, got %.4f", got)
	}
	if got := aprilIndexation(late, 4, start); got != 1 {
		t.Errorf("принятый в апреле, тот же апрель: want 1, got %.4f", got)
	}
}

// «МВ»: выплата 30 000 не индексируется, а оклад — индексируется.
// Множитель равен 30000/оклад, поэтому оклад в произведении сокращается.
func TestIndexation_InterShiftPayNotIndexed(t *testing.T) {
	start := mustDate(2027, 1, 1)
	const duration = 18
	const salary = 200_000

	emp := empFrom(1, duration)
	emp.SalaryNet = salary
	for i := range emp.MonthlySchedule {
		emp.MonthlySchedule[i] = ScheduleMV
	}

	// До апреля и после него выплата одна и та же — 30 000.
	for _, m := range []int{1, 3, 4, 5, 16} {
		if got := employeeFOT(emp, m, start); math.Abs(got-interShiftPay) > 0.01 {
			t.Errorf("МВ, месяц %d: want %.0f, got %.2f", m, interShiftPay, got)
		}
	}

	// А сам оклад при этом проиндексирован: в апреле множитель меньше,
	// потому что делится на выросший оклад.
	base := scheduleMultiplier(ScheduleMV, salary)
	indexed := scheduleMultiplier(ScheduleMV, salary*indexationRate)
	if !(indexed < base) {
		t.Errorf("множитель МВ должен уменьшиться после индексации: было %.6f, стало %.6f", base, indexed)
	}

	// Обычный график индексируется в полный рост.
	emp2 := empFrom(1, duration)
	emp2.SalaryNet = salary
	if got := employeeFOT(emp2, 4, start); math.Abs(got-salary*indexationRate) > 0.01 {
		t.Errorf("ОФ в апреле: want %.2f, got %.2f", salary*indexationRate, got)
	}
}

// Апрели считаются по календарю, а не по номеру месяца проекта.
func TestIndexation_CalendarAprilNotProjectMonth(t *testing.T) {
	// Старт в декабре 2026 → апрель это 5-й месяц проекта.
	start := mustDate(2026, 12, 1)
	emp := empFrom(1, 12)
	if got := aprilIndexation(emp, 4, start); got != 1 {
		t.Errorf("месяц 4 (март): want 1, got %.4f", got)
	}
	if got := aprilIndexation(emp, 5, start); math.Abs(got-1.1) > 1e-9 {
		t.Errorf("месяц 5 (апрель): want 1.1, got %.4f", got)
	}
}
