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
			switch emps[i].Country {
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
				switch b.Country {
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

// calcTickets рассчитывает стоимость авиабилетов по месяцам (строка 184 = 4.6!E5).
// Формула Excel: C5*(E318+E779), где:
//   E318 — кол-во билетов вахтовиков (авто по графику, строки 168-317):
//     "4/2" → 2; "К"/"не принят"/без изменений/пусто → 0; смена графика на
//     непустое значение → 1 (пустая ячейка = COUNTA даёт 0, не "смену")
//   E779 — кол-во билетов в командировках (строки 629+):
//     "К" → 2, иначе 0
func calcTickets(emps []Employee, ticketPrice float64, duration int) []float64 {
	tickets := make([]float64, duration)
	for i := range emps {
		e := &emps[i]
		for m := 1; m <= duration && m <= len(e.MonthlySchedule); m++ {
			cur := e.MonthlySchedule[m-1]

			// Билеты командировочные: "К" → 2 (строка E779 в Excel)
			if cur == ScheduleK {
				tickets[m-1] += 2 * ticketPrice
				continue
			}

			// Билеты вахтовиков: по смене графика (строка E318 в Excel)
			if cur == ScheduleNotHired {
				continue
			}
			var prev string
			if m == 1 {
				prev = e.BaseSchedule
			} else {
				prev = e.MonthlySchedule[m-2]
			}
			var cnt float64
			switch {
			case cur == Schedule42:
				cnt = 2
			case cur == prev:
				cnt = 0
			case cur == "":
				// Excel: COUNTA(пустая ячейка) = 0 — незаполненный график
				// не считается "сменой графика" и не даёт билет
				// (audit/numeric_baseline.md, 4.6!E168).
				cnt = 0
			default:
				cnt = 1
			}
			tickets[m-1] += cnt * ticketPrice
		}
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
