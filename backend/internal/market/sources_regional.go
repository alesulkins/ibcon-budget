package market

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

/*
Площадки, которые подключаются только к своим городам.

Проект в Норильске и проект в Бишкеке — разные рынки, и федеральная
выдача о них почти ничего не знает. Поэтому у каждой площадки есть свой
список городов, а сервис спрашивает только те, что этот город знают:
площадка, города не знающая, молча отдаёт выдачу другого — цифра в
бюджете оказалась бы московской.
*/

// ── Этажи ─────────────────────────────────────────────────────────
//
// Федеральное агентство с сайтом на поддомен для каждого города —
// поэтому у него есть и Норильск, и Сургут, которых нет у площадок
// объявлений. Данные объявления лежат готовым JSON внутри страницы.

type etagiSource struct{}

func (etagiSource) Name() string { return "Этажи" }

/*
Поддомены выверены запросом по каждому: у площадки есть поддомены без
аренды (Мурманск) и поддомены, отдающие выдачу Санкт-Петербурга
(Владивосток). Неизвестный город — это отсутствие площадки, а не повод
угадывать адрес.
*/
var etagiSlugs = map[string]string{
	"казань":          "kazan",
	"москва":          "msk",
	"нижний новгород": "nn",
	"норильск":        "norilsk",
	"ростов-на-дону":  "rostov",
	"самара":          "samara",
	"санкт-петербург": "spb",
	"сочи":            "sochi",
	"сургут":          "surgut",
	"тверь":           "tver",
	"тюмень":          "tyumen",
	"якутск":          "yakutsk",
}

// Сколько страниц берём: на странице тридцать объявлений, пять страниц —
// полторы сотни, этого хватает и после отбора по площади.
const etagiPages = 5

func (etagiSource) Supports(q RentQuery) bool {
	_, ok := etagiSlugs[cityKey(q.City)]
	return ok
}

func (e etagiSource) Fetch(ctx context.Context, q RentQuery) ([]Observation, error) {
	slug := etagiSlugs[cityKey(q.City)]
	if slug == "" {
		return nil, fmt.Errorf("город «%s» не поддерживается площадкой", q.City)
	}
	base := fmt.Sprintf("https://%s.etagi.com/realty_rent/", slug)

	obs, err := fetchPages(ctx, etagiPages, func(page int) string {
		if page == 1 {
			return base
		}
		return base + "?page=" + strconv.Itoa(page)
	}, func(body []byte) ([]Observation, error) {
		return parseEtagi(e.Name(), body)
	})
	if err != nil {
		return nil, err
	}
	return obs, nil
}

/*
parseEtagi разбирает объявления из состояния страницы.

Опора на форму записи, а не на разметку: у объявления поля лежат рядом
одним объектом — цена строкой, площадь и комнаты числами. Цена берётся
из "price", а не из "price_m2": вторая — цена метра, и она на два
порядка меньше.
*/
func parseEtagi(source string, body []byte) ([]Observation, error) {
	const priceKey = `"price":"`
	var out []Observation
	for pos := 0; ; {
		i := bytes.Index(body[pos:], []byte(priceKey))
		if i < 0 {
			break
		}
		i += pos
		pos = i + len(priceKey)
		price, ok := readNumber(body, pos)
		if !ok || price < 3000 || price > 5_000_000 {
			continue
		}
		// Остальные поля объявления записаны после цены — смотрим вперёд
		// до начала следующего объявления.
		ahead := window(body, pos, 900)
		if next := bytes.Index(ahead, []byte(priceKey)); next > 0 {
			ahead = ahead[:next]
		}
		o := Observation{Source: source, PriceMonth: price}
		if v, ok := firstNumberFor(ahead, `"square":`); ok {
			o.Area = v
		}
		if v, ok := firstNumberFor(ahead, `"rooms":`); ok {
			o.Rooms = int(v)
		}
		if v, ok := firstNumberFor(ahead, `"floor":`); ok {
			o.Floor = int(v)
		}
		out = append(out, o)
	}
	if len(out) == 0 {
		return nil, errUnparsed
	}
	return dedupe(out), nil
}

// ── N1.ru ─────────────────────────────────────────────────────────
//
// Площадка объявлений с городами Урала, Сибири и Севера. Даёт вторую
// выборку там, где у федеральных площадок объявлений мало, и объявление
// у неё разобрано по полям: цена, комнаты, этаж и площадь.

type n1Source struct{}

func (n1Source) Name() string { return "N1.ru" }

// Поддомены выверены запросом по каждому: заголовок выдачи должен
// называть тот же город, что и в запросе.
var n1Slugs = map[string]string{
	"архангельск":     "arhangelsk",
	"волжский":        "volzhskiy",
	"екатеринбург":    "ekaterinburg",
	"красноярск":      "krasnoyarsk",
	"магнитогорск":    "magnitogorsk",
	"москва":          "msk",
	"новосибирск":     "novosibirsk",
	"норильск":        "norilsk",
	"пермь":           "perm",
	"ростов-на-дону":  "rostov-na-donu",
	"санкт-петербург": "spb",
	"севастополь":     "sevastopol",
	"челябинск":       "chelyabinsk",
	"южно-сахалинск":  "yuzhno-sahalinsk",
}

