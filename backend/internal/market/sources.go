package market

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Source — площадка объявлений. Каждая отвечает за свой протокол и за
// приведение цены к рублям за месяц; всё остальное — общее.
//
// Ошибка источника не рушит запрос: остальные площадки считаются, а
// упавшая попадает в ответ отдельной строкой. Один недоступный ЦИАН не
// повод не показать цифры по трём другим.
type Source interface {
	Name() string
	Fetch(ctx context.Context, q RentQuery) ([]Observation, error)
}

// Дней в месяце для пересчёта посуточной цены. Ровно 30 — не среднее по
// календарю, а то, как считают сами посуточные площадки в подписи
// «за месяц».
const daysInMonth = 30

// httpClient — общий клиент площадок. Прокси задаётся переменной
// MARKET_PROXY_URL: с адреса дата-центра площадки часто отвечают
// заглушкой, и без выхода через обычного провайдера сбор не работает.
func httpClient() *http.Client {
	tr := &http.Transport{}
	if p := os.Getenv("MARKET_PROXY_URL"); p != "" {
		if u, err := url.Parse(p); err == nil {
			tr.Proxy = http.ProxyURL(u)
		}
	}
	return &http.Client{Timeout: 10 * time.Second, Transport: tr}
}

// Заголовки обычного браузера. Без них площадки отдают страницу-заглушку
// вместо данных.
func setBrowserHeaders(r *http.Request) {
	r.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "+
		"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")
	r.Header.Set("Accept-Language", "ru-RU,ru;q=0.9")
	r.Header.Set("Accept", "application/json, text/html;q=0.9,*/*;q=0.8")
}

func fetchBody(ctx context.Context, method, addr string, body []byte) ([]byte, error) {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, addr, rd)
	if err != nil {
		return nil, err
	}
	setBrowserHeaders(req)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := httpClient().Do(req)
	if err != nil {
		// Сетевую ошибку показываем человеком, а не текстом Go: на экране
		// это читает экономист, а не разработчик.
		return nil, fmt.Errorf("площадка не отвечает")
	}
	defer resp.Body.Close()
	// Ограничение на размер: страница поиска у площадок весит мегабайты,
	// а нужны только цены.
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	switch {
	case resp.StatusCode == http.StatusOK:
	case resp.StatusCode == http.StatusUnauthorized,
		resp.StatusCode == http.StatusForbidden,
		resp.StatusCode == http.StatusTooManyRequests:
		return nil, errClosed
	case resp.StatusCode == http.StatusNotFound:
		return nil, fmt.Errorf("адрес выдачи площадки изменился — сбор не работает")
	default:
		return nil, fmt.Errorf("площадка ответила %d", resp.StatusCode)
	}
	return data, nil
}

// errUnparsed — площадка ответила, но данными это не является: обычно
// вместо выдачи приходит страница проверки на робота. Отдельная ошибка,
// чтобы на экране это читалось не как «сеть недоступна».
var errUnparsed = fmt.Errorf("площадка не отдала объявления автоматическому запросу")

// errClosed — площадка отвечает отказом на запрос без ключа приложения
// или без браузерной сессии. Лечится ключом партнёрского API или
// выходом через MARKET_PROXY_URL, а не правкой кода.
var errClosed = fmt.Errorf("площадка закрыта для автоматических запросов — нужен ключ или прокси")

// ── Яндекс Недвижимость ───────────────────────────────────────────

type yandexSource struct{}

func (yandexSource) Name() string { return "Яндекс Недвижимость" }

func (y yandexSource) Fetch(ctx context.Context, q RentQuery) ([]Observation, error) {
	slug := citySlug(q.City)
	if slug == "" {
		return nil, fmt.Errorf("город «%s» не поддерживается площадкой", q.City)
	}
	addr := fmt.Sprintf("https://realty.yandex.ru/%s/snyat/kvartira/", slug)
	// Число комнат у площадки в адресе словом. Больше четырёх отдельной
	// страницы не имеет — такие запросы идут по всей выдаче города.
	if s, ok := yaRooms[q.Rooms]; ok {
		addr += s + "/"
	}
	// Площадь — фильтром самой площадки: две страницы выдачи по всему
	// городу могли вовсе не содержать квартир нужного размера, и оценка
	// для 200 м² считалась бы по однушкам.
	query := url.Values{}
	if q.Area > 0 {
		lo, hi := areaBand(q.Area, areaBandStart)
		query.Set("areaMin", strconv.Itoa(int(lo)))
		query.Set("areaMax", strconv.Itoa(int(hi)+1))
	}
	// Две страницы выдачи: на одной около полусотни объявлений, а
	// линейная регрессия начинает уступать лесу уже с сорока. Больше не
	// берём — площадку незачем обходить целиком ради оценки уровня цен.
	var out []Observation
	var lastErr error
	for page := 1; page <= 2; page++ {
		params := url.Values{}
		for k, v := range query {
			params[k] = v
		}
		if page > 1 {
			params.Set("page", strconv.Itoa(page))
		}
		addr := addr
		if len(params) > 0 {
			addr += "?" + params.Encode()
		}
		data, err := fetchBody(ctx, http.MethodGet, addr, nil)
		if err != nil {
			lastErr = err
			break
		}
		obs, err := parseYandex(y.Name(), data)
		if err != nil {
			lastErr = err
			break
		}
		out = append(out, obs...)
	}
	if len(out) == 0 {
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, errUnparsed
	}
	return out, nil
}

