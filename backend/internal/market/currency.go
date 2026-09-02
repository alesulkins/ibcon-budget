package market

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

/*
Курс валют для площадок, которые называют цену не в рублях.

Бюджет считается в рублях, а киргизские объявления стоят в сомах.
Пересчёт идёт по официальному курсу Банка России: свой коэффициент в
коде — это скрытое допущение, из-за которого цифра в бюджете разошлась бы
с бухгалтерией на десятки процентов.

Курса нет — оценки по такой площадке тоже нет: показать сомы под
подписью «₽/мес» хуже, чем не показать ничего.
*/

// Курс ЦБ обновляется раз в сутки, поэтому и держим его сутки.
const rateTTL = 24 * time.Hour

const cbrDailyURL = "https://www.cbr-xml-daily.ru/daily_json.js"

type rateCache struct {
	mu    sync.Mutex
	at    time.Time
	rates map[string]float64
}

var rates = &rateCache{}

// rubPer возвращает, сколько рублей стоит одна единица валюты.
func rubPer(ctx context.Context, code string) (float64, error) {
	rates.mu.Lock()
	if time.Since(rates.at) < rateTTL {
		v, ok := rates.rates[code]
		rates.mu.Unlock()
		if !ok {
			return 0, fmt.Errorf("курс %s неизвестен", code)
		}
		return v, nil
	}
	rates.mu.Unlock()

	data, err := fetchBody(ctx, http.MethodGet, cbrDailyURL, nil)
	if err != nil {
		return 0, fmt.Errorf("курс валют недоступен")
	}
	var res struct {
		Valute map[string]struct {
			Nominal float64 `json:"Nominal"`
			Value   float64 `json:"Value"`
		} `json:"Valute"`
	}
	if err := json.Unmarshal(data, &res); err != nil || len(res.Valute) == 0 {
		return 0, fmt.Errorf("курс валют не разобран")
	}

	parsed := make(map[string]float64, len(res.Valute))
	for c, v := range res.Valute {
		// Номинал: сом Банк России котирует за сотню, доллар — за штуку.
		if v.Nominal > 0 {
			parsed[c] = v.Value / v.Nominal
		}
	}
	rates.mu.Lock()
	rates.rates = parsed
	rates.at = time.Now()
	rates.mu.Unlock()

	v, ok := parsed[code]
	if !ok {
		return 0, fmt.Errorf("курс %s неизвестен", code)
	}
	return v, nil
}
