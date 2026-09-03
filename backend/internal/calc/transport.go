package calc

import "fmt"

// ── Лист 4.3 «Транспорт» ─────────────────────────────────────────────────────
//
// Разбор формы — mapping/mapping_4.3.md. Коротко: лист даёт две строки
// бюджета, и одна из них склеена из двух разных вещей.
//
//	4.3!C6  = IF(C4<=$D$8, C5*$B$6, 0)
//	          + SUMPRODUCT(($A$17:$A$27=C4)*($D$17:$D$27))   → 2.Бюджет 180
//	4.3!C11 = IF(C9<=$D$8, C10*$B$11, 0)                     → 2.Бюджет 210
//	4.3!D17 = IF(A17<=$D$8, C17*B17, 0)                      (таблица покупок)
//
// Аренда авто и покупка авто складываются в ОДНУ строку 180 «Аренда
// транспорта» — так устроена форма (заголовок 4.3!C3 «Итого по месяцам
// аренда и покупка авто»). Гараж идёт отдельной строкой 210 «Аренда гаража».
//
// Деления на 0.87 (gross-up на НДФЛ) здесь нет — в отличие от 4.2 и 4.5.
// Транспорт и гараж арендуют у юрлиц, НДФЛ не возникает.
//
// Проверка «месяц <= длительность проекта» в платформе превращается в
// валидацию ввода (ValidateTransport): месяцев за пределами проекта у нас
// не существует, массивы строятся ровно на duration месяцев.

// calcTransport рассчитывает лист 4.3.
func calcTransport(in *InputTransport, duration int) (transport, garage []float64) {
	transport = make([]float64, duration)
	garage = make([]float64, duration)
	if in == nil || duration <= 0 {
		return transport, garage
	}

	// Покупка авто (4.3!A17:D27 → слагаемое SUMPRODUCT в 4.3!C6).
	for i := range in.CarPurchases {
		p := &in.CarPurchases[i]
		// Месяц не заполнен (0) — строка таблицы пустая, пропускаем целиком.
		// Месяц вне проекта сюда не доходит: его отклоняет ValidateTransport.
		if p.Month < 1 || p.Month > duration {
			continue
		}
		// 4.3!D17 = IF(A17<=$D$8, C17*B17, 0) — стоимость × количество
		transport[p.Month-1] += p.Price * float64(p.Count)
	}

	// Аренда: цена × количество в этом месяце, просуммированное по строкам.
	// Ровно как 4.3!C5*$B$6 (авто) и 4.3!C10*$B$11 (гараж), только строк
	// может быть несколько и у каждой своя цена.
	addRentalMonths(transport, in.CarRentals, duration)
	addRentalMonths(garage, in.GarageRentals, duration)

	return transport, garage
}

// addRentalMonths начисляет `цена × количество в месяце` по каждой строке.
// Количества короче duration дополняются нулями, длиннее — обрезаются:
// месяцев за пределами проекта в платформе не существует.
func addRentalMonths(dst []float64, items []RentedItem, duration int) {
	for i := range items {
		it := &items[i]
		if it.Price == 0 {
			continue
		}
		for m := 0; m < duration; m++ {
			if c := intVal(it.Counts, m); c != 0 {
				dst[m] += it.Price * float64(c)
			}
		}
	}
}

// ValidateTransport проверяет ввод листа 4.3. Главное правило — месяц
// покупки и месяцы аренды обязаны лежать внутри проекта.
func ValidateTransport(in *InputTransport, duration int) error {
	if in == nil {
		return nil
	}

	for i := range in.CarPurchases {
		p := &in.CarPurchases[i]
		if p.Price < 0 {
			return fmt.Errorf("покупка авто, строка %d (%s): цена не может быть отрицательной (%.2f)",
				i+1, itemTitle(p.Name), p.Price)
		}
		if p.Count < 0 {
			return fmt.Errorf("покупка авто, строка %d (%s): количество не может быть отрицательным (%d)",
				i+1, itemTitle(p.Name), p.Count)
		}
		if p.Month == 0 {
			continue // строка не заполнена — игнорируется целиком
		}
		if p.Month < 1 || p.Month > duration {
			return fmt.Errorf(
				"покупка авто, строка %d (%s): месяц покупки %d вне проекта, "+
					"допустимо от 1 до %d",
				i+1, itemTitle(p.Name), p.Month, duration)
		}
	}

	if err := validateRentals("аренда авто", in.CarRentals); err != nil {
		return err
	}
	return validateRentals("аренда гаража", in.GarageRentals)
}

// validateRentals — длину массива количеств не проверяем: лишние месяцы
// расчёт обрезает, недостающие считает нулями. Границы проекта здесь ни при
// чём, потому что месяц задан позицией в массиве, а не числом.
func validateRentals(what string, items []RentedItem) error {
	for i := range items {
		it := &items[i]
		if it.Price < 0 {
			return fmt.Errorf("%s, строка %d (%s): цена не может быть отрицательной (%.2f)",
				what, i+1, itemTitle(it.Name), it.Price)
		}
		for m, c := range it.Counts {
			// Ноль допустим: в этом месяце просто не арендуем.
			if c < 0 {
				return fmt.Errorf(
					"%s, строка %d (%s): количество в месяце %d не может быть отрицательным (%d)",
					what, i+1, itemTitle(it.Name), m+1, c)
			}
		}
	}
	return nil
}

// itemTitle подставляет заглушку, если название не заполнено, — иначе в
// тексте ошибки получится «строка 2 (): …».
func itemTitle(name string) string {
	if name == "" {
		return "без названия"
	}
	return name
}
