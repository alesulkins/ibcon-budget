package calc

import "fmt"

// ── Лист 4.4 «Обустройство стройплощадки (вагончики)» ───────────────────────
//
// Разбор формы — mapping/mapping_4.4.md. Лист даёт ОДНУ строку бюджета,
// склеенную из аренды и покупки:
//
//	4.4!C6  = IF(C4<=$D$8, C5*$B$6, 0)
//	          + SUMPRODUCT(($A$12:$A$22=C4)*($D$12:$D$22))   → 2.Бюджет 181
//	4.4!D12 = IF(A12<=$D$8, C12*B12, 0)                      (таблица покупок)
//	4.4!BK5 = SUM(C6:BJ6)                                    (итог, строка 5!)
//
// Деления на 0.87 (gross-up на НДФЛ) здесь нет — вагончики арендуют и
// покупают у юрлиц. Проверено сплошным поиском по листу.
//
// ПРО РАСЩЕПЛЁННУЮ ФОРМУЛУ СТРОКИ 6 — это НЕ опечатка (подтверждено
// владельцем 2026-08-26). Полноценная ветка с SUMPRODUCT стоит только в
// первых колонках, а в хвостовых — вырожденная `IF(месяц=1, $D$12, 0)`,
// которая для реальных месяцев всегда даёт ноль. Так форма выражает
// бизнес-правило: **покупка невозможна в последние два месяца проекта**.
// В `calc_sheets_ibcon-project-russia.xlsm` граница стоит ровно там:
// SUMPRODUCT в C:F (месяцы 1-4 при D=6), вырожденная ветка в G:BJ
// (месяцы 5 и 6). В платформе это правило выражено явно —
// purchaseAllowedMonths, — а формула единая на все месяцы.

// MonthlyQty — цена за единицу плюс количество в каждом месяце.
// Ровно структура блока аренды в форме: 4.4!B6 + 4.4!C5:BJ5.
type MonthlyQty struct {
	// Цена аренды одной единицы за месяц (4.4!B6)
	Price float64 `json:"price"`
	// Количество по месяцам, индекс 0 = первый месяц проекта (4.4!C5:BJ5)
	Counts []int `json:"counts"`
}

// InputWagonciks — данные листа 4.4.
//
// Аренда — цена за единицу плюс количество по месяцам (блок 4.4 строки 5-6).
// Покупка — таблица строк «описание / месяц / цена / количество»
// (блок 4.4!A12:D22), как у покупки авто на листе 4.3: владелец потребовал
// одинакового ввода в обоих местах.
type InputWagonciks struct {
	Rental    MonthlyQty     `json:"rental"`
	Purchases []ItemPurchase `json:"purchases"`
}

// purchaseAllowedMonths — сколько первых месяцев проекта открыты для покупки
// вагончиков. Правило владельца (2026-08-26): покупка невозможна в последние
// два месяца проекта, то есть допустимы месяцы 1…D−2. Именно это правило
// закодировано границей расщепления формулы в 4.4!строка 6.
//
// Краевой случай D=1 оговорён отдельно: единственный месяц проекта остаётся
// открытым, иначе покупку негде было бы указать вовсе.
//
// Следствие, о котором надо помнить: при D=2 покупка недоступна ни в одном
// месяце — правило блокирует оба.
func purchaseAllowedMonths(duration int) int {
	if duration == 1 {
		return 1
	}
	if duration < 2 {
		return 0
	}
	return duration - 2
}

// PurchaseAllowedMonths — то же правило для фронта и валидации.
func PurchaseAllowedMonths(duration int) int { return purchaseAllowedMonths(duration) }

// calcWagonciks рассчитывает лист 4.4. Возвращает массив длиной duration:
// аренда + покупка одной суммой (4.4!строка 6) → 2.Бюджет строка 181.
func calcWagonciks(in *InputWagonciks, duration int) []float64 {
	out := make([]float64, duration)
	if in == nil || duration <= 0 {
		return out
	}

	// Аренда: 4.4!C6, первое слагаемое — IF(C4<=$D$8, C5*$B$6, 0).
	// Проверка «месяц внутри проекта» здесь не нужна: массив строится
	// ровно на duration месяцев.
	for m := 0; m < duration; m++ {
		out[m] = in.Rental.Price * float64(intVal(in.Rental.Counts, m))
	}

	// Покупка: 4.4!C6, второе слагаемое — SUMPRODUCT по таблице A12:D22,
	// то есть сумма строк, чей месяц равен месяцу колонки. Формула единая
	// на все допустимые месяцы, а не только на первый (см. шапку файла).
	allowed := purchaseAllowedMonths(duration)
	for i := range in.Purchases {
		p := &in.Purchases[i]
		// Месяц не заполнен (0) — строка таблицы пустая.
		// Месяц в последних двух месяцах проекта сюда не доходит: его
		// отклоняет ValidateWagonciks. Проверка здесь — страховка на случай
		// данных, сохранённых до сокращения длительности проекта.
		if p.Month < 1 || p.Month > allowed {
			continue
		}
		out[p.Month-1] += p.Price * float64(p.Count)
	}

	return out
}

// ValidateWagonciks проверяет ввод листа 4.4.
//
// Главное правило — месяц покупки должен лежать в 1…D−2: в последние два
// месяца проекта вагончики не покупают. Пустой месяц (0) означает
// незаполненную строку таблицы и пропускается, как в 4.3.
func ValidateWagonciks(in *InputWagonciks, duration int) error {
	if in == nil {
		return nil
	}
	if err := validateMonthlyQty("аренда вагончиков", &in.Rental); err != nil {
		return err
	}

	allowed := purchaseAllowedMonths(duration)
	for i := range in.Purchases {
		p := &in.Purchases[i]
		if p.Price < 0 {
			return fmt.Errorf("покупка вагончиков, строка %d (%s): цена не может быть отрицательной (%.2f)",
				i+1, itemTitle(p.Name), p.Price)
		}
		if p.Count < 0 {
			return fmt.Errorf("покупка вагончиков, строка %d (%s): количество не может быть отрицательным (%d)",
				i+1, itemTitle(p.Name), p.Count)
		}
		if p.Month == 0 {
			continue // строка не заполнена — игнорируется целиком
		}
		if p.Month < 1 || p.Month > allowed {
			return fmt.Errorf(
				"покупка вагончиков, строка %d (%s): месяц %d недоступен — "+
					"вагончики не покупают в последние два месяца проекта, "+
					"допустимо от 1 до %d",
				i+1, itemTitle(p.Name), p.Month, allowed)
		}
	}
	return nil
}

func validateMonthlyQty(what string, q *MonthlyQty) error {
	if q.Price < 0 {
		return fmt.Errorf("%s: цена не может быть отрицательной (%.2f)", what, q.Price)
	}
	for m, c := range q.Counts {
		// Ноль допустим: в этом месяце просто ничего нет.
		if c < 0 {
			return fmt.Errorf("%s: количество в месяце %d не может быть отрицательным (%d)",
				what, m+1, c)
		}
	}
	return nil
}
