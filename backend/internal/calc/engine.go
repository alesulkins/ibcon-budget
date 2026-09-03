package calc

import "math"

// Run выполняет полный расчёт бюджета.
func Run(inp *BudgetInputs) *CalcResult {
	n := inp.DurationMonths
	res := &CalcResult{DurationMonths: n, Monthly: make([]MonthlyResult, n)}

	// ── 1. ФОТ ────────────────────────────────────────────────────────────────
	var emps []Employee
	// Суточные по РФ — фиксированная формула формы, не пользовательский ввод.
	var ticketPrice, perDiemOther float64 = 40_000, 2_500
	perDiemRF := perDiemRFRate

	if inp.Employees != nil {
		emps = inp.Employees.Employees
		if inp.Employees.TicketPrice > 0 {
			ticketPrice = inp.Employees.TicketPrice
		}
		if inp.Employees.PerDiemOther > 0 {
			perDiemOther = inp.Employees.PerDiemOther
		}
	}

	// Премии и компенсации при увольнении (4.1) — считаются из видов премий
	// проекта и списка сотрудников, а не приходят готовой суммой.
	bonusRes := calcBonuses(emps, inp.Bonuses, inp.ProjectStartDate, n)

	fotArr, ndflArr, insRFArr, insKGArr := calcFOTMonthly(
		emps, bonusRes.RF, bonusRes.KG, inp.OvertimeRF, inp.OvertimeKG,
		inp.ProjectStartDate, n,
	)
	ticketsArr := calcTickets(emps, ticketPrice, n)
	perDiemArr := calcPerDiem(emps, perDiemRF, perDiemOther, n)

	// ── 1a. Лист 4.2: аренда квартир (вкл. уборку) и риелтор ─────────────────
	// Строка 178 = аренда, строка 179 = риелтор.
	rentAptsArr, realtorArr := calcRentApartments(inp.RentApts, inp.ExecutorName, n)

	// ── 1b. Лист 4.3: транспорт и гараж ──────────────────────────────────────
	// Строка 180 = аренда авто + покупка авто (в форме это одна строка),
	// строка 210 = аренда гаража.
	transportArr, garageArr := calcTransport(inp.Transport, n)
	if inp.Transport == nil {
		// Версия бюджета сохранена до перехода 4.3 на расчёт по формуле —
		// берём старые готовые суммы, чтобы её итоги не обнулились.
		// Удалить вместе с TypeTransportRental / TypeGarageRent.
		transportArr, garageArr = inp.TransportRental, inp.GarageRent
	}

	// ── 1в. Лист 4.4: вагончики ───────────────────────────────────────────────
	// Строка 181 = аренда вагончиков + покупка вагончиков одной суммой.
	wagonciksArr := calcWagonciks(inp.Wagonciks, n)
	if inp.Wagonciks == nil {
		// Версия сохранена до перехода 4.4 на расчёт по формуле.
		// Удалить вместе с TypeSiteSetup.
		wagonciksArr = inp.SiteSetup
	}

	// ── 1г. Лист 4.5: офис ────────────────────────────────────────────────────
	// Строка 182 = аренда офиса (с gross-up на НДФЛ, кроме Киргизии),
	// строка 183 = уборка офиса (без gross-up).
	officeRentArr, officeCleaningArr := calcOffice(inp.Office, inp.ExecutorName, n)
	if inp.Office == nil {
		// Версия сохранена до перехода 4.5 на расчёт по формуле.
		// Удалить вместе с TypeOfficeRent / TypeOfficeCleaning.
		officeRentArr, officeCleaningArr = inp.OfficeRent, inp.OfficeCleaning
	}

	// ── 1д. Листы-списки 4.8, 4.9, 4.11 ──────────────────────────────────────
	// Устроены одинаково: итог месяца = сумма стоимостей всех позиций.
	//   4.8  → строка 193 приобретение ПО
	//   4.9  → строка 200 ГПХ внешний
	//   4.11 → строка 202 субподряд
	// Если списка позиций нет, версия сохранена до перехода на него — берём
	// старые готовые суммы, чтобы её итоги не обнулились. Удалить вместе с
	// TypeSoftware / TypeSubcontractExt / TypeSubcontractGen.
	softwareArr := calcCostLines(inp.SoftwareLines, n)
	if inp.SoftwareLines == nil {
		softwareArr = inp.Software
	}
	subExtArr := calcCostLines(inp.SubcontractExtLines, n)
	if inp.SubcontractExtLines == nil {
		subExtArr = inp.SubcontractExt
	}
	subGenArr := calcCostLines(inp.SubcontractGenLines, n)
	if inp.SubcontractGenLines == nil {
		subGenArr = inp.SubcontractGen
	}

	// ── 1е. Листы-покупки 4.7 и 4.12 ─────────────────────────────────────────
	// Расход месяца = Σ цена × количество по строкам этого месяца.
	//   4.7  → строка 189 приборы стройконтроля
	//   4.12 → строка 205 корпоративные мероприятия
	// Ограничения на последние месяцы проекта, как у вагончиков, здесь нет.
	equipmentArr := calcPurchases(inp.EquipmentItems, n)
	if inp.EquipmentItems == nil {
		equipmentArr = inp.ControlEquipment // версия до перехода на таблицу
	}
	corporateArr := calcPurchases(inp.CorporateEventItems, n)
	if inp.CorporateEventItems == nil {
		corporateArr = inp.CorporateEvents
	}

	// ── 1ж. Лист 4.10: ГПХ сотрудников ───────────────────────────────────────
	// Строка 201 = среднее количество × средняя стоимость, одинаково во всех
	// месяцах проекта.
	gphArr := calcGphEmployees(inp.GphEmployees, n)
	if inp.GphEmployees == nil {
		gphArr = inp.SubcontractEmp // версия до перехода на два поля
	}

	// ── 2. Премии и компенсации (4.1) суммарно → 2.Бюджет строка 169 ────────
	bonusArr := bonusRes.Total

	// ── 3. Параметры бюджета ──────────────────────────────────────────────────
	var (
		unpredPct, aupPct    float64
		otherMode            string
		otherVal             float64
		bgExec, bgWar, bgAdv BankGuarantee
		markup               float64
		manualRevenue        float64
		contractValue        float64
		// contractTKP — ИСХОДНАЯ стоимость договора (2.Бюджет!G251), без
		// подстановки расчётной выручки.
		contractTKP float64
	)
	if inp.Params != nil {
		unpredPct = inp.Params.UnpredictablesPct / 100
		aupPct = inp.Params.AUPPct / 100
		otherMode = inp.Params.OtherExpenseMode
		otherVal = inp.Params.OtherExpenseValue
		bgExec = inp.Params.BGExecution
		bgWar = inp.Params.BGWarranty
		bgAdv = inp.Params.BGAdvance
		// Коэффициент наценки на расходы (2.Бюджет!F234) считается из
		// целевой рентабельности и ставки налога, а не вводится напрямую.
		markup = markupRateAt(inp.Params.TargetRentPct,
			effectiveTaxRate(inp.Params, inp.ExecutorName))
		contractValue = inp.Params.ContractValue
		contractTKP = inp.Params.ContractValue
		// Ручная стоимость работ (2.Бюджет!F236) — не самостоятельный ввод,
		// а ТКП без НДС: F236 = G252 = IF(КГ, G251, IF(АП, G251, G251/1.22)).
		manualRevenue = contractNetOfVAT(contractTKP, inp.ExecutorName)
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
		rentAptsArr,           // 0 → 178 Аренда квартир, вкл. уборку (4.2, авторасчёт)
		realtorArr,            // 1 → 179 Риелтор (4.2, авторасчёт)
		transportArr,          // 2 → 180 Аренда транспорта + покупка авто (4.3, авторасчёт)
		wagonciksArr,          // 3 → 181 Вагончики: аренда + покупка (4.4, авторасчёт)
		officeRentArr,         // 4 → 182 Аренда офиса (4.5, авторасчёт)
		officeCleaningArr,     // 5 → 183 Уборка офиса (4.5, авторасчёт)
		ticketsArr,            // 6 → 184 Билеты (авторасчёт)
		perDiemArr,            // 7 → 185 Командировочные (авторасчёт)
		inp.Internet,          // 8 → 186
		inp.Mobile,            // 9 → 187
		inp.LabResearch,       // 10 → 188
		equipmentArr,          // 11 → 189 Приборы стройконтроля (4.7, таблица покупок)
		inp.Training,          // 12 → 190
		inp.Medical,           // 13 → 191
		inp.Uniform,           // 14 → 192
		softwareArr,           // 15 → 193 Приобретение ПО (4.8, список позиций)
		inp.Computers,         // 16 → 194
		inp.Furniture,         // 17 → 195
		inp.OfficeSupplies,    // 18 → 196
		inp.Postal,            // 19 → 197
		inp.Fuel,              // 20 → 198
		inp.TransportServices, // 21 → 199
		subExtArr,             // 22 → 200 ГПХ внешний (4.9, список позиций)
		gphArr,                // 23 → 201 ГПХ сотрудников (4.10, два поля)
		subGenArr,             // 24 → 202 Субподряд (4.11, список позиций)
		inp.SubcontractOrg,    // 25 → 203
		inp.Representative,    // 26 → 204
		corporateArr,          // 27 → 205 Корпоративы (4.12, таблица покупок)
		inp.BankServices,      // 28 → 206
		inp.InsuranceLiab,     // 29 → 207
		inp.Utilities,         // 30 → 208
		inp.Security,          // 31 → 209
		garageArr,             // 32 → 210 Аренда гаража (4.3, авторасчёт)
		inp.AutoInsurance,     // 33 → 211
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

		// Строка 215: АУП. Формула H215 = IF(H11 <= $D$8+2,
		// (H212+H214+H176)*$F$215, 0).
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
		// Выручка в режиме наценки: расходы + наценка на них
		// (2.Бюджет!H236 = H234 + H232, где H234 = H232 × F234).
		estimatedRevenue := totalGrossCosts * (1 + markup)
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

	// ── 8. Банковские гарантии (строки 222, 226, 230) ────────────────────────
	// База для БГ — строго ТКП (стоимость договора), а НЕ расчётная выручка.
	// В форме: G220 = G251*F220, G224 = G251*F224, G228 = F228*G251, и уже
	// от них считаются G222 / G226 / G230. Если ТКП не задан (режим наценки),
	// все эти произведения равны нулю — гарантию не от чего считать, договора
	// ещё нет. Поэтому берём исходный ContractValue, а не подставленное выше
	// значение: подстановка расчётной выручки нужна только для «прочих
	// расходов» в режиме «%» и на БГ распространяться не должна.
	bgExecMonthly := calcBGMonthly(bgExec, contractTKP, n)
	bgWarMonthly := calcBGMonthly(bgWar, contractTKP, n)
	bgAdvMonthly := calcBGAdvMonthly(bgAdv, contractTKP, n)

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
			// Наценка на расходы (строка 234): H234 = H232 × F234
			marginArr[m] = totalCosts[m] * markup
			// Выручка = итого расходы + маржа (строка 236)
			revArr[m] = marginArr[m] + totalCosts[m]
		}
	}
	// Операционная прибыль (строка 238).
	var totalOpProfit, opMarginTotal float64
	for m := 0; m < n; m++ {
		totalRevenue += revArr[m]
		if marginArr[m] == 0 {
			opProfitArr[m] = revArr[m] - totalCosts[m]
		}
		totalOpProfit += opProfitArr[m]
		opMarginTotal += marginArr[m]
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
			revWithVATArr[m] = revArr[m] * vatMultiplier
		}
	}

	// ── 12. Налог на прибыль (строка 240) ────────────────────────────────────
	// Формула Excel G240:
	//
	//	=IF($D$10="Айбикон-Проект"; 0;
	//	 IF($D$10="Айбикон Киргизия"; $G$251/$D$8/100*5 + $G$251*2/$D$8/100;
	//	 IF(G234<>0; G234*$F$240; G238*$F$240)))
	//
	// То есть: «Айбикон-Проект» освобождён; у «Айбикон Киргизия» спецрежим
	// от стоимости договора; у остальных — ставка F240 либо от маржи
	// (если она задана), либо от операционной прибыли.
	var tax float64
	switch {
	case sameExecutor(inp.ExecutorName, ExecutorAibiconProject):
		tax = 0

	case sameExecutor(inp.ExecutorName, ExecutorAibiconKG):
		// База — строго ТКП (2.Бюджет!$G$251).
		if n > 0 {
			if rate, ok := inp.Params.TaxRate(); ok {
				tax = contractTKP * rate
			} else {
				tax = contractTKP/float64(n)/100*5 + contractTKP*2/float64(n)/100
			}
		}

	default: // Айбикон
		rate := effectiveTaxRate(inp.Params, inp.ExecutorName)
		if opMarginTotal != 0 {
			tax = opMarginTotal * rate
		} else {
			tax = totalOpProfit * rate
		}
	}

	// ── 13. Чистая прибыль и рентабельность ──────────────────────────────────
	var totalGrossCostsAll float64
	for m := 0; m < n; m++ {
		totalGrossCostsAll += totalCosts[m]
	}

	// Чистая прибыль (строка 242).
	// Формула Excel G242 = IF(G234<>0; G234-G240; G238-G240):
	// если маржа задана — от неё, иначе — от операционной прибыли.
	netProfit := totalOpProfit - tax
	if opMarginTotal != 0 {
		netProfit = opMarginTotal - tax
	}

	profitability := 0.0
	if sameExecutor(inp.ExecutorName, ExecutorAibiconKG) {
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

	// ── 13а. Стоимость + ставка рефинансирования (строка 249) ────────────────
	refRatePct := inp.Params.RefRate()
	refRateArr := calcRefRate(revWithVATArr, refRatePct)

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
		mr.RefRateAmount = refRateArr[m]

		totalFOTSum += md[m].totalFOT
	}

	// ── 15. Итоговые строки ────────────────────────────────────────────────────
	res.TotalFOT = totalFOTSum
	res.TotalCosts = totalGrossCostsAll
	res.TotalRevenue = totalRevenue
	res.OperatingProfit = totalOpProfit
	res.OperatingMargin = opMarginTotal
	res.Tax = tax
	res.NetProfit = netProfit
	res.Profitability = math.Round(profitability*100) / 100
	for _, v := range revWithVATArr {
		res.TotalRevenueWithVAT += v
	}
	res.RefRatePct = refRatePct
	// Ставка налога, которая реально применена. Нужна отчётам: в БДР и
	// БДДС налог на прибыль платится поквартально и считается там заново,
	// от операционной прибыли квартала.
	res.ProfitTaxRate = effectiveTaxRate(inp.Params, inp.ExecutorName)
	for _, v := range refRateArr {
		res.RefRateAmount += v
	}

	return res
}

