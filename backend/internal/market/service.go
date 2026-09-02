package market

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Сколько живёт готовый ответ. Цены на аренду за час не меняются, а
// площадки блокируют частые запросы: один и тот же город в течение
// смены спрашивают из кэша.
const cacheTTL = 6 * time.Hour

// Сколько ждём все площадки вместе. Дольше держать экран нельзя, а
// медленная площадка не должна задерживать быстрые: недоступную видно
// строкой в ответе, и ждать её полминуты незачем.
const fetchTimeout = 12 * time.Second

type Service struct {
	sources []Source

	mu    sync.Mutex
	cache map[string]cacheEntry
}

type cacheEntry struct {
	at  time.Time
	est Estimate
}

// DefaultSources — площадки, которые отвечают на запрос платформы и
// отдают ПОМЕСЯЧНУЮ аренду.
//
// ЦИАН, Суточно.ру и Ostrovok убраны 2026-09-02: первый отдаёт 403 без
// браузерной сессии, второй требует ключ приложения, третий не отвечает
// вовсе. 101hotels.com убран 2026-09-03: он посуточный, а посуточные
// цены в расчёт не идут — адаптер оставлен на случай, если решение
// изменится. Вернутся, когда появится партнёрский доступ.
func DefaultSources() []Source {
	return []Source{yandexSource{}}
}

func NewService(sources ...Source) *Service {
	if len(sources) == 0 {
		sources = DefaultSources()
	}
	return &Service{sources: sources, cache: map[string]cacheEntry{}}
}

func (s *Service) Estimate(ctx context.Context, q RentQuery) (*Estimate, error) {
	q.City = strings.TrimSpace(q.City)
	if q.City == "" {
		return nil, fmt.Errorf("город обязателен")
	}
	key := cacheKey(q)

	s.mu.Lock()
	if e, ok := s.cache[key]; ok && time.Since(e.at) < cacheTTL {
		s.mu.Unlock()
		out := e.est
		out.Cached = true
		return &out, nil
	}
	s.mu.Unlock()

	obs, statuses := s.collect(ctx, q)
	est := build(q, obs, statuses)

	// В кэш кладём и пустой ответ: если площадки закрылись, повторять
	// поход к ним на каждое нажатие бессмысленно.
	s.mu.Lock()
	s.cache[key] = cacheEntry{at: time.Now(), est: *est}
	s.mu.Unlock()
	return est, nil
}

// collect опрашивает площадки одновременно: последовательный обход
// упирался бы в сумму их задержек.
func (s *Service) collect(ctx context.Context, q RentQuery) ([]Observation, []SourceStatus) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	type result struct {
		obs []Observation
		st  SourceStatus
	}
	out := make([]result, len(s.sources))
	var wg sync.WaitGroup
	for i, src := range s.sources {
		wg.Add(1)
		go func(i int, src Source) {
			defer wg.Done()
			st := SourceStatus{Source: src.Name()}
			obs, err := src.Fetch(ctx, q)
			if err != nil {
				st.Error = err.Error()
				out[i] = result{st: st}
				return
			}
			st.Count = len(obs)
			st.Median = median(prices(obs))
			out[i] = result{obs: obs, st: st}
		}(i, src)
	}
	wg.Wait()

	var all []Observation
	statuses := make([]SourceStatus, 0, len(out))
	for _, r := range out {
		all = append(all, r.obs...)
		statuses = append(statuses, r.st)
	}
	return all, statuses
}

// build считает оценку по собранным объявлениям.
func build(q RentQuery, obs []Observation, statuses []SourceStatus) *Estimate {
	est := &Estimate{Query: q, Sources: statuses, CalculatedAt: time.Now()}

	// Только помесячная аренда. Посуточная цена, умноженная на 30, — это
	// не ставка по договору найма, и в расчёт она не идёт вовсе
	// (решение владельца 2026-09-03).
	obs = trimOutliers(longTerm(obs))
	est.Sample = len(obs)
	if len(obs) == 0 {
		est.ModelReason = "объявлений о помесячной аренде нет — оценивать нечего"
		return est
	}

	p := prices(obs)
	sortFloats(p)
	est.P50 = round2(percentile(p, 50))
	est.P95 = round2(percentile(p, 95))
	// В бюджет — среднее по выборке без верхних пяти процентов. Сам 95-й
	// перцентиль это почти самое дорогое предложение рынка: заложив его,
	// проект переплатит за каждую квартиру.
	est.Recommended = round2(meanBelow(p, percentile(p, 95)))
	est.Histogram = histogram(p)

	// Сид фиксирован: одинаковый запрос должен давать одинаковый ответ,
	// иначе две соседние попытки дают разные цифры и цифрам не верят.
	if f := fitModel(obs, 20260902); f != nil {
		est.Model = f.name
		est.ModelReason = f.reason
		est.MAE = round2(f.mae)
		est.Predicted = round2(f.model.predict(f.im.apply(queryFeatures(q))))
		if est.Predicted < 0 {
			// Отрицательная цена — признак того, что параметры далеко за
			// пределами выборки. Показываем перцентили, прогноз прячем.
			est.Predicted = 0
		}
	} else {
		est.ModelReason = "объявлений слишком мало для модели — только перцентили"
	}

	est.Examples = examples(obs)
	return est
}

