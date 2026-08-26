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

// calcTransport рассчитывает лист 4.3. Возвращает два массива длиной duration:
//
//	transport — аренда авто + покупка авто (4.3!строка 6)  → 2.Бюджет 180
//	garage    — аренда гаража              (4.3!строка 11) → 2.Бюджет 210
func calcTransport(in *InputTransport, duration int) (transport, garage []float64) {
	transport = make([]float64, duration)
	garage = make([]float64, duration)
	if in == nil || duration <= 0 {
		return transport, garage
	}

	// Покупка авто (4.3!A17:D27 → слагаемое SUMPRODUCT в 4.3!C6).
	//
	// В форме проверка «месяц покупки не выходит за проект» стоит только у
	// строк 17–24, а у строк 25–27 её нет (`D25 = C25*B25`): покупка на
	// месяц за пределами проекта в последних трёх строках попадала в итог,
	// в первых восьми — обнулялась. Опечатка заполнения, исправляем —
	// см. docs/deviations.md §3. Здесь правило единое для всех строк.
	for i := range in.CarPurchases {
		p := &in.CarPurchases[i]
		// Месяц не заполнен (0) — строка таблицы пустая, пропускаем целиком.
		// Месяц вне проекта сюда не доходит: его отклоняет ValidateTransport.
		if p.Month < 1 || p.Month > duration {
			continue
		}
		transport[p.Month-1] += p.Price
	}

	// Аренда: цена за месяц × месяцы, в которых предмет арендуется.
	// В форме это `кол-во в месяце × единая цена` (4.3!C5*$B$6 и 4.3!C10*$B$11);
	// в платформе каждый арендуемый предмет — своя строка со своей ценой,
	// поэтому «количество в месяце» = число строк, включивших этот месяц.
	addRentalMonths(transport, in.CarRentals, duration)
	addRentalMonths(garage, in.GarageRentals, duration)

	return transport, garage
}

// addRentalMonths начисляет цену каждого предмета в каждый выбранный месяц.
// Повторы месяцев внутри одной строки игнорируются: чекбокс нельзя поставить
// дважды, а вот в сохранённом JSON дубль теоретически возможен.
func addRentalMonths(dst []float64, items []RentedItem, duration int) {
	for i := range items {
		it := &items[i]
		seen := make(map[int]bool, len(it.Months))
		for _, m := range it.Months {
			if m < 1 || m > duration || seen[m] {
				continue
			}
			seen[m] = true
			dst[m-1] += it.Price
		}
	}
}

// ValidateTransport проверяет ввод листа 4.3.
//
// Главное правило — месяц покупки и месяцы аренды обязаны лежать внутри
// проекта. В форме этой проверки нет и ошибку она не показывает: покупка с
// пустым месяцем считалась в колонке «Итого» таблицы, но ни с одним месяцем
// не сопоставлялась и в бюджет не попадала (SUMPRODUCT не находил колонку).
// Здесь пустой месяц — это осознанно пустая строка таблицы, а вот месяц за
// пределами проекта отклоняем.
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

	if err := validateRentals("аренда авто", in.CarRentals, duration); err != nil {
		return err
	}
	return validateRentals("аренда гаража", in.GarageRentals, duration)
}

func validateRentals(what string, items []RentedItem, duration int) error {
	for i := range items {
		it := &items[i]
		if it.Price < 0 {
			return fmt.Errorf("%s, строка %d (%s): цена не может быть отрицательной (%.2f)",
				what, i+1, itemTitle(it.Name), it.Price)
		}
		for _, m := range it.Months {
			if m < 1 || m > duration {
				return fmt.Errorf(
					"%s, строка %d (%s): месяц %d вне проекта, допустимо от 1 до %d",
					what, i+1, itemTitle(it.Name), m, duration)
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
