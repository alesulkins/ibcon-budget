package calc

import (
	"math"
	"testing"
	"time"
)

// mustDate — хелпер для создания дат в тестах
func mustDate(y, m, d int) time.Time {
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
}

func TestScheduleMultiplier(t *testing.T) {
	tests := []struct {
		sched  string
		salary float64
		want   float64
	}{
		{ScheduleMV, 100_000, 0.3},  // 30000/100000
		{ScheduleMV, 60_000, 0.5},   // 30000/60000
		{ScheduleNotHired, 100_000, 0},
		{Schedule42, 100_000, 1},
		{ScheduleOF, 100_000, 1},
		{Schedule44, 100_000, 1},
		{ScheduleOTP, 100_000, 1},
		{ScheduleK, 100_000, 1},
	}
	for _, tt := range tests {
		got := scheduleMultiplier(tt.sched, tt.salary)
		if math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("scheduleMultiplier(%q, %.0f) = %.6f, want %.6f", tt.sched, tt.salary, got, tt.want)
		}
	}
}

func TestEmployeeFOT_BasicSchedules(t *testing.T) {
	start := mustDate(2024, 1, 1)
	emp := &Employee{
		Country:         CountryRF,
		BaseSchedule:    ScheduleOF,
		SalaryNet:       100_000,
		MonthlySchedule: []string{ScheduleOF, ScheduleOF, ScheduleMV, ScheduleNotHired, Schedule42},
	}

	tests := []struct {
		month int
		want  float64
	}{
		{1, 100_000}, // ОФ — полный оклад
		{2, 100_000}, // ОФ
		{3, 30_000},  // МВ — фиксированные 30 000 (формула Excel: IF(МВ, 30000/salary, ...)*salary)
		{4, 0},       // не принят
		{5, 100_000}, // 4/2 — полный оклад (индексации нет, формула Excel не индексирует)
	}
	for _, tt := range tests {
		got := employeeFOT(emp, tt.month, start)
		if math.Abs(got-tt.want) > 0.01 {
			t.Errorf("employeeFOT month=%d: got %.2f, want %.2f", tt.month, got, tt.want)
		}
	}
}

func TestCalcFOT_TaxRF(t *testing.T) {
	emp := Employee{
		Country:         CountryRF,
		BaseSchedule:    ScheduleOF,
		SalaryNet:       100_000,
		MonthlySchedule: []string{ScheduleOF},
	}
	fot, ndfl, insRF, insKG := calcFOTMonthly(
		[]Employee{emp}, nil, nil, nil, mustDate(2024, 1, 1), 1,
	)

	if math.Abs(fot[0]-100_000) > 0.01 {
		t.Errorf("fot: want 100000, got %.2f", fot[0])
	}

	// НДФЛ = 100 000 / 0.85 - 100 000 ≈ 17 647.06
	wantNDFL := 100_000/0.85 - 100_000
	if math.Abs(ndfl[0]-wantNDFL) > 0.01 {
		t.Errorf("ndfl: want %.2f, got %.2f", wantNDFL, ndfl[0])
	}

	// Взносы РФ = 100 000 / 0.85 × 0.302 ≈ 35 529.41
	wantInsRF := 100_000 / 0.85 * 0.302
	if math.Abs(insRF[0]-wantInsRF) > 0.01 {
		t.Errorf("insRF: want %.2f, got %.2f", wantInsRF, insRF[0])
	}

	if insKG[0] != 0 {
		t.Errorf("insKG: want 0, got %.2f", insKG[0])
	}
}

func TestCalcFOT_TaxKG(t *testing.T) {
	emp := Employee{
		Country:         CountryKG,
		BaseSchedule:    ScheduleOF,
		SalaryNet:       80_000,
		MonthlySchedule: []string{ScheduleOF},
	}
	_, ndfl, insRF, insKG := calcFOTMonthly(
		[]Employee{emp}, nil, nil, nil, mustDate(2024, 1, 1), 1,
	)

	if ndfl[0] != 0 {
		t.Errorf("ndfl for KG employee: want 0, got %.2f", ndfl[0])
	}
	if insRF[0] != 0 {
		t.Errorf("insRF for KG employee: want 0, got %.2f", insRF[0])
	}

	// Взносы КГ = 80 000 × 0.2225 = 17 800
	wantInsKG := 80_000 * 0.2225
	if math.Abs(insKG[0]-wantInsKG) > 0.01 {
		t.Errorf("insKG: want %.2f, got %.2f", wantInsKG, insKG[0])
	}
}

