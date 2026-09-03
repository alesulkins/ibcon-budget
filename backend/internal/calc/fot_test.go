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
		{ScheduleMV, 100_000, 0.3}, // 30000/100000
		{ScheduleMV, 60_000, 0.5},  // 30000/60000
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
		{2, 100_000}, // ОФ (февраль, апреля ещё не было)
		{3, 30_000},  // МВ — фиксированные 30 000 (формула Excel: IF(МВ, 30000/salary, ...)*salary)
		{4, 0},       // не принят (апрель 2024, но график «не принят» → ФОТ 0)
		// Месяц 5 = май 2024. Апрель 2024 (месяц 4) уже прошёл после месяца
		// приёма (месяц 1), поэтому оклад проиндексирован: 100 000 × 1.1.
		// Источник: спецификация отклонения №1, не Excel.
		{5, 110_000},
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
		[]Employee{emp}, nil, nil, nil, nil, mustDate(2024, 1, 1), 1,
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
		[]Employee{emp}, nil, nil, nil, nil, mustDate(2024, 1, 1), 1,
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

// TestCalcFOT_CountryCaseInsensitive — страна сравнивается
// регистронезависимо, как SUMIF в Excel (2.Бюджет!H172, H173, H174).
func TestCalcFOT_CountryCaseInsensitive(t *testing.T) {
	wantNDFL := 100_000.0/0.85 - 100_000.0
	wantInsRF := 100_000.0 / 0.85 * 0.302

	for _, country := range []string{"россия", "Россия", "РОССИЯ", "  Россия  "} {
		emp := Employee{
			Country: country, BaseSchedule: ScheduleOF,
			SalaryNet: 100_000, MonthlySchedule: []string{ScheduleOF},
		}
		_, ndfl, insRF, _ := calcFOTMonthly([]Employee{emp}, nil, nil, nil, nil, mustDate(2024, 1, 1), 1)
		if math.Abs(ndfl[0]-wantNDFL) > 0.01 {
			t.Errorf("страна %q: НДФЛ want %.2f, got %.2f", country, wantNDFL, ndfl[0])
		}
		if math.Abs(insRF[0]-wantInsRF) > 0.01 {
			t.Errorf("страна %q: взносы РФ want %.2f, got %.2f", country, wantInsRF, insRF[0])
		}
	}

	for _, country := range []string{"киргизия", "Киргизия", "КИРГИЗИЯ"} {
		emp := Employee{
			Country: country, BaseSchedule: ScheduleOF,
			SalaryNet: 80_000, MonthlySchedule: []string{ScheduleOF},
		}
		_, _, _, insKG := calcFOTMonthly([]Employee{emp}, nil, nil, nil, nil, mustDate(2024, 1, 1), 1)
		if math.Abs(insKG[0]-80_000*0.2225) > 0.01 {
			t.Errorf("страна %q: взносы КГ want %.2f, got %.2f",
				country, 80_000*0.2225, insKG[0])
		}
	}
}

// TestCalcFOT_SelfEmployed — самозанятый платит налоги сам: платформа не
// начисляет ни НДФЛ, ни страховых взносов. При этом его ФОТ в строку 168
// попадает как обычно.
func TestCalcFOT_SelfEmployed(t *testing.T) {
	emp := Employee{
		Country: "Самозанятый, без НО", BaseSchedule: ScheduleOF,
		SalaryNet: 150_000, MonthlySchedule: []string{ScheduleOF},
	}
	fot, ndfl, insRF, insKG := calcFOTMonthly(
		[]Employee{emp}, nil, nil, nil, nil, mustDate(2024, 1, 1), 1)

	if math.Abs(fot[0]-150_000) > 0.01 {
		t.Errorf("ФОТ самозанятого должен считаться: want 150000, got %.2f", fot[0])
	}
	if ndfl[0] != 0 || insRF[0] != 0 || insKG[0] != 0 {
		t.Errorf("самозанятый: налоги должны быть нулевыми, got НДФЛ=%.2f взносыРФ=%.2f взносыКГ=%.2f",
			ndfl[0], insRF[0], insKG[0])
	}

	// Константа должна совпадать со значением справочника после нормализации
	if normalizeCountry("Самозанятый, без НО") != CountrySelfEmployed {
		t.Errorf("normalizeCountry(«Самозанятый, без НО») = %q, want %q",
			normalizeCountry("Самозанятый, без НО"), CountrySelfEmployed)
	}
}

