package calc

import "math"

// Run выполняет полный расчёт бюджета.
func Run(inp *BudgetInputs) *CalcResult {
	n := inp.DurationMonths
	res := &CalcResult{DurationMonths: n, Monthly: make([]MonthlyResult, n)}

	// ── 1. ФОТ ────────────────────────────────────────────────────────────────
	var emps []Employee
	var ticketPrice, perDiemRF, perDiemOther float64 = 40_000, defaultPerDiemRF(), 2_500

	if inp.Employees != nil {
		emps = inp.Employees.Employees
		if inp.Employees.TicketPrice > 0 {
			ticketPrice = inp.Employees.TicketPrice
		}
		if inp.Employees.PerDiemRF > 0 {
			perDiemRF = inp.Employees.PerDiemRF
		}
		if inp.Employees.PerDiemOther > 0 {
			perDiemOther = inp.Employees.PerDiemOther
		}
	}

	fotArr, ndflArr, insRFArr, insKGArr := calcFOTMonthly(
		emps, inp.Bonuses, inp.OvertimeRF, inp.OvertimeKG,
		inp.ProjectStartDate, n,
	)
	ticketsArr := calcTickets(emps, ticketPrice, n)
	perDiemArr := calcPerDiem(emps, perDiemRF, perDiemOther, n)

	// ── 2. Месячные бонусы (4.1) суммарно ────────────────────────────────────
	bonusArr := make([]float64, n)
	if inp.Bonuses != nil {
		for _, b := range inp.Bonuses.Employees {
			for m := 0; m < n && m < len(b.MonthlyAmounts); m++ {
				bonusArr[m] += b.MonthlyAmounts[m]
			}
		}
	}

	// ── 3. Параметры бюджета ──────────────────────────────────────────────────
	var (
		unpredPct, aupPct float64
		otherMode         string
		otherVal          float64
		bgExec, bgWar, bgAdv BankGuarantee
		opMarginPct       float64
		manualRevenue     float64
		contractValue     float64
	)
	if inp.Params != nil {
		unpredPct = inp.Params.UnpredictablesPct / 100
		aupPct = inp.Params.AUPPct / 100
		otherMode = inp.Params.OtherExpenseMode
		otherVal = inp.Params.OtherExpenseValue
		bgExec = inp.Params.BGExecution
		bgWar = inp.Params.BGWarranty
		bgAdv = inp.Params.BGAdvance
		opMarginPct = inp.Params.OpMarginPct / 100
		manualRevenue = inp.Params.ManualRevenue
		contractValue = inp.Params.ContractValue
	}

	// ── 4. Рассчитываем месячные итоги (строки 212, 214-216) ─────────────────
	// Сначала нам нужна общая выручка для "прочих расходов" в режиме "%" —
	// она зависит от стоимости договора (G251), поэтому два прохода.

	// Прочие расходы (строка 218): режим "млн" можно рассчитать сразу.
	otherExpArr := make([]float64, n)
	if otherMode == OtherExpModeMLN && otherVal > 0 && n > 0 {
		perMonth := otherVal * 1_000_000 / float64(n)
		for m := 0; m < n; m++ {
			otherExpArr[m] = perMonth
		}
	}
	// Режим "%" рассчитается после получения contractValue (второй проход).

	// ── 5. Базовые суммы накладных за месяц ───────────────────────────────────
	overheadLines := [34][]float64{
		inp.RentApartments,   // 0 → 178
		inp.Realtor,          // 1 → 179
		inp.TransportRental,  // 2 → 180
		inp.SiteSetup,        // 3 → 181
		inp.OfficeRent,       // 4 → 182
		inp.OfficeCleaning,   // 5 → 183
		ticketsArr,           // 6 → 184 Билеты (авторасчёт)
		perDiemArr,           // 7 → 185 Командировочные (авторасчёт)
		inp.Internet,         // 8 → 186
		inp.Mobile,           // 9 → 187
		inp.LabResearch,      // 10 → 188
		inp.ControlEquipment, // 11 → 189
		inp.Training,         // 12 → 190
		inp.Medical,          // 13 → 191
		inp.Uniform,          // 14 → 192
		inp.Software,         // 15 → 193
		inp.Computers,        // 16 → 194
		inp.Furniture,        // 17 → 195
		inp.OfficeSupplies,   // 18 → 196
		inp.Postal,           // 19 → 197
		inp.Fuel,             // 20 → 198
		inp.TransportServices,// 21 → 199
		inp.SubcontractExt,   // 22 → 200
		inp.SubcontractEmp,   // 23 → 201
		inp.SubcontractGen,   // 24 → 202
		inp.SubcontractOrg,   // 25 → 203
		inp.Representative,   // 26 → 204
		inp.CorporateEvents,  // 27 → 205
		inp.BankServices,     // 28 → 206
		inp.InsuranceLiab,    // 29 → 207
		inp.Utilities,        // 30 → 208
		inp.Security,         // 31 → 209
		inp.GarageRent,       // 32 → 210
		inp.AutoInsurance,    // 33 → 211
	}

	// ── 6. Первый проход: считаем расходы и предварительную выручку ───────────
	type monthData struct {
		fot, projectCosts, totalFOT, unpred, aup, grossCosts float64
	}
	md := make([]monthData, n)

	for m := 0; m < n; m++ {
		d := &md[m]
		d.fot = fotArr[m]

		var projectCosts float64
		for li := 0; li < 34; li++ {
			if li < len(overheadLines) {
				v := lineVal(overheadLines[li], m)
				projectCosts += v
			}
		}
		d.projectCosts = projectCosts

		totalFOT := fotArr[m] + bonusArr[m] +
			arrVal(inp.OvertimeRF, m) + arrVal(inp.OvertimeKG, m) +
			ndflArr[m] + insRFArr[m] + insKGArr[m]
		d.totalFOT = totalFOT

		// Строка 214: непредвиденные
		d.unpred = (projectCosts + totalFOT) * unpredPct

		// Строка 215: АУП (для месяцев <= duration+2)
		if m+1 <= n+2 {
			d.aup = (projectCosts + d.unpred + totalFOT) * aupPct
		}

		// Строка 216: итого (вкл. непредвиденные и АУП)
		d.grossCosts = d.aup + d.unpred + projectCosts + totalFOT
	}

	// ── 7. Итоговая выручка для расчёта BG и "% прочих расходов" ─────────────
	// G232 = SUM(H232) = SUM(grossCosts + otherExp + BGExec + BGWar + BGAdv)
	// но для BG нужна стоимость договора (G251 = contractValue).
	// Если contractValue не задан — равен расчётной выручке (G236).
	// Первый проход: считаем расчётную выручку без BG (greedily from costs+margin).

	var totalGrossCosts float64
	for m := 0; m < n; m++ {
		totalGrossCosts += md[m].grossCosts
	}

	// Предварительная выручка (без BG и прочих расходов в режиме %)
	if manualRevenue > 0 {
		if contractValue == 0 {
			contractValue = manualRevenue
		}
	} else {
		// G232 ≈ totalGrossCosts (без BG). Выручка = costs / (1 - margin%)
		estimatedRevenue := totalGrossCosts / (1 - opMarginPct)
		if contractValue == 0 {
			contractValue = estimatedRevenue
		}
	}

	// Прочие расходы в режиме "%" — теперь можем считать
	if otherMode == OtherExpModePct && otherVal > 0 && n > 0 && contractValue > 0 {
		perMonth := contractValue * otherVal / 100 / float64(n)
		for m := 0; m < n; m++ {
			otherExpArr[m] = perMonth
		}
	}

	// ── 8. Банковские гарантии ────────────────────────────────────────────────
	bgExecMonthly := calcBGMonthly(bgExec, contractValue, n)
	bgWarMonthly := calcBGMonthly(bgWar, contractValue, n)
	bgAdvMonthly := calcBGAdvMonthly(bgAdv, contractValue, n)

	// ── 9. Итого расходов без НДС (строка 232) ────────────────────────────────
	totalCosts := make([]float64, n)
	for m := 0; m < n; m++ {
		totalCosts[m] = bgAdvMonthly[m] + bgWarMonthly[m] + otherExpArr[m] +
			md[m].grossCosts + bgExecMonthly[m]
	}

	// ── 10. Выручка (строка 236) и маржинальность (234) ──────────────────────
	revArr := make([]float64, n)
	marginArr := make([]float64, n)
	opProfitArr := make([]float64, n)

	var totalRevenue float64
	if manualRevenue > 0 {
		// Ручная корректировка: равномерно по проектным месяцам
		for m := 0; m < n; m++ {
			revArr[m] = manualRevenue / float64(n)
		}
	} else {
		for m := 0; m < n; m++ {
			// Операционная маржинальность (строка 234)
			marginArr[m] = totalCosts[m] * opMarginPct
			// Выручка = итого расходы + маржа (строка 236)
			revArr[m] = marginArr[m] + totalCosts[m]
		}
	}
	for m := 0; m < n; m++ {
		totalRevenue += revArr[m]
		// Операционная прибыль (строка 238)
		if manualRevenue == 0 && opMarginPct == 0 {
			opProfitArr[m] = revArr[m] - totalCosts[m]
		}
	}

	// Если G251 не задан, берём итоговую выручку
	if inp.Params == nil || inp.Params.ContractValue == 0 {
		contractValue = totalRevenue
	}

	// ── 11. Выручка с НДС (строка 247) ───────────────────────────────────────
	revWithVATArr := make([]float64, n)
	for m := 0; m < n; m++ {
		switch inp.ExecutorName {
		case ExecutorAibiconProject, ExecutorAibiconKG:
			revWithVATArr[m] = revArr[m] // без НДС
		default: // Айбикон
			revWithVATArr[m] = revArr[m] * 1.22
		}
	}

	// ── 12. Налог (G240) ──────────────────────────────────────────────────────
	var tax float64
	switch inp.ExecutorName {
	case ExecutorAibiconProject:
		tax = 0
	case ExecutorAibiconKG:
		// $G$251/$D$8/100*5 + $G$251*2/$D$8/100 (специальный режим)
		tax = contractValue/float64(n)/100*5*float64(n) +
			contractValue*2/float64(n)/100*float64(n)
		// упрощение: 7% от стоимости договора
		tax = contractValue * 0.07
	default: // Айбикон
		taxRate := 0.25
		if opMarginPct > 0 {
			tax = totalRevenue * opMarginPct * taxRate
		} else {
			// Налог от операционной прибыли
			for m := 0; m < n; m++ {
				tax += opProfitArr[m]
			}
			tax *= taxRate
		}
	}

	// ── 13. Чистая прибыль и рентабельность ──────────────────────────────────
	var totalGrossCostsAll, totalOpProfit float64
	for m := 0; m < n; m++ {
		totalGrossCostsAll += totalCosts[m]
		totalOpProfit += opProfitArr[m]
	}
	var opMarginTotal float64
	for _, v := range marginArr {
		opMarginTotal += v
	}

	netProfit := 0.0
	if opMarginPct > 0 || manualRevenue > 0 {
		netProfit = opMarginTotal - tax
	} else {
		netProfit = totalOpProfit - tax
	}
	if manualRevenue > 0 {
		netProfit = totalRevenue - totalGrossCostsAll - tax
	}

	profitability := 0.0
	if inp.ExecutorName == ExecutorAibiconKG {
		var totalRevWithVAT float64
		for _, v := range revWithVATArr {
			totalRevWithVAT += v
		}
		if totalRevWithVAT != 0 {
			profitability = netProfit / totalRevWithVAT * 100
		}
	} else {
		if totalRevenue != 0 {
			profitability = netProfit / totalRevenue * 100
		}
	}

	// ── 14. Заполняем месячные результаты ────────────────────────────────────
	var totalFOTSum float64
	for m := 0; m < n; m++ {
		mr := &res.Monthly[m]
		mr.Month = m + 1
		mr.FOT = fotArr[m]
		mr.Bonuses = bonusArr[m]
		mr.OvertimeRF = arrVal(inp.OvertimeRF, m)
		mr.OvertimeKG = arrVal(inp.OvertimeKG, m)
		mr.NDFL = ndflArr[m]
		mr.InsuranceRF = insRFArr[m]
		mr.InsuranceKG = insKGArr[m]
		mr.TotalFOT = md[m].totalFOT

		for li := 0; li < 34; li++ {
			mr.Overhead[li] = lineVal(overheadLines[li], m)
		}
		mr.Tickets = lineVal(ticketsArr, m)
		mr.PerDiem = lineVal(perDiemArr, m)

		mr.ProjectCostsExFOT = md[m].projectCosts
		mr.Unpredictables = md[m].unpred
		mr.AUP = md[m].aup
		mr.TotalCostsGross = md[m].grossCosts
		mr.OtherExpenses = otherExpArr[m]
		mr.BGExecution = bgExecMonthly[m]
		mr.BGWarranty = bgWarMonthly[m]
		mr.BGAdvance = bgAdvMonthly[m]
		mr.TotalCosts = totalCosts[m]
		mr.MarginAmount = marginArr[m]
		mr.Revenue = revArr[m]
		mr.OperatingProfit = opProfitArr[m]
		mr.RevenueWithVAT = revWithVATArr[m]

		totalFOTSum += md[m].totalFOT
	}

	// ── 15. Итоговые строки ────────────────────────────────────────────────────
	res.TotalFOT = totalFOTSum
	res.TotalCosts = totalGrossCostsAll
	res.TotalRevenue = totalRevenue
	res.OperatingProfit = totalOpProfit
	res.Tax = tax
	res.NetProfit = netProfit
	res.Profitability = math.Round(profitability*100) / 100
	for _, v := range revWithVATArr {
		res.TotalRevenueWithVAT += v
	}

	return res
}

