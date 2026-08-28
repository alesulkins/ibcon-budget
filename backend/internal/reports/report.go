// Package reports строит БДР (отчёт о доходах и расходах) и БДДС (отчёт о
// движении денежных средств) из результатов расчёта бюджета.
//
// Разбор эталонных листов — audit/service_sheets.md.
//
// Как устроено. Оба отчёта — плоский список статей с иерархическим
// кодификатором («1», «2.2.1», «2.2.1.01»). Уровень читается из самого
// кода, а групповые суммы собираются по префиксу кода: строка «2.2.1»
// равна сумме всех строк, чей код начинается с «2.2.1.». Отдельного
// дерева не строим — кодификатор УЖЕ дерево, а два представления одного
// и того же расходились бы при правках.
//
// В форме агрегация сделана через SUMPRODUCT по календарной дате
// колонки, а не по номеру месяца. Здесь это не нужно: расчёт и так
// возвращает помесячный массив в порядке месяцев проекта.
package reports

import (
	"strings"

	"ibcon-budget/internal/calc"
)

// Kind — какой из двух отчётов строим.
type Kind string

const (
	KindBDR  Kind = "bdr"
	KindBDDS Kind = "bdds"
)

// source — откуда статья берёт значение месяца.
//
// m — итоги месяца, res — итоги всего проекта (нужны там, где в форме
// значение не разложено по месяцам, например налог на прибыль).
type source func(m *calc.MonthlyResult, res *calc.CalcResult) float64

// article — одна строка кодификатора.
type article struct {
	code string
	name string
	// src — nil у групп: их значение собирается из вложенных строк.
	src source
}

// Row — строка готового отчёта.
type Row struct {
	Code string `json:"code"`
	Name string `json:"name"`
	// Level — глубина в кодификаторе: 0 у «1», 1 у «1.1», 2 у «2.2.1»…
	Level int `json:"level"`
	// Group — строка собирает сумму вложенных, а не имеет своего источника.
	Group bool `json:"group"`
	// Monthly — значения по месяцам проекта, индекс 0 = первый месяц.
	Monthly []float64 `json:"monthly"`
	Total   float64   `json:"total"`
}

// Report — готовый отчёт.
type Report struct {
	Kind Kind `json:"kind"`
	// Months — сколько месяцев в отчёте. Ровно длительность проекта:
	// жёстких 12 месяцев формы здесь нет (audit/service_sheets.md,
	// находка БДР-B), иначе проект длиннее года обрезался бы молча.
	Months int   `json:"months"`
	Rows   []Row `json:"rows"`
}

// codeLevel — глубина кода: «1» → 0, «2.2.1» → 2.
func codeLevel(code string) int {
	return strings.Count(code, ".")
}

// build собирает отчёт из кодификатора и результатов расчёта.
func build(kind Kind, arts []article, res *calc.CalcResult) *Report {
	n := len(res.Monthly)
	rep := &Report{Kind: kind, Months: n, Rows: make([]Row, 0, len(arts))}

	for _, a := range arts {
		row := Row{
			Code:    a.code,
			Name:    a.name,
			Level:   codeLevel(a.code),
			Group:   a.src == nil,
			Monthly: make([]float64, n),
		}
		if a.src != nil {
			for m := 0; m < n; m++ {
				row.Monthly[m] = a.src(&res.Monthly[m], res)
			}
		}
		rep.Rows = append(rep.Rows, row)
	}

	rollUp(rep)
	for i := range rep.Rows {
		for _, v := range rep.Rows[i].Monthly {
			rep.Rows[i].Total += v
		}
	}
	return rep
}

// rollUp заполняет групповые строки суммой вложенных.
//
// Вложенность определяется по коду: «2.2.1» собирает все строки с кодом
// «2.2.1.…». Считается по прямым потомкам — иначе внуки попали бы в сумму
// дважды, через себя и через своего родителя.
func rollUp(rep *Report) {
	// От самых глубоких к самым мелким: к моменту, когда доходим до
	// родителя, все его дети уже посчитаны.
	byCode := make(map[string]int, len(rep.Rows))
	for i, r := range rep.Rows {
		byCode[r.Code] = i
	}

	maxLevel := 0
	for _, r := range rep.Rows {
		if r.Level > maxLevel {
			maxLevel = r.Level
		}
	}

	for level := maxLevel; level >= 1; level-- {
		for i := range rep.Rows {
			r := &rep.Rows[i]
			if r.Level != level {
				continue
			}
			parentCode := r.Code[:strings.LastIndex(r.Code, ".")]
			pi, ok := byCode[parentCode]
			if !ok {
				// Родителя нет в кодификаторе — строка висит сама по себе.
				// В форме такое есть: код «2.06.07» внутри группы 2.2.6,
				// опечатка кодификатора. Строку показываем, в сумму
				// группы она не входит — ровно как в форме.
				continue
			}
			if !rep.Rows[pi].Group {
				continue // у родителя свой источник, суммировать не нужно
			}
			for m := range r.Monthly {
				rep.Rows[pi].Monthly[m] += r.Monthly[m]
			}
		}
	}
}

// Build — публичная точка входа.
func Build(kind Kind, res *calc.CalcResult) *Report {
	switch kind {
	case KindBDDS:
		return build(kind, bddsArticles(), res)
	default:
		return build(KindBDR, bdrArticles(), res)
	}
}
