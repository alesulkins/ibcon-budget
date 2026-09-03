package calc

import (
	"math"
	"testing"
)

// Тесты премий и компенсаций при увольнении. ПРЕМИИ — источник ожидаемых
// значений: спецификация санкционированного отклонения №2 из CLAUDE.md, НЕ
// Excel.

// bonusEmp — сотрудник, работающий весь срок проекта.
func bonusEmp(country string, salary float64, months int) Employee {
	return Employee{
		Position: "Инженер ПТО", Country: country,
		BaseSchedule: ScheduleOF, SalaryNet: salary,
		MonthlySchedule: schedule360(ScheduleOF, months),
	}
}

// TestBonuses_BuilderDay — День строителя 20% в августе от
// проиндексированного оклада.
func TestBonuses_BuilderDay(t *testing.T) {
	start := mustDate(2026, 12, 1)
	emps := []Employee{bonusEmp("Россия", 200_000, 12)}
	bonuses := &InputBonuses{BonusTypes: []BonusType{{Kind: BonusKindBuilderDay}}}

	res := calcBonuses(emps, bonuses, start, 12)

	// Месяц 9 = август 2027: 220 000 × 20% = 44 000
	if got := res.Total[8]; math.Abs(got-44_000) > 0.01 {
		t.Errorf("август 2027: want 44000, got %.2f", got)
	}
	// В остальных месяцах премии нет (кроме последнего — там компенсация)
	for _, m := range []int{1, 5, 8, 10} {
		if got := res.Total[m-1]; got != 0 {
			t.Errorf("месяц %d: премии быть не должно, got %.2f", m, got)
		}
	}
	// Премия сотрудника РФ попала в базу НДФЛ
	if got := res.RF[8]; math.Abs(got-44_000) > 0.01 {
		t.Errorf("база РФ в августе: want 44000, got %.2f", got)
	}
	if res.KG[8] != 0 {
		t.Errorf("база КГ в августе должна быть 0, got %.2f", res.KG[8])
	}
}

// TestBonuses_NewYear — Новый год 50% в декабре.
func TestBonuses_NewYear(t *testing.T) {
	start := mustDate(2026, 12, 1)
	emps := []Employee{bonusEmp("Россия", 200_000, 24)}
	bonuses := &InputBonuses{BonusTypes: []BonusType{{Kind: BonusKindNewYear}}}

	res := calcBonuses(emps, bonuses, start, 24)

	// Месяц 1 = декабрь 2026, индексации ещё нет: 200 000 × 50% = 100 000
	if got := res.Total[0]; math.Abs(got-100_000) > 0.01 {
		t.Errorf("декабрь 2026: want 100000, got %.2f", got)
	}
	// Месяц 13 = декабрь 2027, после апреля 2027: 220 000 × 50% = 110 000
	if got := res.Total[12]; math.Abs(got-110_000) > 0.01 {
		t.Errorf("декабрь 2027: want 110000, got %.2f", got)
	}
}

// TestBonuses_GrowWithIndexation — премия растёт вместе с индексацией оклада:
// август 2027 против августа 2028.
func TestBonuses_GrowWithIndexation(t *testing.T) {
	start := mustDate(2026, 12, 1)
	emps := []Employee{bonusEmp("Россия", 200_000, 24)}
	bonuses := &InputBonuses{BonusTypes: []BonusType{{Kind: BonusKindBuilderDay}}}

	res := calcBonuses(emps, bonuses, start, 24)

	// месяц 9 = август 2027, ×1.1 → 220 000 × 20% = 44 000
	if got := res.Total[8]; math.Abs(got-44_000) > 0.01 {
		t.Errorf("август 2027: want 44000, got %.2f", got)
	}
	// месяц 21 = август 2028, ×1.21 → 242 000 × 20% = 48 400
	if got := res.Total[20]; math.Abs(got-48_400) > 0.01 {
		t.Errorf("август 2028: want 48400, got %.2f", got)
	}
}

// TestBonuses_YearlyRepeat — в проекте длиннее года премия начисляется
// каждый год (август встречается дважды).
func TestBonuses_YearlyRepeat(t *testing.T) {
	start := mustDate(2026, 12, 1)
	emps := []Employee{bonusEmp("Россия", 100_000, 24)}
	bonuses := &InputBonuses{BonusTypes: []BonusType{{Kind: BonusKindBuilderDay}}}

	res := calcBonuses(emps, bonuses, start, 24)

	paid := 0
	for m := 1; m <= 24; m++ {
		if res.Total[m-1] > 0 && m != 24 { // 24-й — компенсация, не премия
			paid++
		}
	}
	if paid != 2 {
		t.Errorf("за 24 месяца август должен встретиться 2 раза, начислений: %d", paid)
	}
}

