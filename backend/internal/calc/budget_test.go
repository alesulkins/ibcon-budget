package calc

import (
	"math"
	"testing"
	"time"
)

func TestRun_SimpleOneMonth(t *testing.T) {
	// 1 сотрудник РФ, 1 месяц. Накладных нет. Маржа 20%.
	inp := &BudgetInputs{
		ProjectStartDate: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		DurationMonths:   1,
		ExecutorName:     ExecutorAibicon,
		Employees: &InputEmployees{
			TicketPrice: 40_000,
			Employees: []Employee{
				{
					Country:         CountryRF,
					BaseSchedule:    ScheduleOF,
					SalaryNet:       100_000,
					MonthlySchedule: []string{ScheduleOF},
				},
			},
		},
		Params: &InputBudgetParams{
			UnpredictablesPct: 7,
			AUPPct:            15,
			TargetRentPct:     20,
		},
	}

	res := Run(inp)

	if res.DurationMonths != 1 {
		t.Errorf("DurationMonths: want 1, got %d", res.DurationMonths)
	}
	if len(res.Monthly) != 1 {
		t.Fatalf("Monthly length: want 1, got %d", len(res.Monthly))
	}

	mr := res.Monthly[0]

	// ФОТ = 100 000
	if math.Abs(mr.FOT-100_000) > 0.01 {
		t.Errorf("FOT: want 100000, got %.2f", mr.FOT)
	}

	// Компенсация при увольнении (4.1!BO11) начисляется в последнем месяце
	// проекта, а здесь проект длиной 1 месяц — значит месяц 1 и есть
	// последний. Формула: итогоФОТ×28/264 + 2×ФОТ последнего месяца.
	wantSeverance := 100_000.0*28/264 + 2*100_000.0 // ≈ 210 606.06
	if math.Abs(mr.Bonuses-wantSeverance) > 0.01 {
		t.Errorf("Bonuses (компенсация): want %.2f, got %.2f", wantSeverance, mr.Bonuses)
	}

	// ФОТ вкл. взносы: база налогов = оклад + компенсация
	wantTaxBase := 100_000.0 + wantSeverance
	wantNDFL := wantTaxBase/0.85 - wantTaxBase
	wantInsRF := wantTaxBase / 0.85 * 0.302
	wantTotalFOT := 100_000 + wantSeverance + wantNDFL + wantInsRF
	if math.Abs(mr.TotalFOT-wantTotalFOT) > 0.1 {
		t.Errorf("TotalFOT: want %.2f, got %.2f", wantTotalFOT, mr.TotalFOT)
	}

	// Билеты: проект длиной 1 месяц — этот месяц одновременно последний,
	// поэтому работает СЧЁТЗ-ветка 4.6!318 («в последний месяц уезжают все»):
	// сотрудник на объекте (ОФ) получает 1 билет × 40 000.
	wantTickets := 40_000.0
	if math.Abs(mr.Tickets-wantTickets) > 0.01 {
		t.Errorf("Tickets: want %.2f, got %.2f", wantTickets, mr.Tickets)
	}

	// Непредвиденные = (projectCosts + TotalFOT) × 0.07
	wantUnpred := (wantTickets + wantTotalFOT) * 0.07
	if math.Abs(mr.Unpredictables-wantUnpred) > 0.1 {
		t.Errorf("Unpredictables: want %.2f, got %.2f", wantUnpred, mr.Unpredictables)
	}

	// АУП = (projectCosts + непред + TotalFOT) × 0.15
	wantAUP := (wantTickets + wantTotalFOT + wantUnpred) * 0.15
	if math.Abs(mr.AUP-wantAUP) > 0.1 {
		t.Errorf("AUP: want %.2f, got %.2f", wantAUP, mr.AUP)
	}

	// TotalCostsGross = AUP + непред + projectCosts + TotalFOT
	wantGross := wantAUP + wantUnpred + wantTickets + wantTotalFOT
	if math.Abs(mr.TotalCostsGross-wantGross) > 0.1 {
		t.Errorf("TotalCostsGross: want %.2f, got %.2f", wantGross, mr.TotalCostsGross)
	}

	// Revenue = TotalCosts × (1 + наценка).
	// Целевая рентабельность 20% при ставке налога «Айбикон» 25%:
	// markup = 0.20/(1-0.25-0.20) = 0.363636… (2.Бюджет!F234)
	wantMarkup := 0.20 / (1 - 0.25 - 0.20)
	wantRevenue := wantGross * (1 + wantMarkup)
	if math.Abs(mr.Revenue-wantRevenue) > 1 {
		t.Errorf("Revenue: want %.2f, got %.2f", wantRevenue, mr.Revenue)
	}

	// Выручка с НДС = Revenue × 1.22 (Айбикон)
	wantRevVAT := wantRevenue * 1.22
	if math.Abs(mr.RevenueWithVAT-wantRevVAT) > 1 {
		t.Errorf("RevenueWithVAT: want %.2f, got %.2f", wantRevVAT, mr.RevenueWithVAT)
	}
}