// TestCalcFOT_ReferenceValues — эталон из calc_sheets_ibcon-russia.xlsm,
// месяц 1: два сотрудника РФ (200 000 и 50 000) и один КГ (100 000), плюс
// премии из листа 4.1 (100 000 РФ и 50 000 КГ).
func TestCalcFOT_ReferenceValues(t *testing.T) {
	emps := []Employee{
		// 4.6!BS16=Россия, BT16=200000, график месяца 1 = «4/2» → полный оклад
		{Country: "Россия", BaseSchedule: "вахта", SalaryNet: 200_000,
			MonthlySchedule: []string{Schedule42}},
		// 4.6!BS17=Россия, BT17=50000, график «не принят» → ФОТ 0
		{Country: "Россия", BaseSchedule: "вахта", SalaryNet: 50_000,
			MonthlySchedule: []string{ScheduleNotHired}},
		// 4.6!BS18=Киргизия, BT18=100000, график «К» → полный оклад
		{Country: "Киргизия", BaseSchedule: "офис", SalaryNet: 100_000,
			MonthlySchedule: []string{ScheduleK}},
	}
	// Премии месяца 1 из 4.1: 100 000 у сотрудника РФ, 50 000 у КГ.
	// Передаём уже разложенными по странам — так их отдаёт calcBonuses.
	bonusRF := []float64{100_000}
	bonusKG := []float64{50_000}
	fot, ndfl, insRF, insKG := calcFOTMonthly(
		emps, bonusRF, bonusKG, nil, nil, mustDate(2026, 12, 1), 1)

	checks := []struct {
		name string
		got  float64
		want float64
	}{
		{"ФОТ (2.Бюджет!H168)", fot[0], 300_000},
		{"НДФЛ (2.Бюджет!H172)", ndfl[0], 52_941.17647058825},
		{"взносы РФ (2.Бюджет!H173)", insRF[0], 106_588.23529411765},
		{"взносы КГ (2.Бюджет!H174)", insKG[0], 33_375},
	}
	for _, c := range checks {
		if math.Abs(c.got-c.want) > 0.01 {
			t.Errorf("%s: want %.5f, got %.5f", c.name, c.want, c.got)
		}
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

// TestCalcTickets_EmptySchedule — регрессия на баг из
// audit/numeric_baseline.md: незаполненная ячейка графика месяца не должна
// давать билет.
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

// TestCalcTickets_LastMonthCounta — в ПОСЛЕДНИЙ месяц проекта билет получают
// все, кто на объекте: домой уезжают все.
func TestCalcTickets_LastMonthCounta(t *testing.T) {
	price := 40_000.0

	// Три сотрудника, 6 месяцев. В последнем месяце у сотрудника 3
	// график ОТП — тот же, что в пятом месяце.
	emps := []Employee{
		{BaseSchedule: "вахта", MonthlySchedule: []string{
			Schedule42, ScheduleOF, ScheduleMV, ScheduleNotHired, ScheduleNotHired, ScheduleNotHired}},
		{BaseSchedule: "вахта", MonthlySchedule: []string{
			ScheduleNotHired, Schedule44, Schedule42, ScheduleMV, Schedule42, ScheduleNotHired}},
		{BaseSchedule: "офис", MonthlySchedule: []string{
			ScheduleK, Schedule42, Schedule44, ScheduleOF, ScheduleOTP, ScheduleOTP}},
	}

	got := calcTickets(emps, price, 6)

	// Месяц 6 (последний): СЧЁТЗ=3, «не принят»=2, «К»=0, «4/2»=0
	// → 3 - 0 - 2 + 0 = 1 билет, несмотря на повтор ОТП→ОТП.
	if math.Abs(got[5]-1*price) > 0.01 {
		t.Errorf("последний месяц (ОТП→ОТП): want %.0f (1 билет через СЧЁТЗ), got %.0f",
			price, got[5])
	}

	// Контроль: та же ситуация НЕ в последнем месяце должна дать 0 билетов
	// по построчной формуле. Продлеваем проект до 7 месяцев — месяц 6
	// перестаёт быть последним.
	for i := range emps {
		emps[i].MonthlySchedule = append(emps[i].MonthlySchedule, ScheduleNotHired)
	}
	got7 := calcTickets(emps, price, 7)
	if math.Abs(got7[5]-0) > 0.01 {
		t.Errorf("месяц 6 из 7 (ОТП→ОТП, не последний): want 0, got %.0f", got7[5])
	}
}

// TestCalcTickets_LastMonthComponents — слагаемые СЧЁТЗ-ветки по отдельности.
func TestCalcTickets_LastMonthComponents(t *testing.T) {
	price := 1.0 // цена 1 — чтобы результат читался как количество билетов

	tests := []struct {
		name      string
		lastMonth []string // график каждого сотрудника в последнем месяце
		want      float64
	}{
		// СЧЁТЗ=1, вычетов нет → 1
		{"обычный график", []string{ScheduleOF}, 1},
		// СЧЁТЗ=1, «не принят»=1 → 0 (человека на объекте нет)
		{"не принят", []string{ScheduleNotHired}, 0},
		// СЧЁТЗ=1, «4/2»=1 → 1+1 = 2 билета
		{"4/2 даёт два билета", []string{Schedule42}, 2},
		// СЧЁТЗ=1, «К»=1 → 0 по строке 318, но +2 по строке 779 → 2
		{"командировка", []string{ScheduleK}, 2},
		// пустой график не считается СЧЁТЗ
		{"пустой график", []string{""}, 0},
		// смесь: ОФ(1) + не принят(0) + 4/2(2) + К(2)
		{"смесь", []string{ScheduleOF, ScheduleNotHired, Schedule42, ScheduleK}, 5},
	}

	for _, tt := range tests {
		emps := make([]Employee, len(tt.lastMonth))
		for i, s := range tt.lastMonth {
			// два месяца: во втором (последнем) — проверяемый график
			emps[i] = Employee{BaseSchedule: "офис", MonthlySchedule: []string{ScheduleOF, s}}
		}
		got := calcTickets(emps, price, 2)
		if math.Abs(got[1]-tt.want) > 0.01 {
			t.Errorf("%s: want %.0f билетов, got %.0f", tt.name, tt.want, got[1])
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
