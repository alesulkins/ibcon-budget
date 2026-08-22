package calc

import (
	"math"
	"testing"
)

// Тесты индексации ФОТ.
//
// ИСТОЧНИК ОЖИДАЕМЫХ ЗНАЧЕНИЙ: спецификация санкционированного отклонения
// №1 из CLAUDE.md, НЕ Excel. В эталонной форме индексации нет вообще —
// зарплата это константа 4.6!$BT16, на которую все ~60 месячных колонок
// ссылаются одной абсолютной ссылкой. Сверять эти значения с формой
// невозможно по определению, расхождение с ней здесь ожидаемо.
//
// Механика: множитель ×1.1 накопительно за каждый апрель СТРОГО ПОСЛЕ
// месяца приёма сотрудника (первый → 1.1, второй → 1.21, третий → 1.331).

// schedule360 — вспомогательный конструктор графика: n месяцев подряд
// с одним и тем же значением.
func schedule360(value string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = value
	}
	return out
}

// TestAprilIndexation_ThreeYears — проект на 3 года со стартом в декабре.
// Проверяем помесячно все три ступени индексации.
func TestAprilIndexation_ThreeYears(t *testing.T) {
	start := mustDate(2026, 12, 1) // месяц 1 = декабрь 2026
	emp := &Employee{
		Country:         CountryRF,
		BaseSchedule:    ScheduleOF,
		SalaryNet:       200_000,
		MonthlySchedule: schedule360(ScheduleOF, 36),
	}

	tests := []struct {
		month int
		date  string
		want  float64
	}{
		{1, "дек 2026", 200_000},  // до первого апреля — базовый оклад
		{4, "мар 2027", 200_000},  // март, апреля ещё не было
		{5, "апр 2027", 220_000},  // 1-й апрель после приёма → ×1.1
		{6, "май 2027", 220_000},  // множитель держится до следующего апреля
		{16, "мар 2028", 220_000}, // последний месяц перед 2-м апрелем
		{17, "апр 2028", 242_000}, // 2-й апрель → ×1.21
		{28, "мар 2029", 242_000},
		{29, "апр 2029", 266_200}, // 3-й апрель → ×1.331
		{36, "ноя 2029", 266_200},
	}
	for _, tt := range tests {
		got := employeeFOT(emp, tt.month, start)
		if math.Abs(got-tt.want) > 0.01 {
			t.Errorf("месяц %d (%s): got %.2f, want %.2f", tt.month, tt.date, got, tt.want)
		}
	}
}

// TestAprilIndexation_HiredInMay — сотрудник принят в мае: апрель того же
// года он не застал, первая индексация — только в следующем апреле.
func TestAprilIndexation_HiredInMay(t *testing.T) {
	start := mustDate(2026, 12, 1) // месяц 1 = декабрь 2026

	sched := schedule360(ScheduleOF, 36)
	for i := 0; i < 5; i++ { // месяцы 1..5 (дек 2026 — апр 2027) — не принят
		sched[i] = ScheduleNotHired
	}
	emp := &Employee{
		Country: CountryRF, BaseSchedule: ScheduleOF,
		SalaryNet: 100_000, MonthlySchedule: sched,
	}

	if got := hireMonth(emp); got != 6 {
		t.Fatalf("месяц приёма: got %d, want 6 (май 2027)", got)
	}

	tests := []struct {
		month int
		date  string
		want  float64
	}{
		{5, "апр 2027", 0},        // ещё «не принят» → ФОТ 0
		{6, "май 2027", 100_000},  // принят, апрель 2027 пропущен → ×1.0
		{16, "мар 2028", 100_000}, // весь первый год — базовый оклад
		{17, "апр 2028", 110_000}, // первая индексация через год → ×1.1
		{29, "апр 2029", 121_000}, // вторая → ×1.21
	}
	for _, tt := range tests {
		got := employeeFOT(emp, tt.month, start)
		if math.Abs(got-tt.want) > 0.01 {
			t.Errorf("месяц %d (%s): got %.2f, want %.2f", tt.month, tt.date, got, tt.want)
		}
	}
}

