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

		// Уборка: базовая стоимость × общее количество квартир (4.2!C9)
		cleaning := in.CleaningBase * float64(count)

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

// ValidateInput проверяет входные данные одного типа перед сохранением.
// Для типов без собственных правил возвращает nil.
func ValidateInput(inputType string, raw []byte) error {
	switch inputType {
	case TypeRentApartments:
		var v InputRentApartments
		if err := json.Unmarshal(raw, &v); err != nil {
			return fmt.Errorf("аренда квартир: некорректный формат данных: %w", err)
		}
		return ValidateRentApartments(&v)
	}
	return nil
}