// TestRun_RevenueFromTKP — стоимость работ (2.Бюджет!F236) выводится из ТКП,
// а не вводится отдельно: F236 = G252 = IF(КГ, G251, IF(АП, G251, G251/1.22)).
// У «Айбикон» ТКП задаётся С НДС, поэтому делится на 1.22.
func TestRun_RevenueFromTKP(t *testing.T) {
	tests := []struct {
		executor    string
		tkp         float64
		wantRevenue float64
	}{
		// Айбикон: ТКП с НДС → выручка без НДС = 1 220 000 / 1.22
		{ExecutorAibicon, 1_220_000, 1_000_000},
		// Айбикон-Проект и Киргизия: ТКП уже без НДС, деления нет
		{ExecutorAibiconProject, 1_000_000, 1_000_000},
		{ExecutorAibiconKG, 1_000_000, 1_000_000},
	}

	for _, tt := range tests {
		inp := &BudgetInputs{
			ProjectStartDate: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
			DurationMonths:   2,
			ExecutorName:     tt.executor,
			Params: &InputBudgetParams{
				UnpredictablesPct: 0,
				AUPPct:            0,
				ContractValue:     tt.tkp,
			},
		}
		res := Run(inp)

		// Выручка распределяется равномерно по месяцам (H236 = F236/D8)
		perMonth := tt.wantRevenue / 2
		for i, mr := range res.Monthly {
			if math.Abs(mr.Revenue-perMonth) > 0.01 {
				t.Errorf("%s, месяц %d: выручка want %.2f, got %.2f",
					tt.executor, i+1, perMonth, mr.Revenue)
			}
		}
		if math.Abs(res.TotalRevenue-tt.wantRevenue) > 0.01 {
			t.Errorf("%s: итого выручка want %.2f, got %.2f",
				tt.executor, tt.wantRevenue, res.TotalRevenue)
		}
	}
}

// TestRun_NoTKPNoManualRevenue — без ТКП режим ручной выручки не включается,
// работает наценка. Проверяем, что выручка НЕ нулевая и не равна ТКП.
func TestRun_NoTKPNoManualRevenue(t *testing.T) {
	inp := &BudgetInputs{
		ProjectStartDate: mustDate(2026, 12, 1),
		DurationMonths:   2,
		ExecutorName:     ExecutorAibicon,
		Internet:         []float64{1_000_000, 1_000_000},
		Params: &InputBudgetParams{
			TargetRentPct: 29,
			// ContractValue не задан → режим наценки
		},
	}
	res := Run(inp)
	if res.TotalRevenue <= 2_000_000 {
		t.Errorf("в режиме наценки выручка должна превышать расходы 2 млн, got %.2f",
			res.TotalRevenue)
	}
	if math.Abs(res.Profitability-29) > 0.01 {
		t.Errorf("рентабельность должна выйти целевой 29%%, got %.4f", res.Profitability)
	}
}

func TestRun_BankGuaranteeExecution(t *testing.T) {
	// БГ на исполнение: 10% от договора, ставка 3%/год, срок 12 мес.
	// Договор = 10 000 000. БГ сумма = 1 000 000. Стоимость = 1 000 000 × 3%/12 × 12 = 30 000.
	// По месяцам: 30 000 / 3 = 10 000/мес (проект 3 месяца).
	inp := &BudgetInputs{
		ProjectStartDate: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		DurationMonths:   3,
		ExecutorName:     ExecutorAibicon,
		Params: &InputBudgetParams{
			ContractValue: 10_000_000,
			BGExecution: BankGuarantee{
				Pct:         10,
				RatePct:     3,
				RateType:    BGRatePerYear,
				DurationMos: 12,
			},
		},
	}

	res := Run(inp)
	// BG = 10 000 000 × 10% × 3% / 12 × 12 = 30 000; за 3 месяца = 10 000/мес
	wantMonthly := 30_000.0 / 3
	for i, mr := range res.Monthly {
		if math.Abs(mr.BGExecution-wantMonthly) > 0.01 {
			t.Errorf("month %d BGExecution: want %.2f, got %.2f", i+1, wantMonthly, mr.BGExecution)
		}
	}
}

func TestRun_BankGuaranteeTotalRate(t *testing.T) {
	// БГ на аванс: 15% от договора, ставка 2% за весь срок.
	// Договор = 5 000 000. БГ = 750 000. Стоимость = round(750 000 × 2%) = 15 000.
	// За 2 месяца = 7 500/мес.
	inp := &BudgetInputs{
		ProjectStartDate: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		DurationMonths:   2,
		ExecutorName:     ExecutorAibicon,
		Params: &InputBudgetParams{
			ContractValue: 5_000_000,
			BGAdvance: BankGuarantee{
				Pct:      15,
				RatePct:  2,
				RateType: BGRateTotal,
			},
		},
	}

	res := Run(inp)
	// ROUND(5 000 000 × 15% × 2%) = ROUND(15 000) = 15 000; за 2 мес = 7 500/мес
	wantMonthly := 15_000.0 / 2
	for i, mr := range res.Monthly {
		if math.Abs(mr.BGAdvance-wantMonthly) > 0.01 {
			t.Errorf("month %d BGAdvance: want %.2f, got %.2f", i+1, wantMonthly, mr.BGAdvance)
		}
	}
}