func TestCalcTickets(t *testing.T) {
	emp := Employee{
		BaseSchedule:    ScheduleOF,
		SalaryNet:       100_000,
		MonthlySchedule: []string{ScheduleOF, Schedule42, ScheduleMV, ScheduleOF},
	}
	// Месяц 1: ОФ→ОФ (совпадает с базой) — 0 билетов
	// Месяц 2: 4/2 — 2 билета
	// Месяц 3: МВ — смена с 4/2 → 1 билет
	// Месяц 4: ОФ — смена с МВ → 1 билет
	price := 40_000.0
	got := calcTickets([]Employee{emp}, price, 4)
	want := []float64{0, 2 * price, price, price}
	for i, w := range want {
		if math.Abs(got[i]-w) > 0.01 {
			t.Errorf("tickets month %d: want %.0f, got %.0f", i+1, w, got[i])
		}
	}
}

// TestCalcTickets_EmptySchedule — регрессия на баг из audit/numeric_baseline.md:
// незаполненная ячейка графика месяца не должна давать билет.
// Источник: 4.6!E168 = IF(E16="4/2",2,COUNTA(E16)) — COUNTA(пустая ячейка) = 0,
// т.е. Excel не считает пустой график "сменой графика".
func TestCalcTickets_EmptySchedule(t *testing.T) {
	emp := Employee{
		BaseSchedule:    ScheduleOF,
		SalaryNet:       50_000,
		MonthlySchedule: []string{""},
	}
	price := 40_000.0
	got := calcTickets([]Employee{emp}, price, 1)
	want := []float64{0}
	for i, w := range want {
		if math.Abs(got[i]-w) > 0.01 {
			t.Errorf("tickets month %d: want %.0f, got %.0f", i+1, w, got[i])
		}
	}
}

// TestCalcTickets_Regression — три ключевых случая из исправления бага в одном
// сценарии: "4/2" → 2 билета; график не изменился → 0; график сменился с
// пустого на непустое значение → 1 (не 0, как для "пусто → пусто").
func TestCalcTickets_Regression(t *testing.T) {
	emp := Employee{
		BaseSchedule:    "",
		SalaryNet:       50_000,
		MonthlySchedule: []string{Schedule42, "", ScheduleOF},
	}
	// Месяц 1: "4/2" — 2 билета (приоритет над сравнением с базой)
	// Месяц 2: "4/2" → "" — пустое текущее значение — 0 билетов
	// Месяц 3: "" → "ОФ" — смена на непустое значение — 1 билет
	price := 40_000.0
	got := calcTickets([]Employee{emp}, price, 3)
	want := []float64{2 * price, 0, price}
	for i, w := range want {
		if math.Abs(got[i]-w) > 0.01 {
			t.Errorf("tickets month %d: want %.0f, got %.0f", i+1, w, got[i])
		}
	}
}

func TestCalcTickets_NotHired(t *testing.T) {
	emp := Employee{
		BaseSchedule:    ScheduleNotHired,
		SalaryNet:       100_000,
		MonthlySchedule: []string{ScheduleNotHired, ScheduleOF, ScheduleNotHired},
	}
	// Месяц 1: не принят→не принят (база) — 0
	// Месяц 2: ОФ — смена → 1 билет
	// Месяц 3: не принят — 0
	price := 40_000.0
	got := calcTickets([]Employee{emp}, price, 3)
	want := []float64{0, price, 0}
	for i, w := range want {
		if math.Abs(got[i]-w) > 0.01 {
			t.Errorf("tickets month %d: want %.0f, got %.0f", i+1, w, got[i])
		}
	}
}
