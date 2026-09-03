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
	"sync"
	"time"
)

// Source — площадка объявлений. Каждая отвечает за свой протокол и за
// приведение цены к рублям за месяц; всё остальное — общее.
type Source interface {
	Name() string
	// Supports — работает ли площадка по этому городу.
	Supports(q RentQuery) bool
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

func (yandexSource) Supports(q RentQuery) bool { return citySlug(q.City) != "" }

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
	// Выдача берётся вширь: чем больше подходящих квартир, тем устойчивее
	// медиана. Страницы запрашиваются одновременно — последовательный
	// обход шести страниц не уложился бы в отведённое на запрос время.
	type pageResult struct {
		obs []Observation
		err error
	}
	results := make([]pageResult, yaPages)
	var wg sync.WaitGroup
	for page := 1; page <= yaPages; page++ {
		wg.Add(1)
		go func(page int) {
			defer wg.Done()
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
				results[page-1] = pageResult{err: err}
				return
			}
			obs, err := parseYandex(y.Name(), data)
			results[page-1] = pageResult{obs: obs, err: err}
		}(page)
	}
	wg.Wait()

	var out []Observation
	var firstErr error
	for _, r := range results {
		if r.err != nil {
			if firstErr == nil {
				firstErr = r.err
			}
			continue
		}
		out = append(out, r.obs...)
	}
	out = dedupe(out)
	if len(out) == 0 {
		if firstErr != nil {
			return nil, firstErr
		}
		return nil, errUnparsed
	}
	return out, nil
}

// Сколько страниц выдачи берём. Шесть — около трёхсот объявлений: этого
// хватает и после отбора по площади, а дальше площадка начинает отдавать
// всё более далёкие от запроса варианты.
const yaPages = 6

// dedupe убирает повторы. Страницы запрашиваются одновременно, и если
// площадка когда-нибудь перестанет понимать номер страницы, одно и то же
// объявление попало бы в выборку шесть раз и перекосило медиану.
func dedupe(obs []Observation) []Observation {
	type key struct {
		price float64
		area  float64
		rooms int
		floor int
	}
	seen := map[key]bool{}
	out := make([]Observation, 0, len(obs))
	for _, o := range obs {
		k := key{o.PriceMonth, o.Area, o.Rooms, o.Floor}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, o)
	}
	return out
}

// parseYandex разбирает объявления из состояния приложения, вшитого в
// страницу выдачи.
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

// citySlug — адрес города в Яндекс Недвижимости.
var yaSlugs = map[string]string{
	"анадырь":         "anadyr",
	"апатиты":         "apatity",
	"белоярский":      "beloyarskiy",
	"воркута":         "vorkuta",
	"губкинский":      "gubkinskiy",
	"лабытнанги":      "labytnangi",
	"лангепас":        "langepas",
	"магадан":         "magadan",
	"мегион":          "megion",
	"муравленко":      "muravlenko",
	"надым":           "nadym",
	"новый уренгой":   "novyy_urengoy",
	"пыть-ях":         "pyt-yah",
	"радужный":        "raduzhnyy",
	"салехард":        "salehard",
	"тарко-сале":      "tarko-sale",
	"архангельск":     "arhangelsk",
	"астрахань":       "astrahan",
	"балашиха":        "balashiha",
	"барнаул":         "barnaul",
	"белгород":        "belgorod",
	"брянск":          "bryansk",
	"владивосток":     "vladivostok",
	"владикавказ":     "vladikavkaz",
	"владимир":        "vladimir",
	"волгоград":       "volgograd",
	"волжский":        "volzhskiy",
	"вологда":         "vologda",
	"воронеж":         "voronezh",
	"екатеринбург":    "ekaterinburg",
	"иваново":         "ivanovo",
	"ижевск":          "izhevsk",
	"иркутск":         "irkutsk",
	"йошкар-ола":      "yoshkar-ola",
	"казань":          "kazan",
	"калининград":     "kaliningrad",
	"калуга":          "kaluga",
	"кемерово":        "kemerovo",
	"когалым":         "kogalym",
	"кострома":        "kostroma",
	"краснодар":       "krasnodar",
	"красноярск":      "krasnoyarsk",
	"курган":          "kurgan",
	"курск":           "kursk",
	"магнитогорск":    "magnitogorsk",
	"махачкала":       "mahachkala",
	"москва":          "moskva",
	"мурманск":        "murmansk",
	"нефтеюганск":     "nefteyugansk",
	"нижневартовск":   "nizhnevartovsk",
	"нижний новгород": "nizhniy_novgorod",
	"новороссийск":    "novorossiysk",
	"новосибирск":     "novosibirsk",
	"норильск":        "norilsk",
	"ноябрьск":        "noyabrsk",
	"нягань":          "nyagan",
	"омск":            "omsk",
	"оренбург":        "orenburg",
	"пенза":           "penza",
	"пермь":           "perm",
	"петрозаводск":    "petrozavodsk",
	"петропавловск-камчатский": "petropavlovsk-kamchatskiy",
	"ростов-на-дону":           "rostov-na-donu",
	"рязань":                   "ryazan",
	"самара":                   "samara",
	"санкт-петербург":          "sankt-peterburg",
	"саранск":                  "saransk",
	"саратов":                  "saratov",
	"севастополь":              "sevastopol",
	"симферополь":              "simferopol",
	"смоленск":                 "smolensk",
	"сочи":                     "sochi",
	"ставрополь":               "stavropol",
	"стерлитамак":              "sterlitamak",
	"сургут":                   "surgut",
	"сыктывкар":                "syktyvkar",
	"тамбов":                   "tambov",
	"тверь":                    "tver",
	"тольятти":                 "tolyatti",
	"томск":                    "tomsk",
	"тюмень":                   "tyumen",
	"улан-удэ":                 "ulan-ude",
	"ульяновск":                "ulyanovsk",
	"усинск":                   "usinsk",
	"хабаровск":                "habarovsk",
	"ханты-мансийск":           "hanty-mansiysk",
	"чебоксары":                "cheboksary",
	"челябинск":                "chelyabinsk",
	"южно-сахалинск":           "yuzhno-sahalinsk",
	"якутск":                   "yakutsk",
	"ярославль":                "yaroslavl",
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
