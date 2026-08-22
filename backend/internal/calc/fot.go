package calc

import "time"

// scheduleMultiplier возвращает долю месяца, за которую начисляется ФОТ.
// МВ = межвахтовый перерыв (фиксированная выплата 30 000 руб);
// не принят = 0; всё остальное = полный оклад.
func scheduleMultiplier(schedule string, salaryNet float64) float64 {
	switch schedule {
	case ScheduleMV:
		if salaryNet == 0 {
			return 0
		}
		return 30_000.0 / salaryNet
	case ScheduleNotHired:
		return 0
	default: // ОФ, 4/2, 4/4, К, ОТП
		return 1
	}
}

// employeeFOT рассчитывает ФОТ одного сотрудника за один месяц (monthIdx 1-based).
// Формула Excel: BU16*BT16, где BU16=IF(schedule="МВ",30000/salary,IF(schedule="не принят",0,1))
// Результат: МВ→30000, "не принят"→0, всё остальное→полный оклад.
func employeeFOT(emp *Employee, monthIdx int, _ time.Time) float64 {
	if monthIdx < 1 || monthIdx > len(emp.MonthlySchedule) {
		return 0
	}
	sched := emp.MonthlySchedule[monthIdx-1]
	mult := scheduleMultiplier(sched, emp.SalaryNet)
	return mult * emp.SalaryNet
}

// calcFOTMonthly рассчитывает общий ФОТ и налоги по всем сотрудникам за каждый месяц.
// Возвращает массивы длиной durationMonths:
//
//	fotArr    — ФОТ "на руки" (строка 168)
//	ndflArr   — НДФЛ с сотрудников РФ (строка 172)
//	insRFArr  — взносы с сотрудников РФ 30.2% (строка 173)
//	insKGArr  — взносы с сотрудников Киргизии 22.25% (строка 174)
func calcFOTMonthly(
	emps []Employee,
	bonuses *InputBonuses,
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

		// Бонусы (4.1) по стране
		var rfBonus, kgBonus float64
		if bonuses != nil {
			for _, b := range bonuses.Employees {
				var amount float64
				if m <= len(b.MonthlyAmounts) {
					amount = b.MonthlyAmounts[m-1]
				}
				// Премии облагаются по стране сотрудника так же, как оклад
				// (4.1!F11 → SUMIF в 2.Бюджет!H172/H173/H174).
				switch normalizeCountry(b.Country) {
				case CountryRF:
					rfBonus += amount
				case CountryKG:
					kgBonus += amount
				}
			}
		}

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
		ndflArr[m-1] = rfBase/0.85 - rfBase

		// Взносы сотрудников РФ 30.2% от gross.
		// Формула H173: (rfNet + rfBonus + ovRF) / 0.85 × 0.302
		insRFArr[m-1] = rfBase / 0.85 * 0.302

		// Взносы сотрудников Киргизии 22.25% от net (не от gross).
		// Формула H174: (kgNet + kgBonus + ovKG) × 0.2225
		insKGArr[m-1] = kgBase * 0.2225
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
//
// Построчная формула 4.6!E168 (одинакова во всех месяцах):
//
//	=IF(месяц<=D8, IF(график="4/2", 2,
//	     IF(OR(график="К", график=предыдущий, график="не принят"), 0,
//	        COUNTA(график))), 0)
//
// Порядок проверок важен: "4/2" даёт 2 билета даже если график не менялся.
// Для первого месяца «предыдущим» служит поле «Условия» (4.6!C16 = BR16).
// Пустая ячейка: COUNTA = 0 → билета нет.
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
// проекта. Считается принципиально иначе: не суммой построчных билетов, а
// пересчётом всего столбца графиков (Excel: первая ветка 4.6!BL318).
//
//	=COUNTA(график) - билеты_К/2 - COUNTIF(график;"не принят") + COUNTIF(график;"4/2")
//
// Смысл: в последний месяц проекта домой уезжают ВСЕ, кто на объекте,
// поэтому билет получает каждый — даже если график не менялся с прошлого
// месяца (построчная формула в этом случае дала бы 0).
// Слагаемые: COUNTA считает всех с непустым графиком; вычитаются «не принят»
// (их на объекте нет) и «К» (их билеты идут отдельной строкой 779,
// билеты_К/2 = количество «К»); прибавляются «4/2», чтобы у них вышло 2.
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

// calcTickets рассчитывает стоимость авиабилетов по месяцам
// (строка 184 = 4.6!E5 = $C$5*(E318+E779)).
//
// Итог билетов вахтовиков (строка 318) считается по двум разным формулам:
//
//	месяц < D8  → сумма построчных билетов (shiftTicketsRowwise)
//	месяц = D8  → пересчёт столбца графиков (shiftTicketsLastMonth)
//
// Ветку выбирает само условие IF(месяц=$D$8;...) внутри формы.
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
