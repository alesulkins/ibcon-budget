package calc

import (
	"math"
	"testing"
)

// Справочные значения исполнителя (справочник «Исполнители») попадают в
// форму как значение по умолчанию и сохраняются в версии бюджета.

func pct(v float64) *float64 { return &v }

// Ставка из формы перекрывает ставку эталонной формы.
func TestEffectiveTaxRate_FormWinsOverExcel(t *testing.T) {
	tests := []struct {
		name     string
		params   *InputBudgetParams
		executor string
		want     float64
	}{
		{"нет параметров — ставка формы", nil, ExecutorAibicon, 0.25},
		{"поля нет — ставка формы", &InputBudgetParams{}, ExecutorAibicon, 0.25},
		{"справочное 25%", &InputBudgetParams{ProfitTaxPct: pct(25)}, ExecutorAibicon, 0.25},
		// Ноль — законное значение, а не «поля не было».
		{"справочное 0", &InputBudgetParams{ProfitTaxPct: pct(0)}, ExecutorAibiconProject, 0},
		// Актуальная ставка киргизского филиала вместо 6% эталонной формы.
		{"справочное 4%", &InputBudgetParams{ProfitTaxPct: pct(4)}, ExecutorAibiconKG, 0.04},
	}
	for _, tt := range tests {
		if got := effectiveTaxRate(tt.params, tt.executor); math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("%s: want %.4f, got %.4f", tt.name, tt.want, got)
		}
	}
}

// Наценка через целевую рентабельность считается от ставки из формы, а
// не от ставки, привязанной к исполнителю.
func TestMarkupRate_UsesReferenceRate(t *testing.T) {
	// Киргизия, целевая 27%: по умолчанию налог 4%, в форме поставили 25%.
	const target = 27
	wantDefault := 0.27 / (1 - 0.04 - 0.27)
	wantForm := 0.27 / (1 - 0.25 - 0.27)

	if got := markupRateAt(target, profitTaxRate(ExecutorAibiconKG)); math.Abs(got-wantDefault) > 1e-9 {
		t.Errorf("без справочного значения want %.10f, got %.10f", wantDefault, got)
	}
	got := markupRateAt(target, effectiveTaxRate(&InputBudgetParams{ProfitTaxPct: pct(25)}, ExecutorAibiconKG))
	if math.Abs(got-wantForm) > 1e-9 {
		t.Errorf("со справочным значением want %.10f, got %.10f", wantForm, got)
	}
}

// Киргизский спецрежим: справочная ставка берётся от всего ТКП разом и
// заменяет собой слагаемые «5% и 2%» от ТКП, делённого на длительность.
func TestRun_KGTaxUsesReferenceRate(t *testing.T) {
	const tkp = 120_000_000
	const months = 6

	mk := func(p *InputBudgetParams) *BudgetInputs {
		return &BudgetInputs{
			ProjectStartDate: mustDate(2026, 12, 1),
			DurationMonths:   months,
			ExecutorName:     ExecutorAibiconKG,
			Internet:         []float64{1_000_000},
			Params:           p,
		}
	}

	// Без справочного значения — эталонная форма: 5% + 2%.
	wantExcel := tkp/float64(months)/100*5 + tkp*2/float64(months)/100
	res := Run(mk(&InputBudgetParams{ContractValue: tkp}))
	if math.Abs(res.Tax-wantExcel) > 0.01 {
		t.Errorf("эталонная форма: налог want %.2f, got %.2f", wantExcel, res.Tax)
	}

	// Со справочным значением 4% налог берётся от всего ТКП разом,
	// без деления на длительность.
	wantRef := tkp * 0.04
	res = Run(mk(&InputBudgetParams{ContractValue: tkp, ProfitTaxPct: pct(4)}))
	if math.Abs(res.Tax-wantRef) > 0.01 {
		t.Errorf("справочная ставка: налог want %.2f, got %.2f", wantRef, res.Tax)
	}
}