func TestRun_AibiconProject_NoTax(t *testing.T) {
	// Айбикон-Проект освобождён от налога
	inp := &BudgetInputs{
		ProjectStartDate: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		DurationMonths:   1,
		ExecutorName:     ExecutorAibiconProject,
		Params:           &InputBudgetParams{TargetRentPct: 10},
	}
	res := Run(inp)
	if res.Tax != 0 {
		t.Errorf("AibiconProject tax should be 0, got %.2f", res.Tax)
	}
}

func TestRun_OtherExpensesMln(t *testing.T) {
	// Прочие расходы 2 млн, проект 4 месяца = 500 000/мес
	inp := &BudgetInputs{
		ProjectStartDate: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		DurationMonths:   4,
		ExecutorName:     ExecutorAibicon,
		Params: &InputBudgetParams{
			OtherExpenseMode:  OtherExpModeMLN,
			OtherExpenseValue: 2,
		},
	}
	res := Run(inp)
	for i, mr := range res.Monthly {
		if math.Abs(mr.OtherExpenses-500_000) > 0.01 {
			t.Errorf("month %d OtherExpenses: want 500000, got %.2f", i+1, mr.OtherExpenses)
		}
	}
}

func TestRun_VATByExecutor(t *testing.T) {
	// Айбикон: VAT = ×1.22; Айбикон-Проект = без НДС; КГ = без НДС
	tests := []struct {
		executor string
		wantMult float64
	}{
		{ExecutorAibicon, 1.22},
		{ExecutorAibiconProject, 1.0},
		{ExecutorAibiconKG, 1.0},
	}
	for _, tt := range tests {
		// ТКП подбираем так, чтобы выручка без НДС вышла 1 000 000
		// у всех трёх исполнителей (у «Айбикон» ТКП задаётся с НДС).
		tkp := 1_000_000.0
		if tt.executor == ExecutorAibicon {
			tkp = 1_220_000
		}
		inp := &BudgetInputs{
			ProjectStartDate: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			DurationMonths:   1,
			ExecutorName:     tt.executor,
			Params:           &InputBudgetParams{ContractValue: tkp},
		}
		res := Run(inp)
		wantVAT := 1_000_000 * tt.wantMult
		if math.Abs(res.TotalRevenueWithVAT-wantVAT) > 0.01 {
			t.Errorf("%s: RevenueWithVAT want %.2f, got %.2f", tt.executor, wantVAT, res.TotalRevenueWithVAT)
		}
	}
}

// TestRun_OperatingProfitWithManualRevenue — операционная прибыль (строка 238)
// должна считаться и в режиме ручной выручки.
//
// Формула Excel H238 = IF(H11<=$D$8; IF(H234<>0; 0; H236-H232); 0).
// При ручной выручке форма обнуляет маржу H234, поэтому работает вторая
// ветка: прибыль = выручка − расходы. Раньше Go в этом режиме давал 0
// (audit/numeric_final.md, ошибка 1).
func TestRun_OperatingProfitWithManualRevenue(t *testing.T) {
	inp := &BudgetInputs{
		ProjectStartDate: mustDate(2026, 12, 1),
		DurationMonths:   2,
		ExecutorName:     ExecutorAibicon,
		Internet:         []float64{1_000_000, 1_000_000}, // расходы 1 млн/мес
		Params: &InputBudgetParams{
			// ТКП с НДС 12.2 млн → выручка без НДС ровно 10 млн
			ContractValue: 12_200_000,
			TargetRentPct: 25, // задана, но при заданном ТКП не применяется
		},
	}
	res := Run(inp)

	for m := 0; m < 2; m++ {
		// маржа обнулена (H234)
		if res.Monthly[m].MarginAmount != 0 {
			t.Errorf("месяц %d: маржа при ручной выручке должна быть 0, got %.2f",
				m+1, res.Monthly[m].MarginAmount)
		}
		// прибыль = выручка − расходы = 5 000 000 − 1 000 000
		want := 4_000_000.0
		if got := res.Monthly[m].OperatingProfit; math.Abs(got-want) > 0.01 {
			t.Errorf("месяц %d: операционная прибыль want %.2f, got %.2f", m+1, want, got)
		}
	}
	if math.Abs(res.OperatingProfit-8_000_000) > 0.01 {
		t.Errorf("итого операционная прибыль: want 8000000, got %.2f", res.OperatingProfit)
	}
}

