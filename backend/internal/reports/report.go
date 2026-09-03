// Package reports строит БДР (отчёт о доходах и расходах) и БДДС (отчёт о
// движении денежных средств) из результатов расчёта бюджета.
package reports

import (
	"strings"
	"time"

	"ibcon-budget/internal/calc"
)

// Kind — какой из двух отчётов строим.
type Kind string

const (
	KindBDR  Kind = "bdr"
	KindBDDS Kind = "bdds"
)

// Params — контекст проекта, которого нет в результатах расчёта.
type Params struct {
	// StartDate — первый месяц проекта. Нужен, чтобы разложить месяцы по
	// календарю: налог на прибыль платится в конце квартала, а кварталы
	// календарные, а не «каждые три месяца проекта».
	StartDate time.Time
	// ExecutorName — от него зависит сдвиг выручки в БДДС: у киргизского
	// филиала деньги приходят месяцем позже, у российских — в срок.
	ExecutorName string
	// Manual — суммы, введённые руками в самом отчёте.
	Manual ManualValues
}

// ManualValues — ручные значения статей, которые платформа не считает.
type ManualValues struct {
	BDR  map[string][]float64 `json:"bdr"`
	BDDS map[string][]float64 `json:"bdds"`
}

// forKind — ручные значения нужного отчёта.
func (m ManualValues) forKind(k Kind) map[string][]float64 {
	if k == KindBDDS {
		return m.BDDS
	}
	return m.BDR
}

// ctx — всё, что нужно источнику статьи, чтобы посчитать свой месяц.
type ctx struct {
	res    *calc.CalcResult
	params Params
	// n — число месяцев проекта, оно же горизонт отчёта.
	n int
}

// monthDate — календарная дата месяца проекта i (0-based).
func (c *ctx) monthDate(i int) time.Time {
	return monthDate(c.params.StartDate, i)
}

// monthDate — то же для мест, где контекста ещё нет (подписи месяцев).
func monthDate(start time.Time, i int) time.Time {
	return time.Date(start.Year(), start.Month()+time.Month(i), 1, 0, 0, 0, 0, start.Location())
}

// isKG — киргизский исполнитель. Сравнение регистронезависимое, как и
// везде в расчёте.
func (c *ctx) isKG() bool {
	return strings.EqualFold(strings.TrimSpace(c.params.ExecutorName), calc.ExecutorAibiconKG)
}

// source — сколько статья даёт в месяце i (0-based).
type source func(c *ctx, i int) float64

// article — одна строка кодификатора.
type article struct {
	code string
	name string
	// src — nil у групп: их значение собирается из вложенных строк, и у
	// ручных статей, которые платформа не заполняет (их вводят в самом
	// отчёте, см. audit/service_sheets.md — «ручной ввод»).
	src source
}

// Row — строка готового отчёта.
type Row struct {
	Code string `json:"code"`
	Name string `json:"name"`
	// Level — глубина в кодификаторе: 0 у «1», 1 у «1.1», 2 у «2.2.1».
	Level int `json:"level"`
	// Group — строка собирает сумму вложенных, а не имеет своего источника.
	Group bool `json:"group"`
	// Manual — статью платформа не считает, её заполняют руками прямо в
	// отчёте. Интерфейс по этому признаку открывает ячейки на правку.
	Manual bool `json:"manual"`
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
	Months int `json:"months"`
	// MonthLabels — подписи месяцев («янв. 27»), чтобы выгрузка и экран
	// не собирали их каждый по-своему.
	MonthLabels []string `json:"month_labels"`
	Rows        []Row    `json:"rows"`
}

var monthShort = [...]string{
	"янв", "фев", "мар", "апр", "май", "июн",
	"июл", "авг", "сен", "окт", "ноя", "дек",
}

func monthLabel(start time.Time, i int) string {
	d := monthDate(start, i)
	return monthShort[int(d.Month())-1] + ". " + d.Format("06")
}

// codeLevel — глубина кода: «1» → 0, «2.2.1» → 2.
func codeLevel(code string) int { return strings.Count(code, ".") }

// build собирает отчёт из кодификатора и результатов расчёта.
func build(kind Kind, arts []article, res *calc.CalcResult, p Params) *Report {
	c := &ctx{res: res, params: p, n: len(res.Monthly)}

	rep := &Report{
		Kind:        kind,
		Months:      c.n,
		MonthLabels: make([]string, c.n),
		Rows:        make([]Row, 0, len(arts)),
	}
	for i := 0; i < c.n; i++ {
		rep.MonthLabels[i] = monthLabel(p.StartDate, i)
	}

	// Группа — это строка, У КОТОРОЙ ЕСТЬ ПОТОМКИ в кодификаторе, а не
	// просто строка без источника: у ручных статей источника тоже нет, но
	// их значение вводят, а не собирают снизу.
	hasChildren := make(map[string]bool, len(arts))
	for _, a := range arts {
		if i := strings.LastIndex(a.code, "."); i > 0 {
			hasChildren[a.code[:i]] = true
		}
	}

	manual := p.Manual.forKind(kind)
	for _, a := range arts {
		group := a.src == nil && hasChildren[a.code]
		row := Row{
			Code:    a.code,
			Name:    a.name,
			Level:   codeLevel(a.code),
			Group:   group,
			Manual:  a.src == nil && !group,
			Monthly: make([]float64, c.n),
		}
		switch {
		case a.src != nil:
			for i := 0; i < c.n; i++ {
				row.Monthly[i] = a.src(c, i)
			}
		case row.Manual:
			// Введённое руками. Массив может быть короче горизонта (проект
			// продлили после ввода) — недостающие месяцы остаются нулём.
			for i, v := range manual[a.code] {
				if i < c.n {
					row.Monthly[i] = v
				}
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

// rollUp заполняет групповые строки суммой вложенных. Вложенность
// определяется по коду: «2.2.1» собирает строки «2.2.1.…».
func rollUp(rep *Report) {
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
			parent := r.Code[:strings.LastIndex(r.Code, ".")]
			pi, ok := byCode[parent]
			if !ok {
				// Родителя нет в кодификаторе — строка висит сама по себе. В форме такое
				// есть: код «2.06.07» внутри группы 2.2.6, опечатка кодификатора.
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
func Build(kind Kind, res *calc.CalcResult, p Params) *Report {
	if kind == KindBDDS {
		return build(kind, bddsArticles(), res, p)
	}
	return build(KindBDR, bdrArticles(), res, p)
}
