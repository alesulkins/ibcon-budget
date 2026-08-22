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
			OpMarginPct:       20,
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

	// Revenue = TotalCosts × 1.20
	wantRevenue := wantGross * 1.20
	if math.Abs(mr.Revenue-wantRevenue) > 1 {
		t.Errorf("Revenue: want %.2f, got %.2f", wantRevenue, mr.Revenue)
	}

	// Выручка с НДС = Revenue × 1.22 (Айбикон)
	wantRevVAT := wantRevenue * 1.22
	if math.Abs(mr.RevenueWithVAT-wantRevVAT) > 1 {
		t.Errorf("RevenueWithVAT: want %.2f, got %.2f", wantRevVAT, mr.RevenueWithVAT)
	}
}

func TestRun_ManualRevenue(t *testing.T) {
	// Ручная выручка 1 000 000 руб, 2 месяца — равномерно 500 000/мес
	inp := &BudgetInputs{
		ProjectStartDate: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
		DurationMonths:   2,
		ExecutorName:     ExecutorAibicon,
		Employees:        nil,
		Params: &InputBudgetParams{
			UnpredictablesPct: 0,
			AUPPct:            0,
			ManualRevenue:     1_000_000,
		},
	}

	res := Run(inp)
	for i, mr := range res.Monthly {
		if math.Abs(mr.Revenue-500_000) > 0.01 {
			t.Errorf("month %d Revenue: want 500000, got %.2f", i+1, mr.Revenue)
		}
	}
	if math.Abs(res.TotalRevenue-1_000_000) > 0.01 {
		t.Errorf("TotalRevenue: want 1000000, got %.2f", res.TotalRevenue)
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
		Params:           &InputBudgetParams{OpMarginPct: 10},
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
		inp := &BudgetInputs{
			ProjectStartDate: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			DurationMonths:   1,
			ExecutorName:     tt.executor,
			Params:           &InputBudgetParams{ManualRevenue: 1_000_000},
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
			ManualRevenue: 10_000_000,
			ContractValue: 10_000_000,
			OpMarginPct:   25, // задана, но при ручной выручке не применяется
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
				ManualRevenue: 200_000_000,
				ContractValue: 200_000_000,
			},
		}
	}

	// Айбикон: налог = операционная прибыль × 25%
	res := Run(mk(ExecutorAibicon))
	wantProfit := 200_000_000.0 - 6_000_000.0
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
		Params:           &InputBudgetParams{OpMarginPct: 20}, // ручной выручки нет
	}
	res := Run(inp)

	// маржа = расходы × 20% = 200 000/мес, итого 400 000
	wantMargin := 400_000.0
	var gotMargin float64
	for _, m := range res.Monthly {
		gotMargin += m.MarginAmount
	}
	if math.Abs(gotMargin-wantMargin) > 0.01 {
		t.Fatalf("итого маржа: want %.2f, got %.2f", wantMargin, gotMargin)
	}
	// налог от маржи: 400 000 × 25%
	if want := wantMargin * 0.25; math.Abs(res.Tax-want) > 0.01 {
		t.Errorf("налог от маржи: want %.2f, got %.2f", want, res.Tax)
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
