package calc

import "time"

// interShiftPay — фиксированная выплата за межвахтовый отдых («МВ»).
const interShiftPay = 30_000.0

// Ставки налогов и взносов по ФОТ. Все три зашиты в формулы формы числами
// (2.Бюджет!H172, H173, H174) — полей ввода под них нет.
const (
	// ndflGrossUpDivisor — приведение «на руки» к начисленному: 0.85 = 1 - 0.15.
	ndflGrossUpDivisor = 0.85
	// insuranceRFRate — страховые взносы РФ, доля от начисленного (30.2%).
	insuranceRFRate = 0.302
	// insuranceKGRate — страховые взносы Киргизии, доля от «на руки»
	// (22.25%). База другая: не от начисленного, а от суммы на руки.
	insuranceKGRate = 0.2225
)

// indexationRate — коэффициент ежегодной индексации ФОТ.
// ТЗ п.6 и критерий приёмки 9.1.15. В эталонной форме индексации НЕТ
// (4.6!$BT16 — константа), это санкционированное отклонение №1.
const indexationRate = 1.1

// scheduleMultiplier возвращает долю месяца, за которую начисляется ФОТ.
func scheduleMultiplier(schedule string, salaryNet float64) float64 {
	switch schedule {
	case ScheduleMV:
		if salaryNet == 0 {
			return 0
		}
		return interShiftPay / salaryNet
	case ScheduleNotHired:
		return 0
	default: // ОФ, 4/2, 4/4, К, ОТП
		return 1
	}
}

// monthDate возвращает календарную дату месяца проекта monthIdx (1-based).
// time.Date сам нормализует переполнение месяцев, а день фиксируем первым —
// иначе старт вида «31 января» съезжал бы при сложении месяцев.
func monthDate(startDate time.Time, monthIdx int) time.Time {
	return time.Date(
		startDate.Year(), startDate.Month()+time.Month(monthIdx-1), 1,
		0, 0, 0, 0, startDate.Location(),
	)
}

// hireMonth — номер месяца проекта (1-based), в котором сотрудник появляется
// впервые: первый месяц, где график не «не принят» и не пустой.
func hireMonth(emp *Employee) int {
	for m := 1; m <= len(emp.MonthlySchedule); m++ {
		if s := emp.MonthlySchedule[m-1]; s != "" && s != ScheduleNotHired {
			return m
		}
	}
	return 0
}

// aprilIndexation возвращает накопительный коэффициент индексации ФОТ
// сотрудника на месяц monthIdx (1-based).
func aprilIndexation(emp *Employee, monthIdx int, startDate time.Time) float64 {
	hire := hireMonth(emp)
	if hire == 0 || monthIdx <= hire {
		return 1
	}

	mult := 1.0
	for m := hire + 1; m <= monthIdx; m++ {
		if monthDate(startDate, m).Month() == time.April {
			mult *= indexationRate
		}
	}
	return mult
}

// employeeFOT рассчитывает ФОТ одного сотрудника за один месяц (monthIdx
// 1-based). Формула Excel 4.6!ED16 = BU16*$BT16, где BU16 — множитель
// графика.
func employeeFOT(emp *Employee, monthIdx int, startDate time.Time) float64 {
	if monthIdx < 1 || monthIdx > len(emp.MonthlySchedule) {
		return 0
	}
	salary := emp.SalaryNet * aprilIndexation(emp, monthIdx, startDate)
	return EffectiveMultiplier(emp, monthIdx, salary) * salary
}

// EmployeeFOTAt — ФОТ одного сотрудника за месяц monthIdx (1-based).
func EmployeeFOTAt(emp *Employee, monthIdx int, startDate time.Time) float64 {
	return employeeFOT(emp, monthIdx, startDate)
}

// EffectiveMultiplier — множитель графика, который реально идёт в расчёт ФОТ
// за месяц monthIdx (1-based): ручное значение, если экономист его задал,
// иначе вычисленное по формуле формы.
func EffectiveMultiplier(emp *Employee, monthIdx int, salary float64) float64 {
	if monthIdx >= 1 && monthIdx <= len(emp.MultiplierOverrides) {
		if v := emp.MultiplierOverrides[monthIdx-1]; v != nil {
			return *v
		}
	}
	if monthIdx < 1 || monthIdx > len(emp.MonthlySchedule) {
		return 0
	}
	return scheduleMultiplier(emp.MonthlySchedule[monthIdx-1], salary)
}

