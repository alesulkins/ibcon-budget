package calc

import (
	"encoding/json"
	"fmt"
	"strings"
)

// sameExecutor сравнивает название исполнителя регистронезависимо, как это
// делает Excel (SUMIF/сравнение строк в Excel регистр не различает).
// См. CLAUDE.md → «Санкционированное отклонение №3», п.3.
func sameExecutor(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

// normalizeCountry приводит «Страну НО» к каноническому виду для сравнения
// с константами CountryRF / CountryKG / CountrySelfEmployed.
//
// Нужно потому, что Excel сравнивает страну через SUMIF, а он
// регистронезависим: в форме записано «Россия», а формула ищет «россия»
// (2.Бюджет!H172, H173, H174). Без нормализации все налоги обнуляются.
// У поля 4.6!BS нет выпадающего списка — значение вводится свободным
// текстом, поэтому пробелы по краям тоже срезаем.
func normalizeCountry(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// intVal возвращает значение из массива по индексу (0-based), 0 если за пределами.
func intVal(arr []int, idx int) int {
	if idx >= 0 && idx < len(arr) {
		return arr[idx]
	}
	return 0
}

// apartmentCount — общее количество квартир всех типов в месяце monthIdx
// (0-based). Формула Excel: 4.2!C23 = SUM(C19:C21).
func apartmentCount(in *InputRentApartments, monthIdx int) int {
	return intVal(in.Count1Room, monthIdx) +
		intVal(in.Count2Room, monthIdx) +
		intVal(in.Count3Room, monthIdx)
}

// cleaningEnabled сообщает, начисляется ли уборка в месяце monthIdx (0-based).
//
// Пустой список означает «уборки нет за весь период» — так решил владелец
// 2026-08-23. Это расширение сверх формы: в Excel уборка безусловна.
func cleaningEnabled(in *InputRentApartments, monthIdx int) bool {
	for _, m := range in.CleaningMonths {
		if m == monthIdx+1 { // в списке номера месяцев 1-based
			return true
		}
	}
	return false
}

// calcRentApartments рассчитывает аренду квартир и услуги риелтора по месяцам
// (лист 4.2). Возвращает два массива длиной duration:
//
//	rent    — «Итого аренда» (4.2!строка 5)  → 2.Бюджет строка 178
//	realtor — «Риелтор»      (4.2!строка 13) → 2.Бюджет строка 179
//
// Формулы Excel (месяц 1 = колонка C, месяц D = колонка BJ):
//
//	Основа   4.2!C22 = IF(месяц<=D8, SUMPRODUCT($B$19:$B$21, C19:C21), 0)
//	Уборка   4.2!C9  = IF(месяц<=D8, $B$9*(C19+C20+C21), 0)
//	Аренда   4.2!C5  = IF(исполнитель="Айбикон Киргизия", C22+C9, (C22+C9)/0.87)
//	Риелтор  4.2!C13 = IF(C12=1, C23*$B$13, IF(C23>B23, (C23-B23)*$B$13, 0))
//
// Проверка «месяц <= длительность проекта» здесь не нужна: массивы строятся
// ровно на duration месяцев, месяцев за пределами проекта в платформе нет.
func calcRentApartments(in *InputRentApartments, executor string, duration int) (rent, realtor []float64) {
	rent = make([]float64, duration)
	realtor = make([]float64, duration)
	if in == nil || duration <= 0 {
		return rent, realtor
	}

	// Для «Айбикон Киргизия» gross-up на НДФЛ не применяется (4.2!C5).
	isKG := sameExecutor(executor, ExecutorAibiconKG)

	prevCount := 0
	for m := 0; m < duration; m++ {
		q1 := intVal(in.Count1Room, m)
		q2 := intVal(in.Count2Room, m)
		q3 := intVal(in.Count3Room, m)
		count := q1 + q2 + q3

		// Основа: 1кк×цена1 + 2кк×цена2 + 3кк×цена3 (4.2!C22)
		base := in.Price1Room*float64(q1) +
			in.Price2Room*float64(q2) +
			in.Price3Room*float64(q3)

		// Уборка: базовая стоимость × общее количество квартир (4.2!C9),
		// но только в выбранных экономистом месяцах.
		var cleaning float64
		if cleaningEnabled(in, m) {
			cleaning = in.CleaningBase * float64(count)
		}

		// Итого аренда — уборка входит внутрь, отдельной строкой не идёт (4.2!C5)
		total := base + cleaning
		if !isKG {
			total /= npflGrossUpDivisor
		}
		rent[m] = total

		// Риелтор (4.2!C13). Порядок ветвлений важен.
		switch {
		case m == 0:
			// Первый месяц: платим за все квартиры.
			// Ветка проверяется первой, поэтому при duration==1 риелтор
			// начисляется (в форме там был бы 0 из-за пустой BJ13, но
			// проектов длиной 1 месяц не бывает — решение владельца).
			realtor[m] = float64(count) * in.RealtorBase
		case m == duration-1:
			// Последний месяц проекта: риелтор не начисляется — квартиры
			// уже не ищут. В форме ячейка 4.2!BJ13 пустая (формулы нет),
			// это сознательное правило, а не опечатка.
			// Чтобы прирост квартир не потерялся молча, ввод ограничен
			// валидацией ValidateRentApartments.
			realtor[m] = 0
		case count > prevCount:
			// Прирост количества квартир × базовая стоимость риелтора.
			realtor[m] = float64(count-prevCount) * in.RealtorBase
		}

		prevCount = count
	}

	return rent, realtor
}

// ValidateRentApartments проверяет корректность ввода по листу 4.2:
// цены и количества не могут быть отрицательными.
//
// Правило «в последнем месяце квартир не больше, чем в предыдущем»
// сюда сознательно НЕ включено — оно отложено,
// см. docs/deferred_validations.md.
func ValidateRentApartments(in *InputRentApartments) error {
	if in == nil {
		return nil
	}

	for _, p := range []struct {
		name  string
		value float64
	}{
		{"цена аренды 1-комнатной квартиры", in.Price1Room},
		{"цена аренды 2-комнатной квартиры", in.Price2Room},
		{"цена аренды 3-комнатной квартиры", in.Price3Room},
		{"базовая стоимость уборки", in.CleaningBase},
		{"базовая стоимость услуг риелтора", in.RealtorBase},
	} {
		if p.value < 0 {
			return fmt.Errorf("аренда квартир: %s не может быть отрицательной (%.2f)",
				p.name, p.value)
		}
	}

	for _, c := range []struct {
		name   string
		counts []int
	}{
		{"1-комнатных", in.Count1Room},
		{"2-комнатных", in.Count2Room},
		{"3-комнатных", in.Count3Room},
	} {
		for i, v := range c.counts {
			if v < 0 {
				return fmt.Errorf(
					"аренда квартир: количество %s квартир в месяце %d не может быть отрицательным (%d)",
					c.name, i+1, v)
			}
		}
	}

	return nil
}

// ValidateBudgetParams проверяет параметры бюджета перед сохранением.
//
// Главное правило — целевая рентабельность должна быть достижима при
// ставке налога исполнителя. Коэффициент наценки считается как
// E234/(1-F240-E234) (2.Бюджет!F234), поэтому при
// targetRent + ставка налога >= 1 знаменатель обращается в ноль или
// становится отрицательным. В форме это дало бы #DIV/0! или
// отрицательную наценку; в платформе отклоняем ввод с понятной ошибкой,
// а не пропускаем молча.
func ValidateBudgetParams(p *InputBudgetParams, executor string) error {
	if p == nil {
		return nil
	}

	r := p.TargetRentPct / 100
	if r < 0 {
		return fmt.Errorf("целевая рентабельность не может быть отрицательной (%.2f%%)",
			p.TargetRentPct)
	}
	if r == 0 {
		return nil // наценка не применяется (например, задан ТКП)
	}

	t := profitTaxRate(executor)
	if r+t >= 1 {
		return fmt.Errorf(
			"целевая рентабельность слишком высока для этой ставки налога: "+
				"%.2f%% + налог %.0f%% должно быть строго меньше 100%% "+
				"(исполнитель «%s»)",
			p.TargetRentPct, t*100, executor)
	}
	return nil
}

// ValidateEmployees проверяет список сотрудников перед сохранением.
//
// Главное правило — страна НО «Киргизия» допустима ТОЛЬКО у исполнителя
// «Айбикон Киргизия». У остальных исполнителей киргизских сотрудников не
// бывает: взносы Киргизии (2.Бюджет!174) считаются лишь в киргизской
// ветке, и такой сотрудник молча остался бы без страховых взносов вовсе.
func ValidateEmployees(in *InputEmployees, executor string) error {
	if in == nil {
		return nil
	}

	isKGExecutor := sameExecutor(executor, ExecutorAibiconKG)

	for i, e := range in.Employees {
		country := normalizeCountry(e.Country)

		switch country {
		case CountryRF, CountrySelfEmployed:
			// допустимы у любого исполнителя
		case CountryKG:
			if !isKGExecutor {
				return fmt.Errorf(
					"сотрудник %d (%s): страна НО «Киргизия» допустима только "+
						"у исполнителя «Айбикон Киргизия», а у проекта указан «%s»",
					i+1, employeeName(&in.Employees[i]), executor)
			}
		case "":
			return fmt.Errorf("сотрудник %d (%s): не указана страна НО",
				i+1, employeeName(&in.Employees[i]))
		default:
			return fmt.Errorf(
				"сотрудник %d (%s): неизвестная страна НО «%s». Допустимы: "+
					"Россия, Киргизия, «Самозанятый, без НО»",
				i+1, employeeName(&in.Employees[i]), e.Country)
		}

		if e.SalaryNet < 0 {
			return fmt.Errorf("сотрудник %d (%s): план ФОТ на руки не может быть отрицательным",
				i+1, employeeName(&in.Employees[i]))
		}
	}
	return nil
}

// ValidateInput проверяет входные данные одного типа перед сохранением.
// Для типов без собственных правил возвращает nil.
//
// executor нужен для проверок, зависящих от исполнителя (ставка налога),
// duration — для проверок, где месяц должен лежать внутри проекта (лист 4.3).
func ValidateInput(inputType string, raw []byte, executor string, duration int) error {
	switch inputType {
	case TypeRentApartments:
		var v InputRentApartments
		if err := json.Unmarshal(raw, &v); err != nil {
			return fmt.Errorf("аренда квартир: некорректный формат данных: %w", err)
		}
		return ValidateRentApartments(&v)

	case TypeBudgetParams:
		var v InputBudgetParams
		if err := json.Unmarshal(raw, &v); err != nil {
			return fmt.Errorf("параметры бюджета: некорректный формат данных: %w", err)
		}
		return ValidateBudgetParams(&v, executor)

	case TypeEmployees:
		var v InputEmployees
		if err := json.Unmarshal(raw, &v); err != nil {
			return fmt.Errorf("сотрудники: некорректный формат данных: %w", err)
		}
		return ValidateEmployees(&v, executor)

	case TypeTransport:
		var v InputTransport
		if err := json.Unmarshal(raw, &v); err != nil {
			return fmt.Errorf("транспорт: некорректный формат данных: %w", err)
		}
		return ValidateTransport(&v, duration)

	case TypeWagonciks:
		var v InputWagonciks
		if err := json.Unmarshal(raw, &v); err != nil {
			return fmt.Errorf("вагончики: некорректный формат данных: %w", err)
		}
		return ValidateWagonciks(&v, duration)

	case TypeOffice:
		var v InputOffice
		if err := json.Unmarshal(raw, &v); err != nil {
			return fmt.Errorf("офис: некорректный формат данных: %w", err)
		}
		return ValidateOffice(&v)

	case TypeSoftwareItems, TypeSubcontractExtItems, TypeSubcontractGenItems:
		title := costLinesTitle(inputType)
		var v InputCostLines
		if err := json.Unmarshal(raw, &v); err != nil {
			return fmt.Errorf("%s: некорректный формат данных: %w", title, err)
		}
		return ValidateCostLines(title, &v)

	case TypeEquipmentItems, TypeCorporateEventsItems:
		title := purchasesTitle(inputType)
		var v InputPurchases
		if err := json.Unmarshal(raw, &v); err != nil {
			return fmt.Errorf("%s: некорректный формат данных: %w", title, err)
		}
		return ValidatePurchases(title, &v, duration)

	case TypeGphEmployees:
		var v InputGphEmployees
		if err := json.Unmarshal(raw, &v); err != nil {
			return fmt.Errorf("ГПХ сотрудников: некорректный формат данных: %w", err)
		}
		return ValidateGphEmployees(&v)
	}
	return nil
}