// TestBonuses_NotHiredGetsNothing — сотруднику со статусом «не принят»
// премия не начисляется. Это единственное исключение: на «МВ» и «ОТП»
// премия считается от полного оклада (решение владельца 2026-08-22).
func TestBonuses_NotHiredGetsNothing(t *testing.T) {
	start := mustDate(2026, 12, 1)

	// месяц 9 = август 2027 — проверяем разные графики в нём
	mk := func(schedInAugust string) []Employee {
		s := schedule360(ScheduleOF, 12)
		s[8] = schedInAugust
		return []Employee{{
			Position: "Инженер ПТО", Country: "Россия", BaseSchedule: ScheduleOF,
			SalaryNet: 200_000, MonthlySchedule: s,
		}}
	}
	bonuses := &InputBonuses{BonusTypes: []BonusType{{Kind: BonusKindBuilderDay}}}

	tests := []struct {
		sched string
		want  float64
	}{
		{ScheduleOF, 44_000},  // обычный месяц
		{ScheduleMV, 44_000},  // межвахта — премия от ПОЛНОГО оклада
		{ScheduleOTP, 44_000}, // отпуск — тоже от полного оклада
		{ScheduleNotHired, 0}, // не принят — премии нет
		{"", 0},               // пустой график — премии нет
	}
	for _, tt := range tests {
		res := calcBonuses(mk(tt.sched), bonuses, start, 12)
		if got := res.Total[8]; math.Abs(got-tt.want) > 0.01 {
			t.Errorf("график %q в августе: want %.0f, got %.2f", tt.sched, tt.want, got)
		}
	}
}

// TestBonuses_CustomTypeAndOverrides — «Другое» с произвольным месяцем и
// процентом; для стандартных видов заданные значения перекрывают дефолты.
func TestBonuses_CustomTypeAndOverrides(t *testing.T) {
	start := mustDate(2026, 12, 1)
	emps := []Employee{bonusEmp("Россия", 100_000, 12)}

	// «Другое»: 15% в марте (месяц 4 проекта = март 2027)
	res := calcBonuses(emps, &InputBonuses{BonusTypes: []BonusType{
		{Kind: BonusKindOther, Name: "Квартальная", MonthNum: 3, PctOfSalary: 15},
	}}, start, 12)
	if got := res.Total[3]; math.Abs(got-15_000) > 0.01 {
		t.Errorf("«Другое» 15%% в марте: want 15000, got %.2f", got)
	}

	// Переопределённый процент у стандартного вида: 35% вместо дефолтных 20%
	res = calcBonuses(emps, &InputBonuses{BonusTypes: []BonusType{
		{Kind: BonusKindBuilderDay, PctOfSalary: 35},
	}}, start, 12)
	// месяц 9 = август 2027, оклад проиндексирован до 110 000
	if got := res.Total[8]; math.Abs(got-38_500) > 0.01 {
		t.Errorf("День строителя 35%%: want 38500, got %.2f", got)
	}
}

// TestBonuses_DoubleBonusNeedsConfirmation — две премии одному сотруднику
// в один месяц: суммируются, но помечаются как требующие подтверждения.
func TestBonuses_DoubleBonusNeedsConfirmation(t *testing.T) {
	start := mustDate(2026, 12, 1)
	emps := []Employee{bonusEmp("Россия", 100_000, 12)}

	// обе премии выпадают на август (месяц 9 проекта)
	bonuses := &InputBonuses{BonusTypes: []BonusType{
		{Kind: BonusKindBuilderDay}, // 20%, август
		{Kind: BonusKindOther, Name: "Разовая", MonthNum: 8, PctOfSalary: 5}, // 5%, август
	}}
	res := calcBonuses(emps, bonuses, start, 12)

	// оклад в августе 2027 проиндексирован: 110 000; 20% + 5% = 27 500
	if got := res.Total[8]; math.Abs(got-27_500) > 0.01 {
		t.Errorf("суммирование двух премий: want 27500, got %.2f", got)
	}
	if len(res.Conflicts) != 1 {
		t.Fatalf("ожидался 1 конфликт, got %d", len(res.Conflicts))
	}
	c := res.Conflicts[0]
	if c.MonthIdx != 9 {
		t.Errorf("месяц конфликта: want 9, got %d", c.MonthIdx)
	}
	if len(c.BonusNames) != 2 {
		t.Errorf("в конфликте должно быть 2 премии, got %v", c.BonusNames)
	}

	// Одна премия — конфликтов нет
	res = calcBonuses(emps, &InputBonuses{
		BonusTypes: []BonusType{{Kind: BonusKindBuilderDay}},
	}, start, 12)
	if len(res.Conflicts) != 0 {
		t.Errorf("одна премия не должна давать конфликт, got %v", res.Conflicts)
	}
}

