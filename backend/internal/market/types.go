// Package market — рыночная стоимость аренды квартир.
//
// Экономист заполняет лист 4.2 «Аренда квартир» по памяти или по
// сохранённым где-то объявлениям. Здесь платформа собирает объявления с
// публичных площадок сама, приводит их к одному виду — рубли за месяц —
// и считает по ним оценку для города и параметров, которые ввёл человек.
//
// Оценка НИКОГДА не додумывается: если площадки ничего не отдали, ответ
// так и говорит — данных нет. Придуманная цифра в бюджете хуже, чем её
// отсутствие.
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
	// Этаж. 0 — не задано.
	Floor int `json:"floor"`
	// Лифт: nil — не задано, иначе есть/нет.
	Elevator *bool `json:"elevator"`
	// Пешком до метро, минут. 0 — не задано.
	MetroMinutes int `json:"metro_minutes"`
}

// Observation — одно объявление, приведённое к рублям за месяц.
type Observation struct {
	Source string `json:"source"`
	// Цена в рублях за месяц. Посуточные площадки пересчитываются
	// умножением на 30 — и помечаются флагом Daily, потому что суточная
	// цена систематически выше месячной и модель это учитывает
	// отдельным признаком.
	PriceMonth float64 `json:"price_month"`
	Daily      bool    `json:"daily"`

	Rooms        int     `json:"rooms,omitempty"`
	Area         float64 `json:"area,omitempty"`
	Floor        int     `json:"floor,omitempty"`
	Elevator     *bool   `json:"elevator,omitempty"`
	MetroMinutes int     `json:"metro_minutes,omitempty"`
	District     string  `json:"district,omitempty"`
	Title        string  `json:"title,omitempty"`
	URL          string  `json:"url,omitempty"`
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
	// Считается только ДЛИТЕЛЬНАЯ аренда: посуточная цена, пересчитанная
	// в месяц, к помесячной ставке отношения не имеет — решение
	// владельца 2026-09-03.
	Sample int `json:"sample"`
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
	// перцентилем. Сам 95-й перцентиль — это почти самое дорогое
	// объявление на рынке, закладывать его в бюджет значит переплатить;
	// среднее по той же выборке без верхних пяти процентов даёт цену, по
	// которой квартиру действительно снимают.
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