// TestRun_TaxByExecutor — налог на прибыль (строка 240) по всем трём ветвям
// формулы Excel G240:
//
//	=IF($D$10="Айбикон-Проект"; 0;
//	 IF($D$10="Айбикон Киргизия"; $G$251/$D$8/100*5 + $G$251*2/$D$8/100;
//	 IF(G234<>0; G234*$F$240; G238*$F$240)))
func TestRun_TaxByExecutor(t *testing.T) {
	mk := func(executor string) *BudgetInputs {
		return &BudgetInputs{
			ProjectStartDate: mustDate(2026, 12, 1),
			DurationMonths:   6,
			ExecutorName:     executor,
			Internet:         []float64{1_000_000, 1_000_000, 1_000_000, 1_000_000, 1_000_000, 1_000_000},
			Params: &InputBudgetParams{
				ContractValue: 200_000_000,
			},
		}
	}

	// Айбикон: налог = операционная прибыль × 25%.
	// Выручка = ТКП без НДС = 200 млн / 1.22, расходы 6 млн.
	res := Run(mk(ExecutorAibicon))
	wantProfit := 200_000_000.0/vatMultiplier - 6_000_000.0
	if math.Abs(res.OperatingProfit-wantProfit) > 0.01 {
		t.Fatalf("Айбикон: опер.прибыль want %.2f, got %.2f", wantProfit, res.OperatingProfit)
	}
	if want := wantProfit * 0.25; math.Abs(res.Tax-want) > 0.01 {
		t.Errorf("Айбикон: налог want %.2f, got %.2f", want, res.Tax)
	}

	// Айбикон-Проект: освобождён
	if res := Run(mk(ExecutorAibiconProject)); res.Tax != 0 {
		t.Errorf("Айбикон-Проект: налог должен быть 0, got %.2f", res.Tax)
	}

	// Айбикон Киргизия: спецрежим от стоимости договора, эталон Excel
	// G240 = 200000000/6/100*5 + 200000000*2/6/100 = 2 333 333.33
	// Значение НЕ зависит от расходов и прибыли — только от ТКП и срока.
	if res := Run(mk(ExecutorAibiconKG)); math.Abs(res.Tax-2_333_333.3333333335) > 0.01 {
		t.Errorf("Киргизия: налог want 2333333.33 (эталон Excel), got %.2f", res.Tax)
	}

	// Регистр исполнителя не должен ломать ветвление
	kgLower := mk(ExecutorAibiconKG)
	kgLower.ExecutorName = "айбикон киргизия"
	if res := Run(kgLower); math.Abs(res.Tax-2_333_333.3333333335) > 0.01 {
		t.Errorf("Киргизия (нижний регистр): налог want 2333333.33, got %.2f", res.Tax)
	}
}

// TestRun_TaxFromMarginWhenSet — если маржа задана (ручной выручки нет),
// налог считается от неё, а не от операционной прибыли (первая ветка
// внутреннего IF в G240).
func TestRun_TaxFromMarginWhenSet(t *testing.T) {
	inp := &BudgetInputs{
		ProjectStartDate: mustDate(2026, 12, 1),
		DurationMonths:   2,
		ExecutorName:     ExecutorAibicon,
		Internet:         []float64{1_000_000, 1_000_000},
		Params:           &InputBudgetParams{TargetRentPct: 20}, // ручной выручки нет
	}
	res := Run(inp)

	// Наценка считается из целевой рентабельности и ставки налога
	// (2.Бюджет!F234 = E234/(1-F240-E234)): 20% при налоге 25% → 0.363636…
	markup := 20.0 / 100 / (1 - 0.25 - 20.0/100)
	wantMargin := 2_000_000.0 * markup
	var gotMargin float64
	for _, m := range res.Monthly {
		gotMargin += m.MarginAmount
	}
	if math.Abs(gotMargin-wantMargin) > 0.01 {
		t.Fatalf("итого маржа: want %.2f, got %.2f", wantMargin, gotMargin)
	}
	// налог от маржи: маржа × 25%
	if want := wantMargin * 0.25; math.Abs(res.Tax-want) > 0.01 {
		t.Errorf("налог от маржи: want %.2f, got %.2f", want, res.Tax)
	}

	// Ключевая проверка смысла формулы: после уплаты налога итоговая
	// рентабельность должна выйти ровно целевой (20%).
	if math.Abs(res.Profitability-20) > 0.01 {
		t.Errorf("итоговая рентабельность: want 20%%, got %.4f%%", res.Profitability)
	}
	// при заданной марже операционная прибыль не показывается (H238 → 0)
	if res.OperatingProfit != 0 {
		t.Errorf("при заданной марже опер.прибыль должна быть 0, got %.2f", res.OperatingProfit)
	}
}

// TestProfitTaxRate — ставка налога по исполнителю (2.Бюджет!F240).
func TestProfitTaxRate(t *testing.T) {
	tests := []struct {
		executor string
		want     float64
	}{
		{ExecutorAibicon, 0.25},
		{ExecutorAibiconProject, 0},
		{ExecutorAibiconKG, 0.06}, // в форме есть, но для КГ не применяется
		{"айбикон-проект", 0},     // регистронезависимо
		{"АЙБИКОН КИРГИЗИЯ", 0.06},
	}
	for _, tt := range tests {
		if got := profitTaxRate(tt.executor); math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("profitTaxRate(%q): want %.2f, got %.2f", tt.executor, tt.want, got)
		}
	}
}

// TestMarkupRate — коэффициент наценки на расходы.
// Источник: 2.Бюджет!F234 = E234/(1-F240-E234), где E234 — целевая
// рентабельность без налога на прибыль, F240 — ставка налога исполнителя.
func TestMarkupRate(t *testing.T) {
	tests := []struct {
		name       string
		targetRent float64
		executor   string
		want       float64
	}{
		// Айбикон, налог 25%: 0.29/(1-0.25-0.29) = 0.29/0.46
		{"Айбикон 29%", 29, ExecutorAibicon, 0.6304347826086957},
		// Айбикон-Проект, налог 0: 0.70/(1-0-0.70) = 0.70/0.30
		{"Проект 70%", 70, ExecutorAibiconProject, 2.3333333333333335},
		// Айбикон-Проект, налог 0: 0.20/(1-0-0.20) = 0.20/0.80
		{"Проект 20%", 20, ExecutorAibiconProject, 0.25},
		// Киргизия, налог 6%: 0.27/(1-0.06-0.27) = 0.27/0.67
		{"Киргизия 27%", 27, ExecutorAibiconKG, 0.40298507462686567},
		// значения из обновлённых файлов
		{"Айбикон 27% (файл)", 27, ExecutorAibicon, 0.5625},
		{"Проект 27% (файл)", 27, ExecutorAibiconProject, 0.3698630136986301},
		// не задана — наценки нет
		{"ноль", 0, ExecutorAibicon, 0},
		// вырожденный случай: 80% + 25% >= 100% → 0 (валидация отсечёт раньше)
		{"вырожденный", 80, ExecutorAibicon, 0},
	}
	for _, tt := range tests {
		if got := markupRate(tt.targetRent, tt.executor); math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("%s: markupRate want %.10f, got %.10f", tt.name, tt.want, got)
		}
	}
}

