package calc

import "time"

// Константы формулы компенсации при увольнении (источник: 4.1!BO11).
const (
	severanceWorkDays     = 22.0 // рабочих дней в месяце (делитель среднего)
	severanceCalendarDays = 28.0 // календарных дней (множитель)
	severanceMonthsInYear = 12.0
	severanceLastMonthsX  = 2.0 // сколько ФОТ последнего месяца добавляется
)

// effectiveBonusParams возвращает месяц и процент премии с учётом значений
// по умолчанию для стандартных видов. Ноль означает «не задано».
func effectiveBonusParams(bt BonusType) (month int, pct float64) {
	month, pct = bt.MonthNum, bt.PctOfSalary

	switch bt.Kind {
	case BonusKindBuilderDay:
		if month <= 0 || month > 12 {
			month = defaultBuilderDayMonth
		}
		if pct <= 0 {
			pct = defaultBuilderDayPct
		}
	case BonusKindNewYear:
		if month <= 0 || month > 12 {
			month = defaultNewYearMonth
		}
		if pct <= 0 {
			pct = defaultNewYearPct
		}
	}
	return month, pct
}

// bonusDisplayName — название премии для сообщения о конфликте.
func bonusDisplayName(bt BonusType) string {
	if bt.Name != "" {
		return bt.Name
	}
	switch bt.Kind {
	case BonusKindBuilderDay:
		return "День строителя"
	case BonusKindNewYear:
		return "Новый год"
	}
	return "Премия"
}

// employeeName — имя сотрудника для сообщений; если ФИО не заполнено,
// подставляем должность.
func employeeName(e *Employee) string {
	if e.FullName != "" {
		return e.FullName
	}
	if e.Position != "" {
		return e.Position
	}
	return "сотрудник без имени"
}

// severancePay — компенсация при увольнении для одного сотрудника (последний
// месяц проекта). Источник: 4.1!BO11 (Excel).
func severancePay(emp *Employee, startDate time.Time, duration int) float64 {
	if duration <= 0 {
		return 0
	}

	var totalFOT float64
	for m := 1; m <= duration; m++ {
		totalFOT += employeeFOT(emp, m, startDate)
	}
	if totalFOT == 0 {
		return 0 // не работал ни одного месяца
	}

	lastFOT := employeeFOT(emp, duration, startDate)

	return totalFOT/severanceWorkDays*severanceCalendarDays/severanceMonthsInYear +
		severanceLastMonthsX*lastFOT
}

// calcBonuses рассчитывает премии и компенсации при увольнении по месяцам
// (2.Бюджет строка 169 «Премии и компенсации при увольнении»).
func calcBonuses(
	emps []Employee,
	bonuses *InputBonuses,
	startDate time.Time,
	duration int,
) *BonusCalcResult {
	res := &BonusCalcResult{
		Total: make([]float64, duration),
		RF:    make([]float64, duration),
		KG:    make([]float64, duration),
	}
	if duration <= 0 {
		return res
	}

	// add разносит сумму по стране сотрудника — для налогов 172/173/174.
	add := func(monthIdx int, country string, amount float64) {
		if amount == 0 {
			return
		}
		res.Total[monthIdx-1] += amount
		switch normalizeCountry(country) {
		case CountryRF:
			res.RF[monthIdx-1] += amount
		case CountryKG:
			res.KG[monthIdx-1] += amount
		}
		// «Самозанятый, без НО» не попадает ни в одну налоговую базу
	}

	// ── Премии ────────────────────────────────────────────────────────────
	if bonuses != nil && len(bonuses.BonusTypes) > 0 {
		for m := 1; m <= duration; m++ {
			calMonth := int(monthDate(startDate, m).Month())

			// Виды премий, выпадающие на этот календарный месяц
			var due []BonusType
			for _, bt := range bonuses.BonusTypes {
				if month, _ := effectiveBonusParams(bt); month == calMonth {
					due = append(due, bt)
				}
			}
			if len(due) == 0 {
				continue
			}

			for i := range emps {
				e := &emps[i]

				// «Не принят» — единственный случай, когда премии нет
				if sched := scheduleAt(e, m); sched == ScheduleNotHired || sched == "" {
					continue
				}

				salary := e.SalaryNet * aprilIndexation(e, m, startDate)

				var sum float64
				names := make([]string, 0, len(due))
				for _, bt := range due {
					_, pct := effectiveBonusParams(bt)
					sum += salary * pct / 100
					names = append(names, bonusDisplayName(bt))
				}
				add(m, e.Country, sum)

				if len(due) > 1 {
					res.Conflicts = append(res.Conflicts, BonusConflict{
						EmployeeName: employeeName(e),
						MonthIdx:     m,
						BonusNames:   names,
					})
				}
			}
		}
	}

	// ── Компенсация при увольнении (последний месяц) ─────────────────────
	for i := range emps {
		e := &emps[i]
		add(duration, e.Country, severancePay(e, startDate, duration))
	}

	return res
}