// TestSeverancePay_ExcelReference — компенсация при увольнении, сверка с
// эталоном из calc_sheets_ibcon-russia.xlsm. ИСТОЧНИК: 4.1!BO11 (Excel) —
// эта часть формы рабочая.
func TestSeverancePay_ExcelReference(t *testing.T) {
	start := mustDate(2026, 12, 1)

	// Индексацию исключаем: апрель 2027 приходится на месяц 5, где график
	// «не принят» и ФОТ равен нулю, поэтому на итог она не влияет.
	emp := &Employee{
		Position: "Инженер ПТО", Country: "Россия", BaseSchedule: "вахта",
		SalaryNet: 200_000,
		MonthlySchedule: []string{
			Schedule42, ScheduleOF, ScheduleMV,
			ScheduleNotHired, ScheduleNotHired, ScheduleNotHired,
		},
	}

	got := severancePay(emp, start, 6)
	if math.Abs(got-45_606.06060606061) > 0.01 {
		t.Errorf("компенсация сотрудника 1: want 45606.06, got %.2f", got)
	}

	// Второй эталон — сотрудник 3 того же файла: ФОТ по месяцам [100000,
	// 100000, 100000, 100000, 100000, 30000] = 530 000, последний месяц 30 000.
	// Эталон 4.1!BO13 = 116 212.12.
	if got := 530_000.0/severanceWorkDays*severanceCalendarDays/severanceMonthsInYear +
		severanceLastMonthsX*30_000.0; math.Abs(got-116_212.12121212122) > 0.01 {
		t.Errorf("формула на данных сотрудника 3: want 116212.12, got %.2f", got)
	}
}

// TestSeverancePay_NeverWorked — не работавшему ни одного месяца
// компенсация не начисляется (защита IF('2.Бюджет'!G14=0; 0; ...)).
func TestSeverancePay_NeverWorked(t *testing.T) {
	start := mustDate(2026, 12, 1)
	emp := &Employee{
		Country: "Россия", SalaryNet: 200_000,
		MonthlySchedule: schedule360(ScheduleNotHired, 6),
	}
	if got := severancePay(emp, start, 6); got != 0 {
		t.Errorf("не работал ни разу: want 0, got %.2f", got)
	}
}

// TestSeverancePay_PaidEvenIfNotHiredInLastMonth — компенсация начисляется
// каждому, кто хоть когда-нибудь работал, даже если в последнем месяце у
// него стоит «не принят».
func TestSeverancePay_PaidEvenIfNotHiredInLastMonth(t *testing.T) {
	start := mustDate(2026, 12, 1)
	emps := []Employee{{
		Position: "Инженер ПТО", Country: "Россия", BaseSchedule: "вахта",
		SalaryNet: 200_000,
		MonthlySchedule: []string{
			Schedule42, ScheduleOF, ScheduleMV,
			ScheduleNotHired, ScheduleNotHired, ScheduleNotHired,
		},
	}}

	res := calcBonuses(emps, nil, start, 6)

	if got := res.Total[5]; math.Abs(got-45_606.06060606061) > 0.01 {
		t.Errorf("компенсация уволенному до конца проекта: want 45606.06, got %.2f", got)
	}
	// Она облагается налогами наравне с окладом
	if got := res.RF[5]; math.Abs(got-45_606.06060606061) > 0.01 {
		t.Errorf("компенсация должна попасть в базу РФ: got %.2f", got)
	}
}

// TestBonuses_IncreaseTaxBase — премия увеличивает базу НДФЛ и взносов.
func TestBonuses_IncreaseTaxBase(t *testing.T) {
	start := mustDate(2026, 12, 1)
	emps := []Employee{bonusEmp("Россия", 200_000, 12)}
	bonuses := &InputBonuses{BonusTypes: []BonusType{{Kind: BonusKindBuilderDay}}}

	res := calcBonuses(emps, bonuses, start, 12)
	_, ndflWith, insWith, _ := calcFOTMonthly(emps, res.RF, res.KG, nil, nil, start, 12)
	_, ndflNo, insNo, _ := calcFOTMonthly(emps, nil, nil, nil, nil, start, 12)

	// Месяц 9 = август 2027: ФОТ 220 000, премия 44 000
	wantBase := 220_000.0 + 44_000.0
	wantNDFL := wantBase/0.85 - wantBase
	if math.Abs(ndflWith[8]-wantNDFL) > 0.01 {
		t.Errorf("НДФЛ с премией: want %.2f, got %.2f", wantNDFL, ndflWith[8])
	}
	if ndflWith[8] <= ndflNo[8] {
		t.Errorf("премия должна увеличивать НДФЛ: без премии %.2f, с премией %.2f",
			ndflNo[8], ndflWith[8])
	}
	if insWith[8] <= insNo[8] {
		t.Errorf("премия должна увеличивать взносы: без %.2f, с %.2f",
			insNo[8], insWith[8])
	}
}

// TestBonuses_SelfEmployedNotTaxed — премия самозанятому начисляется,
// но в налоговую базу не попадает.
func TestBonuses_SelfEmployedNotTaxed(t *testing.T) {
	start := mustDate(2026, 12, 1)
	emps := []Employee{bonusEmp("Самозанятый, без НО", 100_000, 12)}
	bonuses := &InputBonuses{BonusTypes: []BonusType{{Kind: BonusKindBuilderDay}}}

	res := calcBonuses(emps, bonuses, start, 12)

	if got := res.Total[8]; math.Abs(got-22_000) > 0.01 {
		t.Errorf("премия самозанятому: want 22000 (110000×20%%), got %.2f", got)
	}
	if res.RF[8] != 0 || res.KG[8] != 0 {
		t.Errorf("премия самозанятого не должна попадать в налоговую базу: РФ=%.2f КГ=%.2f",
			res.RF[8], res.KG[8])
	}
}