// На странице выдачи два с половиной десятка объявлений — пять страниц
// дают полторы сотни.
const n1Pages = 5

func (n1Source) Supports(q RentQuery) bool {
	_, ok := n1Slugs[cityKey(q.City)]
	return ok
}

func (n n1Source) Fetch(ctx context.Context, q RentQuery) ([]Observation, error) {
	slug := n1Slugs[cityKey(q.City)]
	if slug == "" {
		return nil, fmt.Errorf("город «%s» не поддерживается площадкой", q.City)
	}
	base := fmt.Sprintf("https://%s.n1.ru/snyat/kvartiry/", slug)

	return fetchPages(ctx, n1Pages, func(page int) string {
		if page == 1 {
			return base
		}
		return base + "?page=" + strconv.Itoa(page)
	}, func(body []byte) ([]Observation, error) {
		return parseN1(n.Name(), body)
	})
}

/*
parseN1 разбирает объявления из состояния страницы.

Площадь у площадки записана в сотых долях метра целым числом
("total_area":4410 — это 44,1 м²): читаем и делим, иначе однушка
выглядела бы как гектар и вылетала из отбора по площади.
*/
func parseN1(source string, body []byte) ([]Observation, error) {
	const roomsKey = `"rooms_count":`
	var out []Observation
	for pos := 0; ; {
		i := bytes.Index(body[pos:], []byte(roomsKey))
		if i < 0 {
			break
		}
		i += pos
		pos = i + len(roomsKey)

		o := Observation{Source: source}
		if v, ok := readNumber(body, pos); ok {
			o.Rooms = int(v)
		}
		// Цена и площадь стоят ПЕРЕД числом комнат в том же объекте
		// объявления — смотрим назад до начала предыдущего.
		back := body[max0(i-6000):i]
		if v, ok := lastNumberFor(back, `"price":`); ok {
			o.PriceMonth = v
		}
		if v, ok := lastNumberFor(back, `"total_area":`); ok {
			o.Area = v / 100
		}
		if v, ok := lastNumberFor(back, `"floor":`); ok {
			o.Floor = int(v)
		}
		if o.PriceMonth < 3000 || o.PriceMonth > 5_000_000 {
			continue
		}
		out = append(out, o)
	}
	if len(out) == 0 {
		return nil, errUnparsed
	}
	return dedupe(out), nil
}

// ── House.kg ──────────────────────────────────────────────────────
//
// Киргизская площадка объявлений. Цены на ней в сомах, поэтому оценка
// пересчитывается по курсу Банка России — см. currency.go.

type houseKGSource struct{}

func (houseKGSource) Name() string { return "House.kg" }

// Города Киргизии, где у площадки есть аренда. Прочие киргизские города
// спрашиваются по всей стране: объявлений там единицы, и выдача по
// стране ближе к правде, чем пустой ответ.
var kgTowns = map[string]int{
	"бишкек":      2,
	"токмок":      3,
	"кара-балта":  5,
	"кант":        9,
	"каракол":     11,
	"балыкчы":     12,
	"чолпон-ата":  17,
	"джалал-абад": 27,
}

// Киргизские города, для которых площадка подключается. Ош и остальные
// идут по выдаче всей страны.
var kgCities = map[string]bool{
	"бишкек": true, "ош": true, "джалал-абад": true, "каракол": true,
	"токмок": true, "кара-балта": true, "балыкчы": true, "нарын": true,
	"талас": true, "баткен": true, "кант": true, "узген": true,
	"чолпон-ата": true, "кызыл-кия": true, "сулюкта": true,
	"майлуу-суу": true, "исфана": true, "кербен": true,
	// Проект могут завести и на страну целиком.
	"киргизия": true, "кыргызстан": true,
}

const kgPages = 4

func (houseKGSource) Supports(q RentQuery) bool { return kgCities[cityKey(q.City)] }

func (h houseKGSource) Fetch(ctx context.Context, q RentQuery) ([]Observation, error) {
	if !h.Supports(q) {
		return nil, fmt.Errorf("город «%s» не поддерживается площадкой", q.City)
	}
	// Курс берём до похода за объявлениями: без него сомы в рубли не
	// перевести, а показывать сомы под подписью «₽/мес» нельзя.
	rate, err := rubPer(ctx, "KGS")
	if err != nil {
		return nil, err
	}

	params := url.Values{}
	if town, ok := kgTowns[cityKey(q.City)]; ok {
		params.Set("town", strconv.Itoa(town))
	}
	obs, err := fetchPages(ctx, kgPages, func(page int) string {
		p := url.Values{}
		for k, v := range params {
			p[k] = v
		}
		if page > 1 {
			p.Set("page", strconv.Itoa(page))
		}
		addr := "https://www.house.kg/snyat-kvartiru"
		if len(p) > 0 {
			addr += "?" + p.Encode()
		}
		return addr
	}, func(body []byte) ([]Observation, error) {
		return parseHouseKG(h.Name(), body, rate)
	})
	if err != nil {
		return nil, err
	}
	return obs, nil
}

