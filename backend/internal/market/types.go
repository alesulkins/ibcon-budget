// Package market — рыночная стоимость аренды квартир. Экономист заполняет
// лист 4.2 «Аренда квартир» по памяти или по сохранённым где-то объявлениям.
package market

import "time"

// RentQuery — что спрашивает пользователь. Обязателен только город:
// остальное уточняет оценку, но без него она тоже считается — по всей
// выборке города.
type RentQuery struct {
	City string `json:"city" binding:"required"`

	District string `json:"district"`
	// Комнат: 0 — не задано, 1..N — сколько комнат. Студия — 1.
	Rooms int `json:"rooms"`
	// Площадь, м². 0 — не задано.
	Area float64 `json:"area"`
}

// Этажа, лифта и расстояния до метро в запросе нет: у большинства
// объявлений этих полей не бывает, отбор по ним схлопывал выборку до
// единиц, а оценку строить не на чем (решение владельца 2026-09-03).

// Observation — одно объявление, приведённое к рублям за месяц.
type Observation struct {
	Source string `json:"source"`
	// Цена в рублях за месяц.
	PriceMonth float64 `json:"price_month"`
	Daily      bool    `json:"daily"`

	Rooms    int     `json:"rooms,omitempty"`
	Area     float64 `json:"area,omitempty"`
	Floor    int     `json:"floor,omitempty"`
	District string  `json:"district,omitempty"`
	Title    string  `json:"title,omitempty"`
	URL      string  `json:"url,omitempty"`
}

// Bin — столбик гистограммы: сколько объявлений попало в диапазон цен.
type Bin struct {
	From  float64 `json:"from"`
	To    float64 `json:"to"`
	Count int     `json:"count"`
}

// SourceStatus — что ответила площадка. Отдаётся на экран целиком:
// человек должен видеть, на чём основана оценка и чего в ней нет.
type SourceStatus struct {
	Source string `json:"source"`
	Count  int    `json:"count"`
	// Медиана по площадке, ₽/мес. 0, если объявлений нет.
	Median float64 `json:"median"`
	// Пусто, если площадка ответила. Иначе — почему не ответила.
	Error string `json:"error,omitempty"`
}

// Estimate — ответ платформы.
type Estimate struct {
	Query RentQuery `json:"query"`

	// Сколько объявлений легло в расчёт после отбраковки выбросов.
	Sample int `json:"sample"`
	// По каким параметрам отобраны объявления и что пришлось ослабить —
	// строкой для экрана. Пусто, если отбирать было не по чему.
	Matched string `json:"matched"`
	// Какая модель считала прогноз и почему — см. chooseModel.
	Model       string `json:"model"`
	ModelReason string `json:"model_reason"`

	// Прогноз модели для введённых параметров, ₽/мес. 0 — модель не
	// строилась (мало данных или не заданы признаки).
	Predicted float64 `json:"predicted"`
	// Средняя ошибка прогноза на отложенной части выборки, ₽/мес.
	MAE float64 `json:"mae"`

	// Перцентили выборки, ₽/мес.
	P50 float64 `json:"p50"`
	P95 float64 `json:"p95"`

	// Что закладывать в бюджет: СРЕДНЕЕ по выборке, обрезанной 95-м
	// перцентилем.
	Recommended float64 `json:"recommended"`

	// Распределение цен: столбики гистограммы для графика на экране.
	Histogram []Bin `json:"histogram"`

	Sources []SourceStatus `json:"sources"`
	// Несколько объявлений, на которых видно, из чего сложилась цифра.
	Examples []Observation `json:"examples"`

	CalculatedAt time.Time `json:"calculated_at"`
	// Ответ отдан из кэша, а не запрошен заново.
	Cached bool `json:"cached"`
}
