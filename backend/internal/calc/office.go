package calc

import "fmt"

// ── Лист 4.5 «Офис» ──────────────────────────────────────────────────────────
//
// Разбор формы — mapping/mapping_4.5.md. Лист даёт ДВЕ строки бюджета:
//
//	4.5!C18 = IF(C14<=$D$8, SUMPRODUCT($B$15:$B$17, C15:C17), 0)  итого стоимость
//	4.5!C19 = SUM(C15:C17)                                        итого количество
//	4.5!C5  = IF(C4<=$D$8,
//	             IF($D$10="Айбикон Киргизия", C18, C18/0.87), 0)  → 2.Бюджет 182
//	4.5!C11 = IF(C10<=$D$8, C19*$B$11, 0)                         → 2.Бюджет 183
//
// Две особенности, которых нет у соседних листов:
//
//  1. Gross-up на НДФЛ применяется ТОЛЬКО к аренде и ТОЛЬКО у не-киргизских
//     исполнителей. Уборка не делится на 0.87 вообще. В аренде квартир
//     (4.2!C5) gross-up накрывал аренду и уборку вместе — здесь иначе.
//  2. Уборка идёт ОТДЕЛЬНОЙ строкой бюджета (183), а не внутри аренды, как
//     в 4.2.
//
// Уборка — не фиксированная сумма: экономист задаёт цену уборки ОДНОГО
// офиса за месяц, а сумма месяца получается умножением на количество
// офисов в этом месяце (4.5!C19). Подтверждено владельцем 2026-08-26.
// Выбора месяцев, как в 4.2 (CleaningMonths), здесь нет: уборка идёт в
// каждом месяце, где стоит хотя бы один офис.

// InputOffice — данные листа 4.5.
type InputOffice struct {
	// Offices — арендуемые офисы: у каждого своя цена за месяц и количество
	// по месяцам (4.5!B15:B17 и 4.5!C15:BJ17). В форме слотов ровно три,
	// в платформе список динамический — ограничение формы верстальное.
	Offices []RentedItem `json:"offices"`
	// CleaningPrice — стоимость уборки ОДНОГО офиса за месяц (4.5!B11).
	CleaningPrice float64 `json:"cleaning_price"`
}

// calcOffice рассчитывает лист 4.5.
func calcOffice(in *InputOffice, executor string, duration int) (rent, cleaning []float64) {
	rent = make([]float64, duration)
	cleaning = make([]float64, duration)
	if in == nil || duration <= 0 {
		return rent, cleaning
	}

	// Для «Айбикон Киргизия» gross-up на НДФЛ не применяется (4.5!C5).
	isKG := sameExecutor(executor, ExecutorAibiconKG)

	for m := 0; m < duration; m++ {
		// 4.5!C18 — SUMPRODUCT(цены, количества) по всем офисам месяца,
		// и 4.5!C19 — сумма количеств, она же база уборки.
		var cost float64
		var count int
		for i := range in.Offices {
			o := &in.Offices[i]
			c := intVal(o.Counts, m)
			cost += o.Price * float64(c)
			count += c
		}

		// 4.5!C5 — аренда с gross-up на НДФЛ 13% (кроме Киргизии).
		// Проверка «месяц внутри проекта» не нужна: массив строится ровно
		// на duration месяцев.
		if !isKG {
			cost /= npflGrossUpDivisor
		}
		rent[m] = cost

		// 4.5!C11 — уборка: количество офисов × цена уборки одного офиса.
		// Без gross-up: в форме деления на 0.87 в этой строке нет.
		cleaning[m] = float64(count) * in.CleaningPrice
	}

	return rent, cleaning
}

// ValidateOffice проверяет ввод листа 4.5: цены и количества не могут быть
// отрицательными. Ноль допустим — офиса в этом месяце просто нет.
func ValidateOffice(in *InputOffice) error {
	if in == nil {
		return nil
	}
	if in.CleaningPrice < 0 {
		return fmt.Errorf("уборка офиса: стоимость не может быть отрицательной (%.2f)",
			in.CleaningPrice)
	}
	for i := range in.Offices {
		o := &in.Offices[i]
		if o.Price < 0 {
			return fmt.Errorf("аренда офиса, строка %d (%s): цена не может быть отрицательной (%.2f)",
				i+1, itemTitle(o.Name), o.Price)
		}
		for m, c := range o.Counts {
			if c < 0 {
				return fmt.Errorf(
					"аренда офиса, строка %d (%s): количество в месяце %d не может быть отрицательным (%d)",
					i+1, itemTitle(o.Name), m+1, c)
			}
		}
	}
	return nil
}