// TestRun_TargetRentabilityIsReached — главная проверка смысла формулы:
// при режиме наценки итоговая рентабельность должна выйти РОВНО целевой,
// какой бы ни была ставка налога исполнителя.
func TestRun_TargetRentabilityIsReached(t *testing.T) {
	tests := []struct {
		name       string
		executor   string
		targetRent float64
	}{
		{"Айбикон 29% (налог 25%)", ExecutorAibicon, 29},
		{"Проект 70% (налог 0)", ExecutorAibiconProject, 70},
		{"Проект 20% (налог 0)", ExecutorAibiconProject, 20},
		{"Айбикон 27% (налог 25%)", ExecutorAibicon, 27},
	}
	for _, tt := range tests {
		inp := &BudgetInputs{
			ProjectStartDate: mustDate(2026, 12, 1),
			DurationMonths:   3,
			ExecutorName:     tt.executor,
			Internet:         []float64{1_000_000, 2_000_000, 3_000_000},
			Params:           &InputBudgetParams{TargetRentPct: tt.targetRent},
		}
		res := Run(inp)

		if math.Abs(res.Profitability-tt.targetRent) > 0.01 {
			t.Errorf("%s: итоговая рентабельность want %.2f%%, got %.4f%%",
				tt.name, tt.targetRent, res.Profitability)
		}

		// Выручка = расходы × (1 + наценка), маржа = расходы × наценка
		markup := markupRate(tt.targetRent, tt.executor)
		var costs, margin float64
		for _, m := range res.Monthly {
			costs += m.TotalCosts
			margin += m.MarginAmount
		}
		if want := costs * markup; math.Abs(margin-want) > 0.01 {
			t.Errorf("%s: маржа want %.2f, got %.2f", tt.name, want, margin)
		}
		if want := costs * (1 + markup); math.Abs(res.TotalRevenue-want) > 0.01 {
			t.Errorf("%s: выручка want %.2f, got %.2f", tt.name, want, res.TotalRevenue)
		}
	}
}

// TestRun_KirgiziaAlwaysManualRevenue — у «Айбикон Киргизия» ТКП задаётся
// всегда (правило формы), поэтому работает режим ручной выручки, а наценка
// через целевую рентабельность к ней не применяется: H234 обнуляется.
func TestRun_KirgiziaAlwaysManualRevenue(t *testing.T) {
	inp := &BudgetInputs{
		ProjectStartDate: mustDate(2026, 12, 1),
		DurationMonths:   3,
		ExecutorName:     ExecutorAibiconKG,
		Internet:         []float64{1_000_000, 1_000_000, 1_000_000},
		Params: &InputBudgetParams{
			TargetRentPct: 27,          // задана, но не должна применяться
			ContractValue: 100_000_000, // ТКП задан
		},
	}
	res := Run(inp)

	// Наценка в расчёт не идёт: маржа по всем месяцам нулевая
	for m, mr := range res.Monthly {
		if mr.MarginAmount != 0 {
			t.Errorf("месяц %d: при заданном ТКП маржа должна быть 0, got %.2f",
				m+1, mr.MarginAmount)
		}
	}
	// Выручка — ровно ТКП, распределённый по месяцам
	if math.Abs(res.TotalRevenue-100_000_000) > 0.01 {
		t.Errorf("выручка want 100000000 (ТКП), got %.2f", res.TotalRevenue)
	}
	// Рентабельность считается от реальной прибыли, а не от целевой 27%
	if math.Abs(res.Profitability-27) < 0.01 {
		t.Errorf("рентабельность не должна совпадать с целевой 27%%: got %.4f%%",
			res.Profitability)
	}
}