// Валидация целевой рентабельности тоже смотрит на справочную ставку:
// при налоге 0 допустима рентабельность, недопустимая при 25%.
func TestValidateBudgetParams_UsesReferenceRate(t *testing.T) {
	p := &InputBudgetParams{TargetRentPct: 80, ProfitTaxPct: pct(0)}
	if err := ValidateBudgetParams(p, ExecutorAibicon); err != nil {
		t.Errorf("со справочной ставкой 0 рентабельность 80%% допустима, получили: %v", err)
	}
	p = &InputBudgetParams{TargetRentPct: 80}
	if err := ValidateBudgetParams(p, ExecutorAibicon); err == nil {
		t.Error("без справочной ставки 80% + налог 25% должны быть отвергнуты")
	}
}

// Строка 249 «Стоимость + ставка рефинансирования на 1–4 месяцы».
func TestCalcRefRate(t *testing.T) {
	rev := []float64{1_000_000, 2_000_000, 3_000_000, 4_000_000, 5_000_000, 6_000_000}

	// ROUND(14.5/12, 2) = ROUND(1.20833…, 2) = 1.21 — округляется частное,
	// до умножения. Ставка задана в процентах за год, не долей единицы.
	got := calcRefRate(rev, 14.5)
	want := []float64{1_210_000, 2_420_000, 3_630_000, 4_840_000, 0, 0}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 0.01 {
			t.Errorf("месяц %d: want %.2f, got %.2f", i+1, want[i], got[i])
		}
	}

	// Проект короче четырёх месяцев — считаем сколько есть, без паники.
	if got := calcRefRate(rev[:2], 14.5); len(got) != 2 {
		t.Errorf("короткий проект: длина %d, ожидали 2", len(got))
	}
}

// В итог расчёта строка 249 попадает суммой за первые четыре месяца и
// при этом не влияет ни на расходы, ни на прибыль, ни на налог.
func TestRun_RefRateIsInformationalOnly(t *testing.T) {
	mk := func(p *InputBudgetParams) *BudgetInputs {
		return &BudgetInputs{
			ProjectStartDate: mustDate(2026, 12, 1),
			DurationMonths:   6,
			ExecutorName:     ExecutorAibiconProject,
			Internet:         []float64{1_000_000, 1_000_000, 1_000_000, 1_000_000, 1_000_000, 1_000_000},
			Params:           p,
		}
	}

	base := Run(mk(&InputBudgetParams{TargetRentPct: 20}))
	// Та же версия с другой ставкой рефинансирования.
	other := Run(mk(&InputBudgetParams{TargetRentPct: 20, RefinancingPct: pct(30)}))

	if base.TotalCosts != other.TotalCosts ||
		base.NetProfit != other.NetProfit ||
		base.Tax != other.Tax ||
		base.TotalRevenue != other.TotalRevenue {
		t.Error("ставка рефинансирования не должна влиять на расходы, прибыль, налог и выручку")
	}

	// А сам показатель от ставки зависит и берёт её из формы.
	if base.RefRatePct != 14.5 {
		t.Errorf("ставка по умолчанию: want 14.5, got %v", base.RefRatePct)
	}
	if other.RefRatePct != 30 {
		t.Errorf("ставка из формы: want 30, got %v", other.RefRatePct)
	}

	var wantBase float64
	for m := 0; m < refRateMonths; m++ {
		wantBase += base.Monthly[m].RevenueWithVAT * 1.21
	}
	if math.Abs(base.RefRateAmount-wantBase) > 0.01 {
		t.Errorf("итог строки 249: want %.2f, got %.2f", wantBase, base.RefRateAmount)
	}
	// Месяцы после четвёртого в показатель не входят.
	for m := refRateMonths; m < len(base.Monthly); m++ {
		if base.Monthly[m].RefRateAmount != 0 {
			t.Errorf("месяц %d должен быть нулевым, got %.2f", m+1, base.Monthly[m].RefRateAmount)
		}
	}
}
