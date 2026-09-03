package calc

import "fmt"

// ── Листы-списки: 4.8 «ПО», 4.9 «ГПХ внешний», 4.11 «Субподряд» ─────────────
//
// Три листа устроены одинаково, поэтому и структура ввода, и расчёт у них
// общие: список позиций, у каждой своё наименование и стоимость, заданная
// отдельно в каждом месяце. Итог месяца — сумма стоимостей всех позиций,
// уходит ОДНОЙ строкой в 2.Бюджет:
//
//	4.8  ПО и лицензии → строка 193 «Приобретение ПО»
//	4.9  ГПХ внешний   → строка 200
//	4.11 Субподряд     → строка 202
//
// Расчётных формул в этих листах нет — стоимость вводится напрямую, поэтому
// расчёт сводится к суммированию, и сверка с Excel по ним не проводилась
// (решение владельца 2026-08-26). Тем же и отличаются от 4.2–4.5: там
// стоимость выводилась из цены и количества, здесь её вводят готовой.
//
// От старого ввода {monthly_amounts:[...]} отличие одно, но существенное:
// сумма месяца разложена по позициям, поэтому видно, из чего она сложилась.

// CostLine — одна позиция листа-списка: наименование и стоимость по месяцам.
type CostLine struct {
	// Name — наименование ПО / контрагент-услуга / наименование работ.
	// В расчёте не участвует, нужно только чтобы опознать строку.
	Name string `json:"name"`
	// MonthlyAmounts — стоимость по месяцам проекта, индекс 0 = первый месяц.
	MonthlyAmounts []float64 `json:"monthly_amounts"`
}

// InputCostLines — ввод листа-списка (4.8, 4.9, 4.11).
type InputCostLines struct {
	Lines []CostLine `json:"lines"`
}

// calcCostLines рассчитывает лист-список. Возвращает массив длиной duration:
// итог месяца = сумма стоимостей всех позиций в этом месяце.
func calcCostLines(in *InputCostLines, duration int) []float64 {
	out := make([]float64, duration)
	if in == nil || duration <= 0 {
		return out
	}
	for i := range in.Lines {
		amounts := in.Lines[i].MonthlyAmounts
		for m := 0; m < duration && m < len(amounts); m++ {
			out[m] += amounts[m]
		}
	}
	return out
}

// costLinesTitle — название листа для текста ошибки валидации. Экономист
// видит его в тосте, поэтому это подпись шага мастера, а не ключ ввода.
func costLinesTitle(inputType string) string {
	switch inputType {
	case TypeSubcontractExtItems:
		return "ГПХ внешний"
	case TypeSubcontractGenItems:
		return "субподрядные работы"
	default:
		return "ПО и лицензии"
	}
}

// ValidateCostLines проверяет ввод листа-списка: стоимость не может быть
// отрицательной. Ноль допустим — в этом месяце позиции просто нет.
func ValidateCostLines(title string, in *InputCostLines) error {
	if in == nil {
		return nil
	}
	for i := range in.Lines {
		l := &in.Lines[i]
		for m, v := range l.MonthlyAmounts {
			if v < 0 {
				return fmt.Errorf(
					"%s, строка %d (%s): стоимость в месяце %d не может быть отрицательной (%.2f)",
					title, i+1, itemTitle(l.Name), m+1, v)
			}
		}
	}
	return nil
}