// TestValidateBudgetParams_TargetRent — вырожденный случай:
// целевая рентабельность + ставка налога должны быть строго меньше 100%.
func TestValidateBudgetParams_TargetRent(t *testing.T) {
	tests := []struct {
		name       string
		targetRent float64
		executor   string
		wantErr    bool
	}{
		{"29% при налоге 25%", 29, ExecutorAibicon, false},
		{"74% при налоге 25%", 74, ExecutorAibicon, false},
		{"75% при налоге 25% — ровно 100%", 75, ExecutorAibicon, true},
		{"80% при налоге 25%", 80, ExecutorAibicon, true},
		{"80% при налоге 0 (Проект)", 80, ExecutorAibiconProject, false},
		{"100% при налоге 0", 100, ExecutorAibiconProject, true},
		{"94% при налоге 6% (Киргизия)", 94, ExecutorAibiconKG, true},
		{"не задана", 0, ExecutorAibicon, false},
		{"отрицательная", -5, ExecutorAibicon, true},
	}
	for _, tt := range tests {
		p := &InputBudgetParams{TargetRentPct: tt.targetRent}
		err := ValidateBudgetParams(p, tt.executor)
		if (err != nil) != tt.wantErr {
			t.Errorf("%s: wantErr=%v, got err=%v", tt.name, tt.wantErr, err)
		}
	}

	if err := ValidateBudgetParams(nil, ExecutorAibicon); err != nil {
		t.Errorf("nil должен проходить: %v", err)
	}

	// Через диспетчер ValidateInput
	bad := []byte(`{"target_rent_pct":80}`)
	if err := ValidateInput(TypeBudgetParams, bad, ExecutorAibicon, 6); err == nil {
		t.Error("ValidateInput: ожидалась ошибка для 80% при налоге 25%")
	}
	if err := ValidateInput(TypeBudgetParams, bad, ExecutorAibiconProject, 6); err != nil {
		t.Errorf("ValidateInput: 80%% при налоге 0 должно проходить, got %v", err)
	}
}

// TestRun_BankGuaranteesNeedContractValue — банковские гарантии считаются
// строго от ТКП (стоимости договора), а не от расчётной выручки.
//
// Источник: 2.Бюджет!G220 = G251*F220, G224 = G251*F224, G228 = F228*G251,
// и уже от них G222 / G226 / G230. Если ТКП не задан (режим наценки), эти
// произведения равны нулю — договора ещё нет, гарантию не от чего считать.
func TestRun_BankGuaranteesNeedContractValue(t *testing.T) {
	bg := func() *InputBudgetParams {
		return &InputBudgetParams{
			BGExecution: BankGuarantee{Pct: 15, RatePct: 4, RateType: BGRateTotal, DurationMos: 6},
			BGWarranty:  BankGuarantee{Pct: 33, RatePct: 20, RateType: BGRateTotal, DurationMos: 12},
			BGAdvance:   BankGuarantee{Pct: 7, RatePct: 9, RateType: BGRatePerYear, DurationMos: 12},
		}
	}

	// Режим наценки: ТКП не задан → все БГ нулевые
	pNac := bg()
	pNac.TargetRentPct = 27
	res := Run(&BudgetInputs{
		ProjectStartDate: mustDate(2026, 12, 1),
		DurationMonths:   3,
		ExecutorName:     ExecutorAibicon,
		Internet:         []float64{1_000_000, 1_000_000, 1_000_000},
		Params:           pNac,
	})
	for m, mr := range res.Monthly {
		if mr.BGExecution != 0 || mr.BGWarranty != 0 || mr.BGAdvance != 0 {
			t.Errorf("месяц %d: без ТКП все БГ должны быть 0, got %.2f / %.2f / %.2f",
				m+1, mr.BGExecution, mr.BGWarranty, mr.BGAdvance)
		}
	}

	// Режим ручной выручки: ТКП задан → БГ считаются как раньше
	pTKP := bg()
	pTKP.ContractValue = 200_000_000
	res = Run(&BudgetInputs{
		ProjectStartDate: mustDate(2026, 12, 1),
		DurationMonths:   6,
		ExecutorName:     ExecutorAibicon,
		Params:           pTKP,
	})
	// Эталоны из заполненного файла: G222 = 1 200 000, G226 = 13 200 000,
	// G230 = 1 260 000 — распределяются равномерно по месяцам.
	var gotExec, gotWar, gotAdv float64
	for _, mr := range res.Monthly {
		gotExec += mr.BGExecution
		gotWar += mr.BGWarranty
		gotAdv += mr.BGAdvance
	}
	for _, c := range []struct {
		name      string
		got, want float64
	}{
		{"БГ исполнение (G222)", gotExec, 1_200_000},
		{"БГ гарантийный (G226)", gotWar, 13_200_000},
		{"БГ аванс (G230)", gotAdv, 1_260_000},
	} {
		if math.Abs(c.got-c.want) > 0.01 {
			t.Errorf("%s: want %.2f, got %.2f", c.name, c.want, c.got)
		}
	}
}

// TestRun_OtherExpensesPctUsesEstimatedRevenue — «прочие расходы» в режиме
// процента продолжают считаться от расчётной выручки, даже когда ТКП не
// задан. Этот механизм отдельный от БГ и правкой БГ не затронут.
//
// Источник: 2.Бюджет!H218 = IF(...; $G$251*$F$218/100/$D$8; ...) — в форме
// база тоже ТКП, но платформа подставляет расчётную выручку, чтобы статья
// не обнулялась в режиме наценки.
func TestRun_OtherExpensesPctUsesEstimatedRevenue(t *testing.T) {
	res := Run(&BudgetInputs{
		ProjectStartDate: mustDate(2026, 12, 1),
		DurationMonths:   2,
		ExecutorName:     ExecutorAibiconProject, // налог 0 → наценка = 0.25 при цели 20%
		Internet:         []float64{1_000_000, 1_000_000},
		Params: &InputBudgetParams{
			TargetRentPct:     20,
			OtherExpenseMode:  OtherExpModePct,
			OtherExpenseValue: 10, // 10% от базы
		},
	})

	var other float64
	for _, mr := range res.Monthly {
		other += mr.OtherExpenses
	}
	if other == 0 {
		t.Fatal("прочие расходы в режиме % не должны обнуляться при пустом ТКП")
	}
	// БГ при этом всё равно нулевые — механизмы независимы
	for m, mr := range res.Monthly {
		if mr.BGExecution != 0 || mr.BGWarranty != 0 || mr.BGAdvance != 0 {
			t.Errorf("месяц %d: БГ должны остаться нулевыми", m+1)
		}
	}
}

