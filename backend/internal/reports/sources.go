package reports

import (
	"time"

	"ibcon-budget/internal/calc"
)

// Источники статей: откуда берётся сумма месяца.

// vatMultiplier — НДС. Пять статей БДДС берутся с НДС, потому что
// платятся поставщикам с ним (audit/service_sheets.md, БДДС п.1).
const vatMultiplier = 1.22

// ohIdx — индекс строки 2.Бюджет 178…211 в массиве Overhead.
func ohIdx(row int) int { return row - 178 }

// oh — накладная строка 2.Бюджет своим месяцем.
func oh(row int) source {
	i := ohIdx(row)
	return func(c *ctx, m int) float64 { return c.res.Monthly[m].Overhead[i] }
}

// ohVAT — то же с НДС: статья платится поставщику с налогом.
func ohVAT(row int) source {
	i := ohIdx(row)
	return func(c *ctx, m int) float64 { return c.res.Monthly[m].Overhead[i] * vatMultiplier }
}

// ohSum — несколько строк 2.Бюджет в одной статье отчёта.
func ohSum(rows ...int) source {
	idx := make([]int, len(rows))
	for i, r := range rows {
		idx[i] = ohIdx(r)
	}
	return func(c *ctx, m int) float64 {
		var t float64
		for _, i := range idx {
			t += c.res.Monthly[m].Overhead[i]
		}
		return t
	}
}

// fld — поле месячного итога (не накладная строка).
func fld(f func(m *calc.MonthlyResult) float64) source {
	return func(c *ctx, i int) float64 { return f(&c.res.Monthly[i]) }
}

// prevMonth — значение ПРЕДЫДУЩЕГО месяца: платёж сдвинут на месяц вперёд. В
// первом месяце проекта платить нечего — ноль.
func prevMonth(s source) source {
	return func(c *ctx, i int) float64 {
		if i == 0 {
			return 0
		}
		return s(c, i-1)
	}
}

// halfAndHalf — платёж двумя частями: половина за текущий месяц и половина
// за предыдущий. В первом месяце проекта — только своя половина, второй
// половине неоткуда взяться.
func halfAndHalf(s source) source {
	return func(c *ctx, i int) float64 {
		v := s(c, i) / 2
		if i > 0 {
			v += s(c, i-1) / 2
		}
		return v
	}
}

// shiftIfKG — сдвиг на месяц только у киргизского исполнителя.
// У российских деньги приходят своим месяцем (audit/service_sheets.md:
// «то же правило для выручки КГ — не распространяется на РФ»).
func shiftIfKG(s source) source {
	shifted := prevMonth(s)
	return func(c *ctx, i int) float64 {
		if c.isKG() {
			return shifted(c, i)
		}
		return s(c, i)
	}
}

// ── Налог на прибыль ────────────────────────────────────────────────────
//
// Налог не начисляется помесячно: он платится раз в квартал от суммарной
// операционной прибыли этого квартала (решение владельца 2026-08-29):
//
//	БДР  — в последнем месяце квартала: март, июнь, сентябрь, декабрь;
//	БДДС — месяцем позже: апрель, июль, октябрь, а за IV квартал в марте
//	       следующего года (годовой расчёт).
//
// Кварталы КАЛЕНДАРНЫЕ, а не «каждые три месяца проекта»: проект,
// начатый в феврале, первый платёж делает уже в марте — за февраль и
// март, а не через три месяца.

// quarterOf — номер календарного квартала (1..4) месяца m.
func quarterOf(m time.Month) int { return (int(m)-1)/3 + 1 }

// quarterProfit — операционная прибыль календарного квартала q года y,
// сложенная по тем месяцам проекта, которые в этот квартал попали.
// Месяцы вне проекта в сумму не входят: их прибыль неизвестна.
func quarterProfit(c *ctx, year, q int) float64 {
	var t float64
	for i := 0; i < c.n; i++ {
		d := c.monthDate(i)
		if d.Year() == year && quarterOf(d.Month()) == q {
			t += c.res.Monthly[i].OperatingProfit
		}
	}
	return t
}

// profitTaxAt — налог, уплачиваемый в месяце i.
func profitTaxAt(payMonth func(m time.Month) (q, yearOffset int, ok bool)) source {
	return func(c *ctx, i int) float64 {
		d := c.monthDate(i)
		q, off, ok := payMonth(d.Month())
		if !ok {
			return 0
		}
		return quarterProfit(c, d.Year()+off, q) * c.res.ProfitTaxRate
	}
}

// taxBDR — платёж в последнем месяце своего квартала.
var taxBDR = profitTaxAt(func(m time.Month) (int, int, bool) {
	switch m {
	case time.March, time.June, time.September, time.December:
		return quarterOf(m), 0, true
	}
	return 0, 0, false
})

// taxBDDS — платёж месяцем позже: за I квартал в апреле, за II в июле,
// за III в октябре, за IV — в марте СЛЕДУЮЩЕГО года (yearOffset −1
// означает «квартал прошлого года»).
var taxBDDS = profitTaxAt(func(m time.Month) (int, int, bool) {
	switch m {
	case time.April:
		return 1, 0, true
	case time.July:
		return 2, 0, true
	case time.October:
		return 3, 0, true
	case time.March:
		return 4, -1, true
	}
	return 0, 0, false
})