// calcFOTMonthly рассчитывает общий ФОТ и налоги по всем сотрудникам за
// каждый месяц.
func calcFOTMonthly(
	emps []Employee,
	bonusRF []float64,
	bonusKG []float64,
	overtimeRF []float64,
	overtimeKG []float64,
	startDate time.Time,
	duration int,
) (fotArr, ndflArr, insRFArr, insKGArr []float64) {
	fotArr = make([]float64, duration)
	ndflArr = make([]float64, duration)
	insRFArr = make([]float64, duration)
	insKGArr = make([]float64, duration)

	for m := 1; m <= duration; m++ {
		var totalFOT, rfNet, kgNet float64
		for i := range emps {
			fot := employeeFOT(&emps[i], m, startDate)
			totalFOT += fot
			// Excel сравнивает страну через SUMIF, то есть регистронезависимо
			// (2.Бюджет!H172, H173, H174). Самозанятый не попадает ни в одну
			// ветку — он платит налоги сам.
			switch normalizeCountry(emps[i].Country) {
			case CountryRF:
				rfNet += fot
			case CountryKG:
				kgNet += fot
			}
		}
		fotArr[m-1] = totalFOT

		// Премии и компенсации (4.1), уже разложенные по стране в calcBonuses
		rfBonus := lineVal(bonusRF, m-1)
		kgBonus := lineVal(bonusKG, m-1)

		// Переработки
		var ovRF, ovKG float64
		if m <= len(overtimeRF) {
			ovRF = overtimeRF[m-1]
		}
		if m <= len(overtimeKG) {
			ovKG = overtimeKG[m-1]
		}

		// "На руки" суммарно по каждой стране (с бонусами и переработками)
		rfBase := rfNet + rfBonus + ovRF
		kgBase := kgNet + kgBonus + ovKG

		// НДФЛ для сотрудников РФ.
		// Формула Excel H172: SUMIF(россия, salary)/0.85 - SUMIF(россия, salary)
		// = rfBase/0.85 - rfBase = rfBase × (1/0.85 - 1)
		ndflArr[m-1] = rfBase/ndflGrossUpDivisor - rfBase

		// Взносы сотрудников РФ 30.2% от gross.
		// Формула H173: (rfNet + rfBonus + ovRF) / 0.85 × 0.302
		insRFArr[m-1] = rfBase / ndflGrossUpDivisor * insuranceRFRate

		// Взносы сотрудников Киргизии 22.25% от net (не от gross).
		// Формула H174: (kgNet + kgBonus + ovKG) × 0.2225
		insKGArr[m-1] = kgBase * insuranceKGRate
	}

	return
}

// scheduleAt возвращает график сотрудника за месяц monthIdx (1-based),
// пустую строку — если месяц за пределами заполненной сетки.
func scheduleAt(e *Employee, monthIdx int) string {
	if monthIdx >= 1 && monthIdx <= len(e.MonthlySchedule) {
		return e.MonthlySchedule[monthIdx-1]
	}
	return ""
}

// shiftTicketsRowwise — количество билетов вахтовиков за месяц как сумма
// построчных значений по сотрудникам (Excel: SUM(E168:E317)).
func shiftTicketsRowwise(emps []Employee, monthIdx int) float64 {
	var count float64
	for i := range emps {
		e := &emps[i]
		cur := scheduleAt(e, monthIdx)

		if cur == Schedule42 {
			count += 2
			continue
		}
		prev := e.BaseSchedule
		if monthIdx > 1 {
			prev = scheduleAt(e, monthIdx-1)
		}
		if cur == ScheduleK || cur == prev || cur == ScheduleNotHired || cur == "" {
			continue
		}
		count++ // COUNTA(непустая ячейка) = 1
	}
	return count
}

// shiftTicketsLastMonth — количество билетов вахтовиков за ПОСЛЕДНИЙ месяц
// проекта.
func shiftTicketsLastMonth(emps []Employee, monthIdx int) float64 {
	var counta, notHired, k, sched42 float64
	for i := range emps {
		switch scheduleAt(&emps[i], monthIdx) {
		case "":
			// COUNTA пустую ячейку не считает
		case ScheduleNotHired:
			counta++
			notHired++
		case ScheduleK:
			counta++
			k++
		case Schedule42:
			counta++
			sched42++
		default:
			counta++
		}
	}
	return counta - k - notHired + sched42
}

// businessTripTickets — количество билетов в командировках за месяц
// (Excel: SUM(E629:E778), построчно 4.6!E629 = IF(месяц<=D8, IF(график="К", 2, 0), 0)).
// Формула единообразна во всех месяцах, включая последний.
func businessTripTickets(emps []Employee, monthIdx int) float64 {
	var count float64
	for i := range emps {
		if scheduleAt(&emps[i], monthIdx) == ScheduleK {
			count += 2
		}
	}
	return count
}

// calcTickets рассчитывает стоимость авиабилетов по месяцам (строка 184 =
// 4.6!E5 = $C$5*(E318+E779)).
func calcTickets(emps []Employee, ticketPrice float64, duration int) []float64 {
	tickets := make([]float64, duration)
	for m := 1; m <= duration; m++ {
		var shift float64
		if m == duration {
			shift = shiftTicketsLastMonth(emps, m)
		} else {
			shift = shiftTicketsRowwise(emps, m)
		}
		tickets[m-1] = ticketPrice * (shift + businessTripTickets(emps, m))
	}
	return tickets
}

// calcPerDiem рассчитывает командировочные расходы по месяцам (строка 185).
// D9 * сумма_дней_РФ + D10 * сумма_дней_других_стран
func calcPerDiem(emps []Employee, perDiemRF, perDiemOther float64, duration int) []float64 {
	perDiem := make([]float64, duration)
	for i := range emps {
		e := &emps[i]
		for m := 1; m <= duration; m++ {
			var daysRF, daysOther int
			if m <= len(e.TripDaysRF) {
				daysRF = e.TripDaysRF[m-1]
			}
			if m <= len(e.TripDaysOther) {
				daysOther = e.TripDaysOther[m-1]
			}
			perDiem[m-1] += float64(daysRF)*perDiemRF + float64(daysOther)*perDiemOther
		}
	}
	return perDiem
}