// TestRun_KirgiziaTaxNeedsContractValue — налог киргизского спецрежима
// считается строго от ТКП (2.Бюджет!$G$251), а не от расчётной выручки.
//
// По правилу формы у киргизского филиала ТКП задаётся всегда, поэтому
// проекта без ТКП быть не может; если он всё же придёт — налог 0, а не
// посчитанный от подставленной выручки.
func TestRun_KirgiziaTaxNeedsContractValue(t *testing.T) {
	base := func(p *InputBudgetParams) *BudgetInputs {
		return &BudgetInputs{
			ProjectStartDate: mustDate(2026, 12, 1),
			DurationMonths:   6,
			ExecutorName:     ExecutorAibiconKG,
			Internet:         []float64{1_000_000, 1_000_000, 1_000_000, 1_000_000, 1_000_000, 1_000_000},
			Params:           p,
		}
	}

	// ТКП задан — налог считается: 200 млн × 7% / 6 мес
	res := Run(base(&InputBudgetParams{
		ContractValue: 200_000_000,
	}))
	if want := 2_333_333.3333333335; math.Abs(res.Tax-want) > 0.01 {
		t.Errorf("с ТКП: налог want %.2f, got %.2f", want, res.Tax)
	}

	// ТКП не задан — налог 0, расчётная выручка в базу не подставляется
	res = Run(base(&InputBudgetParams{TargetRentPct: 27}))
	if res.Tax != 0 {
		t.Errorf("без ТКП: налог должен быть 0, got %.2f", res.Tax)
	}
	// выручка при этом посчитана (режим наценки работает)
	if res.TotalRevenue == 0 {
		t.Error("без ТКП выручка всё равно должна считаться по наценке")
	}
}

// TestRun_AUPPlusTwoMonths — АУП (строка 215) начисляется, пока номер
// месяца <= длительность + 2: H215 = IF(H11<=$D$8+2, (H212+H214+H176)*F215, 0).
//
// Проверено на calc_sheets_ibcon-russia.xlsm (D8=6): в колонках месяцев 7 и 8
// база (212, 214, 176) равна нулю, поэтому АУП там тоже ноль, а G215
// совпадает с суммой первых шести месяцев. То есть «+2» ничего не
// добавляет: строки расходов сами закрыты проверкой «месяц <= D8».
//
// Отсюда требование к платформе: массивы длиной ровно duration дают тот же
// итог, что и форма, и никакого «хвоста» дописывать не нужно.
func TestRun_AUPPlusTwoMonths(t *testing.T) {
	const n = 6
	inp := &BudgetInputs{
		ProjectStartDate: mustDate(2027, 6, 1),
		DurationMonths:   n,
		ExecutorName:     ExecutorAibicon,
		Internet:         []float64{100_000, 100_000, 100_000, 100_000, 100_000, 100_000},
		Params: &InputBudgetParams{
			UnpredictablesPct: 7,
			AUPPct:            15,
		},
	}
	res := Run(inp)

	// Условие «месяц <= duration+2» выполняется для месяцев 1..8, поэтому
	// внутри проекта АУП начисляется в каждом месяце без исключений.
	for m := 0; m < n; m++ {
		if res.Monthly[m].AUP <= 0 {
			t.Errorf("месяц %d: АУП должен начисляться, got %.2f",
				m+1, res.Monthly[m].AUP)
		}
	}

	// Месяцы 7 и 8 существуют только в форме и имеют нулевую базу,
	// поэтому итог АУП равен сумме по месяцам проекта.
	var totalAUP float64
	for m := 0; m < n; m++ {
		totalAUP += res.Monthly[m].AUP
	}
	// Проверяем через формулу: АУП = (расходы + непредвиденные + ФОТ) × 15%
	var want float64
	for m := 0; m < n; m++ {
		mr := res.Monthly[m]
		want += (mr.ProjectCostsExFOT + mr.Unpredictables + mr.TotalFOT) * 0.15
	}
	if math.Abs(totalAUP-want) > 0.01 {
		t.Errorf("итого АУП: want %.2f, got %.2f", want, totalAUP)
	}

	// Массив результатов не должен вырастать до duration+2
	if len(res.Monthly) != n {
		t.Errorf("месяцев в результате должно быть %d, got %d", n, len(res.Monthly))
	}
}