// meanBelow — среднее по значениям не выше границы. Отсечённые пять
// процентов — это верхние выбросы рынка: премиальные квартиры и
// объявления с завышенной ценой, которые месяцами висят несданными.
func meanBelow(sorted []float64, limit float64) float64 {
	sum, n := 0.0, 0
	for _, v := range sorted {
		if v <= limit {
			sum += v
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

// Сколько столбиков в графике распределения. Двенадцать — читаемо и на
// узком экране, и на полусотне объявлений: больше превращается в частокол
// из единиц, меньше — в три ступеньки, по которым разброс не виден.
const histogramBins = 12

// histogram раскладывает цены по равным диапазонам.
func histogram(sorted []float64) []Bin {
	if len(sorted) == 0 {
		return nil
	}
	lo, hi := sorted[0], sorted[len(sorted)-1]
	if hi <= lo {
		return []Bin{{From: lo, To: hi, Count: len(sorted)}}
	}
	step := (hi - lo) / histogramBins
	bins := make([]Bin, histogramBins)
	for i := range bins {
		bins[i] = Bin{From: round2(lo + step*float64(i)), To: round2(lo + step*float64(i+1))}
	}
	for _, v := range sorted {
		i := int((v - lo) / step)
		if i >= histogramBins {
			i = histogramBins - 1 // самое дорогое объявление — в последний столбик
		}
		bins[i].Count++
	}
	return bins
}

// trimOutliers убирает объявления с ценой за полтора межквартильных
// размаха от квартилей. На досках объявлений всегда есть и «квартира за
// 1 ₽», и месячная цена, проставленная как суточная: без отбраковки они
// утащат и среднее, и 95-й перцентиль.
func trimOutliers(obs []Observation) []Observation {
	if len(obs) < 8 {
		return obs
	}
	p := prices(obs)
	sortFloats(p)
	q1, q3 := percentile(p, 25), percentile(p, 75)
	iqr := q3 - q1
	lo, hi := q1-1.5*iqr, q3+1.5*iqr
	out := make([]Observation, 0, len(obs))
	for _, o := range obs {
		if o.PriceMonth >= lo && o.PriceMonth <= hi {
			out = append(out, o)
		}
	}
	return out
}

// examples — по одному-двум объявлениям с каждой площадки, ближе к
// середине цены: крайние объявления производят ложное впечатление.
//
// Показываем только те, где заполнено всё, что выведено в таблице
// примеров: цена, комнаты и площадь. Пример с прочерками вместо
// половины полей ничего не подтверждает.
func examples(obs []Observation) []Observation {
	bySource := map[string][]Observation{}
	for _, o := range obs {
		if o.PriceMonth <= 0 || o.Rooms <= 0 || o.Area <= 0 {
			continue
		}
		bySource[o.Source] = append(bySource[o.Source], o)
	}
	var out []Observation
	for _, list := range bySource {
		sort.Slice(list, func(i, j int) bool { return list[i].PriceMonth < list[j].PriceMonth })
		mid := len(list) / 2
		out = append(out, list[mid])
		if len(list) > 4 {
			out = append(out, list[mid/2])
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Source < out[j].Source })
	return out
}

// longTerm — объявления о помесячной аренде. Посуточные площадки в
// список источников сейчас не входят, но фильтр остаётся: он и есть то
// место, где принято решение «в расчёт идёт только помесячная».
func longTerm(obs []Observation) []Observation {
	out := make([]Observation, 0, len(obs))
	for _, o := range obs {
		if !o.Daily {
			out = append(out, o)
		}
	}
	return out
}

func prices(obs []Observation) []float64 {
	out := make([]float64, len(obs))
	for i, o := range obs {
		out[i] = o.PriceMonth
	}
	return out
}

func sortFloats(v []float64) { sort.Float64s(v) }

func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}

func cacheKey(q RentQuery) string {
	lift := "?"
	if q.Elevator != nil {
		lift = fmt.Sprint(*q.Elevator)
	}
	return fmt.Sprintf("%s|%s|%d|%.1f|%d|%s|%d",
		strings.ToLower(q.City), strings.ToLower(q.District),
		q.Rooms, q.Area, q.Floor, lift, q.MetroMinutes)
}