// parseYandex разбирает объявления из состояния приложения, вшитого в
// страницу выдачи. Цена, комнаты, площадь и этаж лежат в одном объекте
// объявления, поэтому признаки берутся не «в среднем по странице», а от
// конкретного объявления — модели есть на чём учиться.
//
// Опора на форму записи, а не на разметку: разметка у площадки меняется
// от выката к выкату, а поля объявления — редко.
func parseYandex(source string, body []byte) ([]Observation, error) {
	const priceKey = `"price":{"currency":"RUR","value":`
	var out []Observation
	for pos := 0; ; {
		i := bytes.Index(body[pos:], []byte(priceKey))
		if i < 0 {
			break
		}
		i += pos
		valAt := i + len(priceKey)
		price, ok := readNumber(body, valAt)
		pos = valAt
		// Месячная аренда: в той же выдаче попадаются цены за сутки и
		// за метр — они помечены другим периодом.
		if !ok || !bytes.Contains(window(body, valAt, 160), []byte(`"period":"PER_MONTH"`)) {
			continue
		}
		// Смотрим строго НАЗАД от цены: окно, залезающее вперёд, брало бы
		// признаки следующего объявления.
		back := body[max0(i-1200):i]
		o := Observation{Source: source, PriceMonth: price}
		if v, ok := lastNumberFor(back, `"roomsTotal":`); ok {
			o.Rooms = int(v)
		}
		if v, ok := lastNumberFor(back, `"area":{"value":`); ok {
			o.Area = v
		}
		if v, ok := lastNumberFor(back, `"floorsOffered":[`); ok {
			o.Floor = int(v)
		}
		out = append(out, o)
	}
	if len(out) == 0 {
		return nil, errUnparsed
	}
	return out, nil
}

// readNumber читает число, начинающееся в позиции i.
func readNumber(body []byte, i int) (float64, bool) {
	j := i
	for j < len(body) && (body[j] == '.' || (body[j] >= '0' && body[j] <= '9')) {
		j++
	}
	if j == i {
		return 0, false
	}
	v, err := strconv.ParseFloat(string(body[i:j]), 64)
	return v, err == nil
}

// lastNumberFor берёт число у ПОСЛЕДНЕГО вхождения ключа: объявление
// записано перед своей ценой, и ближайшее к ней вхождение — его.
func lastNumberFor(chunk []byte, key string) (float64, bool) {
	i := bytes.LastIndex(chunk, []byte(key))
	if i < 0 {
		return 0, false
	}
	return readNumber(chunk, i+len(key))
}

func window(body []byte, from, n int) []byte {
	if from < 0 {
		from = 0
	}
	to := from + n
	if to > len(body) {
		to = len(body)
	}
	return body[from:to]
}

func max0(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

var yaRooms = map[int]string{
	1: "odnokomnatnaya",
	2: "dvuhkomnatnaya",
	3: "tryohkomnatnaya",
	4: "chetyryohkomnatnaya",
}

// ── 101hotels.com ─────────────────────────────────────────────────

type hotels101Source struct{}

func (hotels101Source) Name() string { return "101hotels.com" }

// Посуточные апартаменты. Нужны не сами по себе: когда объявлений о
// длительной аренде мало, посуточная цена всё равно показывает уровень
// рынка в городе — модель учитывает разницу отдельным признаком.
func (h hotels101Source) Fetch(ctx context.Context, q RentQuery) ([]Observation, error) {
	slug := citySlug(q.City)
	if slug == "" {
		return nil, fmt.Errorf("город «%s» не поддерживается площадкой", q.City)
	}
	data, err := fetchBody(ctx, http.MethodGet,
		"https://101hotels.com/main/cities/"+slug+"/apartments", nil)
	if err != nil {
		return nil, err
	}
	return parse101(h.Name(), data)
}

// parse101 читает цены из разметки карточек: цена вынесена в атрибут
// data-price-value рядом с валютой, и это устойчивее, чем разбирать
// подпись «от 7 215,19 руб.» с пробелами и запятой.
func parse101(source string, body []byte) ([]Observation, error) {
	const key = `data-price-value="`
	var out []Observation
	for pos := 0; ; {
		i := bytes.Index(body[pos:], []byte(key))
		if i < 0 {
			break
		}
		i += pos
		pos = i + len(key)
		price, ok := readNumber(body, pos)
		if !ok {
			continue
		}
		// Суточная цена — в месяц. Признак Daily остаётся: без него
		// месячная оценка уехала бы вслед за суточной ценой.
		month := price * daysInMonth
		if month < 5000 || month > 5_000_000 {
			continue
		}
		out = append(out, Observation{Source: source, PriceMonth: month, Daily: true})
	}
	if len(out) == 0 {
		return nil, errUnparsed
	}
	return out, nil
}

// citySlug — адрес города в Яндекс Недвижимости. Список короткий
// намеренно: у площадки свои написания, и угаданный транслитом адрес
// отдал бы страницу другого города, а не ошибку.
var yaSlugs = map[string]string{
	"москва":          "moskva",
	"санкт-петербург": "sankt-peterburg",
	"екатеринбург":    "ekaterinburg",
	"новосибирск":     "novosibirsk",
	"казань":          "kazan",
	"нижний новгород": "nizhniy-novgorod",
	"краснодар":       "krasnodar",
	"ростов-на-дону":  "rostov-na-donu",
	"самара":          "samara",
	"уфа":             "ufa",
	"тюмень":          "tyumen",
	"мурманск":        "murmansk",
	"владивосток":     "vladivostok",
}

func citySlug(city string) string {
	return yaSlugs[strings.ToLower(strings.TrimSpace(city))]
}

func median(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	c := append([]float64(nil), v...)
	sortFloats(c)
	return percentile(c, 50)
}