// calcRefRate — «Стоимость + ставка рефинансирования на 1–4 месяцы»
// (2.Бюджет!249). Формула строки: `выручка с НДС × ОКРУГЛ(ставка/12; 2)`.
func calcRefRate(revenueWithVAT []float64, ratePct float64) []float64 {
	arr := make([]float64, len(revenueWithVAT))
	mult := math.Round(ratePct/12*100) / 100
	for m := 0; m < len(arr) && m < refRateMonths; m++ {
		arr[m] = revenueWithVAT[m] * mult
	}
	return arr
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

// markupRate — коэффициент наценки на расходы при целевой рентабельности.
const vatMultiplier = 1.22

// contractNetOfVAT приводит ТКП к сумме без НДС — формула 2.Бюджет!G252:
//
//	=IF(D10="Айбикон Киргизия", G251, IF(D10="Айбикон-Проект", G251, G251/1.22))
//
// У «Айбикон» ТКП задаётся С НДС (подпись F251 «В ТКП с НДС»), у остальных
// двух исполнителей — уже без НДС, поэтому делить не нужно.
func contractNetOfVAT(tkp float64, executor string) float64 {
	if tkp == 0 {
		return 0
	}
	switch {
	case sameExecutor(executor, ExecutorAibiconKG),
		sameExecutor(executor, ExecutorAibiconProject):
		return tkp
	default: // Айбикон
		return tkp / vatMultiplier
	}
}

func markupRate(targetRentPct float64, executor string) float64 {
	return markupRateAt(targetRentPct, profitTaxRate(executor))
}

// markupRateAt — та же наценка, но по явно заданной ставке налога:
// справочное значение исполнителя, подставленное в форму, может
// отличаться от ставки эталонной формы.
func markupRateAt(targetRentPct, taxRate float64) float64 {
	r := targetRentPct / 100
	if r <= 0 {
		return 0
	}
	denom := 1 - taxRate - r
	if denom <= 0 {
		return 0
	}
	return r / denom
}

// effectiveTaxRate — ставка налога на прибыль, которая реально идёт в
// расчёт: справочное значение из формы, если оно там есть, иначе ставка
// эталонной формы.
func effectiveTaxRate(p *InputBudgetParams, executor string) float64 {
	if rate, ok := p.TaxRate(); ok {
		return rate
	}
	return profitTaxRate(executor)
}

// profitTaxRate — ставка налога на прибыль по исполнителю, когда в форме
// бюджета своей ставки нет (версия сохранена до появления справочных
// значений).
func profitTaxRate(executor string) float64 {
	switch {
	case sameExecutor(executor, ExecutorAibiconProject):
		return 0
	case sameExecutor(executor, ExecutorAibiconKG):
		return 0.04
	default: // Айбикон
		return 0.25
	}
}

// perDiemRFRate — суточные командировочные по РФ, ₽/день. Источник: 4.6!D9 =
// `=700+300/0.87*1.3` ≈ 1148.28.
const perDiemRFRate = 700 + 300/npflGrossUpDivisor*1.3

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