// TestPerDiemRFIsFormulaNotInput — суточные по РФ берутся из формулы
// 4.6!D9 = 700+300/0.87*1.3 ≈ 1148.28, а НЕ из пользовательского ввода.
func TestPerDiemRFIsFormulaNotInput(t *testing.T) {
	const want = 700 + 300/0.87*1.3
	if math.Abs(perDiemRFRate-want) > 1e-9 {
		t.Fatalf("perDiemRFRate: want %.10f, got %.10f", want, perDiemRFRate)
	}
	if math.Abs(perDiemRFRate-1148.2758620689656) > 1e-9 {
		t.Errorf("perDiemRFRate должен совпадать с эталоном 4.6!D9 = 1148.2758620689656, got %.10f",
			perDiemRFRate)
	}

	// Сотрудник с 10 днями командировки по РФ; в ввод кладём заведомо
	// «чужое» значение суточных — оно должно быть проигнорировано.
	mk := func(inputRF float64) *CalcResult {
		return Run(&BudgetInputs{
			ProjectStartDate: mustDate(2027, 6, 1),
			DurationMonths:   2,
			ExecutorName:     ExecutorAibicon,
			Employees: &InputEmployees{
				PerDiemRF:    inputRF,
				PerDiemOther: 2_500,
				Employees: []Employee{{
					Position:        "Инженер",
					Country:         CountryRF,
					BaseSchedule:    "вахта",
					SalaryNet:       100_000,
					MonthlySchedule: []string{"К", "ОФ"},
					TripDaysRF:      []int{10, 0},
					TripDaysOther:   []int{0, 0},
				}},
			},
			Params: &InputBudgetParams{},
		})
	}

	wantPerDiem := 10 * perDiemRFRate
	for _, inputRF := range []float64{0, 1, 5_000, 1_148} {
		got := mk(inputRF).Monthly[0].PerDiem
		if math.Abs(got-wantPerDiem) > 0.01 {
			t.Errorf("ввод суточных РФ = %.0f: командировочные want %.2f (10 × %.4f), got %.2f",
				inputRF, wantPerDiem, perDiemRFRate, got)
		}
	}

	// Суточные по другим странам, наоборот, задаёт пользователь (4.6!D10)
	res := Run(&BudgetInputs{
		ProjectStartDate: mustDate(2027, 6, 1),
		DurationMonths:   1,
		ExecutorName:     ExecutorAibicon,
		Employees: &InputEmployees{
			PerDiemOther: 3_000,
			Employees: []Employee{{
				Position:        "Инженер",
				Country:         CountryRF,
				BaseSchedule:    "вахта",
				SalaryNet:       100_000,
				MonthlySchedule: []string{"К"},
				TripDaysRF:      []int{0},
				TripDaysOther:   []int{4},
			}},
		},
		Params: &InputBudgetParams{},
	})
	if got := res.Monthly[0].PerDiem; math.Abs(got-4*3_000) > 0.01 {
		t.Errorf("суточные за рубеж должны браться из ввода: want 12000.00, got %.2f", got)
	}
}

// TestValidateEmployees_CountryByExecutor — «Киргизия» допустима только
// у исполнителя «Айбикон Киргизия».
func TestValidateEmployees_CountryByExecutor(t *testing.T) {
	mk := func(country string) *InputEmployees {
		return &InputEmployees{Employees: []Employee{{
			Position: "Инженер", FullName: "Асанов А.А.",
			Country: country, BaseSchedule: "вахта", SalaryNet: 100_000,
		}}}
	}

	// Киргизский сотрудник у киргизского исполнителя — можно
	if err := ValidateEmployees(mk(CountryKG), ExecutorAibiconKG); err != nil {
		t.Errorf("Киргизия при исполнителе «Айбикон Киргизия» должна проходить: %v", err)
	}
	// Регистр не должен ломать правило
	if err := ValidateEmployees(mk("Киргизия"), "айбикон киргизия"); err != nil {
		t.Errorf("сравнение должно быть регистронезависимым: %v", err)
	}

	// У остальных исполнителей — нельзя
	for _, ex := range []string{ExecutorAibicon, ExecutorAibiconProject} {
		if err := ValidateEmployees(mk(CountryKG), ex); err == nil {
			t.Errorf("исполнитель %q: Киргизия должна отклоняться, got nil", ex)
		}
	}

	// Россия и самозанятый допустимы у любого исполнителя
	for _, ex := range []string{ExecutorAibicon, ExecutorAibiconProject, ExecutorAibiconKG} {
		for _, c := range []string{CountryRF, CountrySelfEmployed} {
			if err := ValidateEmployees(mk(c), ex); err != nil {
				t.Errorf("исполнитель %q, страна %q: должно проходить, got %v", ex, c, err)
			}
		}
	}

	// Пустая и неизвестная страна отклоняются
	if err := ValidateEmployees(mk(""), ExecutorAibicon); err == nil {
		t.Error("пустая страна должна отклоняться")
	}
	if err := ValidateEmployees(mk("Казахстан"), ExecutorAibicon); err == nil {
		t.Error("неизвестная страна должна отклоняться")
	}

	// Через диспетчер ValidateInput — тот же результат
	raw := []byte(`{"employees":[{"position":"Инженер","country":"киргизия","salary_net":100000}]}`)
	if err := ValidateInput(TypeEmployees, raw, ExecutorAibicon, 6); err == nil {
		t.Error("ValidateInput: Киргизия при «Айбикон» должна отклоняться")
	}
	if err := ValidateInput(TypeEmployees, raw, ExecutorAibiconKG, 6); err != nil {
		t.Errorf("ValidateInput: Киргизия при «Айбикон Киргизия» должна проходить: %v", err)
	}
}
