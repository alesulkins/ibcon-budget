package calc

import (
	"math"
	"testing"
	"time"
)

// Дополнение к отклонению №1 (правило владельца 2026-08-28): если проект
// стартовал до апреля и больше половины сотрудников приняты до первого
// апреля проекта, апрельскую индексацию получает вся команда сразу —
// принятые в апреле и позже не ждут своего апреля через год.

// empHiredAt — сотрудник, принятый в месяце hire (1-based) и работающий
// до конца проекта.
func empHiredAt(hire, duration int, salary float64) Employee {
	sched := make([]string, duration)
	for m := 0; m < duration; m++ {
		if m+1 < hire {
			sched[m] = ScheduleNotHired
		} else {
			sched[m] = ScheduleOF
		}
	}
	return Employee{Country: CountryRF, SalaryNet: salary, MonthlySchedule: sched}
}

func TestProjectWideIndexation_Conditions(t *testing.T) {
	const duration = 12

	tests := []struct {
		name  string
		start time.Time
		// месяцы приёма сотрудников
		hires []int
		want  bool
	}{
		{
			// Старт январь → первый апрель проекта это 4-й месяц.
			// Двое из трёх приняты раньше → больше половины.
			name:  "старт в январе, 2 из 3 до апреля",
			start: mustDate(2027, 1, 1),
			hires: []int{1, 2, 5},
			want:  true,
		},
		{
			// Ровно половина — правило строгое, не включается.
			name:  "старт в январе, 2 из 4 до апреля",
			start: mustDate(2027, 1, 1),
			hires: []int{1, 2, 4, 5},
			want:  false,
		},
		{
			// Принятый в апреле считается «не до апреля».
			name:  "старт в марте, 1 из 3 до апреля",
			start: mustDate(2027, 3, 1),
			hires: []int{1, 2, 3},
			want:  false,
		},
		{
			// Условие 1 не выполнено: проект стартовал в апреле.
			name:  "старт в апреле",
			start: mustDate(2027, 4, 1),
			hires: []int{1, 1, 1},
			want:  false,
		},
		{
			// Условие 1 не выполнено: старт после апреля.
			name:  "старт в декабре",
			start: mustDate(2026, 12, 1),
			hires: []int{1, 1, 1},
			want:  false,
		},
		{
			// Апреля в проекте нет вовсе — индексировать нечего.
			name:  "старт в январе, проект на 2 месяца",
			start: mustDate(2027, 1, 1),
			hires: []int{1, 1},
			want:  false,
		},
		{
			name:  "сотрудников нет",
			start: mustDate(2027, 1, 1),
			hires: nil,
			want:  false,
		},
	}

	for _, tt := range tests {
		d := duration
		if tt.name == "старт в январе, проект на 2 месяца" {
			d = 2
		}
		emps := make([]Employee, 0, len(tt.hires))
		for _, h := range tt.hires {
			emps = append(emps, empHiredAt(h, d, 100_000))
		}
		if got := projectWideIndexation(emps, tt.start, d); got != tt.want {
			t.Errorf("%s: want %v, got %v", tt.name, tt.want, got)
		}
	}
}

// Сотрудник, принятый в апреле, при включённом правиле индексируется с
// того же апреля, а при выключенном — ждёт следующего года.
func TestAprilIndexation_ProjectWideCoversAprilHire(t *testing.T) {
	// Старт январь 2027 → апрель это 4-й месяц, следующий апрель — 16-й.
	start := mustDate(2027, 1, 1)
	const duration = 18
	emp := empHiredAt(4, duration, 100_000)

	// Индивидуальное правило: апрель приёма не считается (он не «строго
	// после» приёма), первая индексация — только через год.
	if got := aprilIndexation(&emp, 4, start, false); got != 1 {
		t.Errorf("индивидуально, месяц 4: want 1, got %.4f", got)
	}
	if got := aprilIndexation(&emp, 16, start, false); math.Abs(got-indexationRate) > 1e-9 {
		t.Errorf("индивидуально, месяц 16: want %.4f, got %.4f", indexationRate, got)
	}

	// Общее правило: апрель проекта даёт прибавку сразу.
	if got := aprilIndexation(&emp, 4, start, true); math.Abs(got-indexationRate) > 1e-9 {
		t.Errorf("общее правило, месяц 4: want %.4f, got %.4f", indexationRate, got)
	}
	// Второй апрель — накопительно.
	want := indexationRate * indexationRate
	if got := aprilIndexation(&emp, 16, start, true); math.Abs(got-want) > 1e-9 {
		t.Errorf("общее правило, месяц 16: want %.4f, got %.4f", want, got)
	}
}

// При включённом правиле коэффициент одинаков у всей команды, независимо
// от месяца приёма.
func TestAprilIndexation_ProjectWideSameForEveryone(t *testing.T) {
	start := mustDate(2027, 1, 1)
	const duration = 12
	early := empHiredAt(1, duration, 100_000)
	late := empHiredAt(6, duration, 100_000)

	for m := 1; m <= duration; m++ {
		a := aprilIndexation(&early, m, start, true)
		b := aprilIndexation(&late, m, start, true)
		if a != b {
			t.Errorf("месяц %d: коэффициенты разошлись — %.4f и %.4f", m, a, b)
		}
	}
}

// Сквозная проверка через расчёт ФОТ: оклад принятого в апреле вырастает
// в том же апреле, потому что правило на проекте включено.
func TestCalcFOT_ProjectWideIndexation(t *testing.T) {
	start := mustDate(2027, 1, 1)
	const duration = 12
	const salary = 100_000

	// Двое из трёх приняты до апреля → правило включается.
	emps := []Employee{
		empHiredAt(1, duration, salary),
		empHiredAt(2, duration, salary),
		empHiredAt(4, duration, salary), // принят в апреле
	}

	fot, _, _, _ := calcFOTMonthly(emps, nil, nil, nil, nil, start, duration)

	// Месяц 3: работают двое, индексации ещё не было.
	if want := 2.0 * salary; math.Abs(fot[2]-want) > 0.01 {
		t.Errorf("месяц 3: want %.2f, got %.2f", want, fot[2])
	}
	// Месяц 4 (апрель): работают трое, и все трое — по проиндексированному
	// окладу, включая принятого этим же апрелем.
	if want := 3.0 * salary * indexationRate; math.Abs(fot[3]-want) > 0.01 {
		t.Errorf("месяц 4: want %.2f, got %.2f", want, fot[3])
	}
}

// Зеркальная проверка: правило выключено — принятый в апреле остаётся на
// неиндексированном окладе, остальные индексируются.
func TestCalcFOT_IndividualIndexationWhenRuleOff(t *testing.T) {
	start := mustDate(2027, 1, 1)
	const duration = 12
	const salary = 100_000

	// Только один из трёх принят до апреля → правило не включается.
	emps := []Employee{
		empHiredAt(1, duration, salary),
		empHiredAt(4, duration, salary),
		empHiredAt(5, duration, salary),
	}

	fot, _, _, _ := calcFOTMonthly(emps, nil, nil, nil, nil, start, duration)

	// Месяц 4: первый проиндексирован, второй принят этим апрелем — нет.
	want := salary*indexationRate + salary
	if math.Abs(fot[3]-want) > 0.01 {
		t.Errorf("месяц 4: want %.2f, got %.2f", want, fot[3])
	}
}