/*
parseHouseKG разбирает карточки объявлений.

У карточки цена вынесена дважды: в долларах и в сомах — берём сомы, они
и есть цена договора. Комнаты и площадь стоят в заголовке карточки
строкой вида «3-комн. кв., 76 м2, 10 этаж из 10», её и читаем: разметка
у площадки меняется, а эта подпись — нет.
*/
func parseHouseKG(source string, body []byte, rubPerKGS float64) ([]Observation, error) {
	const priceKey = `class="price-addition">`
	var out []Observation
	for pos := 0; ; {
		i := bytes.Index(body[pos:], []byte(priceKey))
		if i < 0 {
			break
		}
		i += pos
		pos = i + len(priceKey)

		som := readSpacedNumber(body[pos:])
		if som <= 0 {
			continue
		}
		price := som * rubPerKGS
		if price < 3000 || price > 5_000_000 {
			continue
		}
		o := Observation{Source: source, PriceMonth: price}
		// Заголовок карточки стоит перед ценой.
		if rooms, area, ok := parseKGTitle(body[max0(i-3000):i]); ok {
			o.Rooms, o.Area = rooms, area
		}
		out = append(out, o)
	}
	if len(out) == 0 {
		return nil, errUnparsed
	}
	return dedupe(out), nil
}

// readSpacedNumber читает число, записанное с пробелами между разрядами:
// «78 831 сом/мес.». Обычные пробелы и неразрывные — вперемешку.
func readSpacedNumber(b []byte) float64 {
	digits := make([]byte, 0, 12)
	for i := 0; i < len(b) && i < 24; i++ {
		c := b[i]
		switch {
		case c >= '0' && c <= '9':
			digits = append(digits, c)
		case c == ' ' || c == '\n' || c == '\t' || c == 0xC2 || c == 0xA0:
			// разделитель разрядов — пропускаем
		default:
			i = len(b) // дальше уже подпись валюты
		}
	}
	if len(digits) == 0 {
		return 0
	}
	v, err := strconv.ParseFloat(string(digits), 64)
	if err != nil {
		return 0
	}
	return v
}

// parseKGTitle читает «3-комн. кв., 76 м2» из последнего заголовка в куске.
func parseKGTitle(chunk []byte) (rooms int, area float64, ok bool) {
	i := bytes.LastIndex(chunk, []byte("-комн. кв.,"))
	if i < 0 {
		return 0, 0, false
	}
	// Число комнат — цифры прямо перед подписью.
	j := i
	for j > 0 && chunk[j-1] >= '0' && chunk[j-1] <= '9' {
		j--
	}
	if j == i {
		return 0, 0, false
	}
	rooms, err := strconv.Atoi(string(chunk[j:i]))
	if err != nil {
		return 0, 0, false
	}
	rest := chunk[i:]
	k := bytes.Index(rest, []byte("м2"))
	if k < 0 {
		return rooms, 0, true
	}
	// Площадь — число перед «м2».
	end := k
	for end > 0 && (rest[end-1] == ' ') {
		end--
	}
	start := end
	for start > 0 && (rest[start-1] == '.' || (rest[start-1] >= '0' && rest[start-1] <= '9')) {
		start--
	}
	area, err = strconv.ParseFloat(strings.TrimSpace(string(rest[start:end])), 64)
	if err != nil {
		return rooms, 0, true
	}
	return rooms, area, true
}

// ── Общее ─────────────────────────────────────────────────────────

// cityKey — город в сравнимом виде: регистр и пробелы по краям у
// названий проектов бывают любые.
func cityKey(city string) string {
	return strings.ToLower(strings.TrimSpace(city))
}

/*
fetchPages запрашивает страницы выдачи одновременно и складывает
объявления в одну выборку.

Одновременно — потому что последовательный обход пяти страниц не
уложился бы в отведённое запросу время. Ошибку возвращаем, только если
не удалось ничего: одна отвалившаяся страница из пяти оценке не мешает.
*/
func fetchPages(
	ctx context.Context,
	pages int,
	addr func(page int) string,
	parse func(body []byte) ([]Observation, error),
) ([]Observation, error) {
	type result struct {
		obs []Observation
		err error
	}
	results := make([]result, pages)
	var wg sync.WaitGroup
	for page := 1; page <= pages; page++ {
		wg.Add(1)
		go func(page int) {
			defer wg.Done()
			data, err := fetchBody(ctx, http.MethodGet, addr(page), nil)
			if err != nil {
				results[page-1] = result{err: err}
				return
			}
			obs, err := parse(data)
			results[page-1] = result{obs: obs, err: err}
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

// firstNumberFor берёт число у ПЕРВОГО вхождения ключа в куске.
func firstNumberFor(chunk []byte, key string) (float64, bool) {
	i := bytes.Index(chunk, []byte(key))
	if i < 0 {
		return 0, false
	}
	return readNumber(chunk, i+len(key))
}