// calcBGMonthly — банковская гарантия равномерно по месяцам проекта.
// Для БГ на исполнение (222) и гарантийный период (226).
func calcBGMonthly(bg BankGuarantee, contractValue float64, n int) []float64 {
	arr := make([]float64, n)
	if bg.Pct == 0 || contractValue == 0 || n == 0 {
		return arr
	}
	totalBG := contractValue * bg.Pct / 100
	bgRate := totalBG * bg.RatePct / 100

	var bgCost float64
	if bg.RateType == BGRateTotal {
		bgCost = bgRate
	} else { // %/год
		bgCost = bgRate / 12 * bg.DurationMos
	}

	perMonth := bgCost / float64(n)
	for m := 0; m < n; m++ {
		arr[m] = perMonth
	}
	return arr
}

// calcBGAdvMonthly — банковская гарантия на аванс (230).
// Если DurationMos=0 — нет гарантии на аванс.
func calcBGAdvMonthly(bg BankGuarantee, contractValue float64, n int) []float64 {
	arr := make([]float64, n)
	if bg.Pct == 0 || contractValue == 0 || n == 0 {
		return arr
	}
	totalBG := contractValue * bg.Pct / 100
	bgRate := math.Round(totalBG * bg.RatePct / 100)

	var bgCost float64
	if bg.RateType == BGRateTotal {
		bgCost = bgRate
	} else {
		if bg.DurationMos == 0 {
			return arr
		}
		bgCost = bgRate / 12 * bg.DurationMos
	}

	perMonth := bgCost / float64(n)
	for m := 0; m < n; m++ {
		arr[m] = perMonth
	}
	return arr
}

// defaultPerDiemRF — суточные по РФ из Excel: 700 + 300/0.87*1.3
func defaultPerDiemRF() float64 {
	return 700 + 300/0.87*1.3
}

// lineVal возвращает значение из массива по индексу (0-based), 0 если за пределами.
func lineVal(arr []float64, idx int) float64 {
	if idx < len(arr) {
		return arr[idx]
	}
	return 0
}

// arrVal — алиас для lineVal (для читаемости).
func arrVal(arr []float64, idx int) float64 {
	return lineVal(arr, idx)
}