// TestAprilIndexation_ProjectStartsInApril — проект стартует в апреле.
// Апрель месяца приёма НЕ индексирует (апрели считаются строго после
// месяца приёма), следующий апрель — индексирует.
func TestAprilIndexation_ProjectStartsInApril(t *testing.T) {
	start := mustDate(2027, 4, 1) // месяц 1 = апрель 2027
	emp := &Employee{
		Country: CountryRF, BaseSchedule: ScheduleOF,
		SalaryNet: 150_000, MonthlySchedule: schedule360(ScheduleOF, 30),
	}

	tests := []struct {
		month int
		date  string
		want  float64
	}{
		{1, "апр 2027", 150_000},  // апрель приёма — без индексации
		{12, "мар 2028", 150_000}, // весь первый год
		{13, "апр 2028", 165_000}, // следующий апрель → ×1.1
		{25, "апр 2029", 181_500}, // ещё через год → ×1.21
	}
	for _, tt := range tests {
		got := employeeFOT(emp, tt.month, start)
		if math.Abs(got-tt.want) > 0.01 {
			t.Errorf("месяц %d (%s): got %.2f, want %.2f", tt.month, tt.date, got, tt.want)
		}
	}
}

// TestAprilIndexation_RehiredKeepsSeniority — сотрудник уволился и вернулся:
// стаж считается от ПЕРВОГО появления и не обнуляется.
func TestAprilIndexation_RehiredKeepsSeniority(t *testing.T) {
	start := mustDate(2026, 12, 1)

	sched := schedule360(ScheduleOF, 24)
	// месяцы 7..10 — уволен, потом снова принят
	for i := 6; i < 10; i++ {
		sched[i] = ScheduleNotHired
	}
	emp := &Employee{
		Country: CountryRF, BaseSchedule: ScheduleOF,
		SalaryNet: 100_000, MonthlySchedule: sched,
	}

	if got := hireMonth(emp); got != 1 {
		t.Fatalf("месяц приёма при перерыве: got %d, want 1 (первое появление)", got)
	}

	// Месяц 11 (окт 2027) — уже после возврата. Апрель 2027 (месяц 5)
	// засчитан, потому что стаж не обнулялся.
	if got := employeeFOT(emp, 11, start); math.Abs(got-110_000) > 0.01 {
		t.Errorf("после возврата: got %.2f, want 110000 (стаж сохранён)", got)
	}
	// Месяц 17 (апр 2028) — второй апрель → ×1.21
	if got := employeeFOT(emp, 17, start); math.Abs(got-121_000) > 0.01 {
		t.Errorf("второй апрель после возврата: got %.2f, want 121000", got)
	}
}

// TestAprilIndexation_InterShiftPayStaysFixed — выплата за межвахтовый
// отдых остаётся 30 000 при любой индексации.
//
// Причина: множитель графика «МВ» равен 30000/оклад, а сумма считается как
// множитель × оклад, поэтому оклад сокращается: (30000/S)×S = 30000 при
// любом S. Индексируется «План ФОТ на руки», а 30 000 в форме зашито
// числом (4.6!BU16) и долей оклада не является.
func TestAprilIndexation_InterShiftPayStaysFixed(t *testing.T) {
	start := mustDate(2026, 12, 1)

	sched := schedule360(ScheduleMV, 36) // всё время межвахтовый отдых
	emp := &Employee{
		Country: CountryRF, BaseSchedule: "вахта",
		SalaryNet: 200_000, MonthlySchedule: sched,
	}

	// Месяцы до и после каждой ступени индексации — везде ровно 30 000
	for _, m := range []int{1, 4, 5, 17, 29, 36} {
		if got := employeeFOT(emp, m, start); math.Abs(got-interShiftPay) > 0.01 {
			t.Errorf("месяц %d: МВ должен оставаться %.0f, got %.2f",
				m, interShiftPay, got)
		}
	}

	// При этом сам множитель индексации на этот месяц не равен единице —
	// то есть фиксированность выплаты не следствие отсутствия индексации.
	if k := aprilIndexation(emp, 29, start); math.Abs(k-1.331) > 1e-9 {
		t.Errorf("множитель индексации на месяц 29: got %.4f, want 1.331", k)
	}
}

// TestAprilIndexation_TaxesFollowIndexedSalary — НДФЛ и страховые взносы
// считаются от ПРОИНДЕКСИРОВАННОЙ суммы (требование отклонения №1).
func TestAprilIndexation_TaxesFollowIndexedSalary(t *testing.T) {
	start := mustDate(2026, 12, 1)
	emps := []Employee{{
		Country: "Россия", BaseSchedule: ScheduleOF,
		SalaryNet: 200_000, MonthlySchedule: schedule360(ScheduleOF, 12),
	}}

	fot, ndfl, insRF, _ := calcFOTMonthly(emps, nil, nil, nil, start, 12)

	// Месяц 4 (март 2027) — до индексации
	wantFOT4 := 200_000.0
	wantNDFL4 := wantFOT4/0.85 - wantFOT4
	wantIns4 := wantFOT4 / 0.85 * 0.302
	// Месяц 5 (апрель 2027) — после индексации ×1.1
	wantFOT5 := 220_000.0
	wantNDFL5 := wantFOT5/0.85 - wantFOT5
	wantIns5 := wantFOT5 / 0.85 * 0.302

	checks := []struct {
		name      string
		got, want float64
	}{
		{"ФОТ месяц 4", fot[3], wantFOT4},
		{"НДФЛ месяц 4", ndfl[3], wantNDFL4},
		{"взносы РФ месяц 4", insRF[3], wantIns4},
		{"ФОТ месяц 5", fot[4], wantFOT5},
		{"НДФЛ месяц 5", ndfl[4], wantNDFL5},
		{"взносы РФ месяц 5", insRF[4], wantIns5},
	}
	for _, c := range checks {
		if math.Abs(c.got-c.want) > 0.01 {
			t.Errorf("%s: got %.2f, want %.2f", c.name, c.got, c.want)
		}
	}

	// Налог должен вырасти ровно пропорционально индексации (×1.1)
	if ratio := ndfl[4] / ndfl[3]; math.Abs(ratio-indexationRate) > 1e-9 {
		t.Errorf("рост НДФЛ в апреле: got ×%.6f, want ×%.1f", ratio, indexationRate)
	}
	if ratio := insRF[4] / insRF[3]; math.Abs(ratio-indexationRate) > 1e-9 {
		t.Errorf("рост взносов в апреле: got ×%.6f, want ×%.1f", ratio, indexationRate)
	}
}

// TestAprilIndexation_Multiplier — множитель по годам отдельно от ФОТ.
func TestAprilIndexation_Multiplier(t *testing.T) {
	start := mustDate(2026, 12, 1)
	emp := &Employee{
		BaseSchedule: ScheduleOF, SalaryNet: 100_000,
		MonthlySchedule: schedule360(ScheduleOF, 36),
	}

	tests := []struct {
		month int
		want  float64
	}{
		{1, 1.0}, {4, 1.0}, {5, 1.1}, {16, 1.1},
		{17, 1.21}, {28, 1.21}, {29, 1.331}, {36, 1.331},
	}
	for _, tt := range tests {
		if got := aprilIndexation(emp, tt.month, start); math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("множитель на месяц %d: got %.6f, want %.4f", tt.month, got, tt.want)
		}
	}

	// Сотрудник, не принятый ни в одном месяце: индексации нет
	never := &Employee{MonthlySchedule: schedule360(ScheduleNotHired, 12)}
	if got := aprilIndexation(never, 12, start); got != 1 {
		t.Errorf("не принят ни разу: множитель got %.4f, want 1", got)
	}
}
