package market

import (
	"math"
	"math/rand"
	"sort"
)

// Признаки объявления: комнаты, площадь, этаж, лифт, минуты до метро и
// признак посуточной площадки. Порядок фиксирован — на него опирается и
// обучение, и прогноз.
const (
	fRooms = iota
	fArea
	fFloor
	fElevator
	fMetro
	fDaily
	featureCount
)

// nan — «признак не задан». Пропуски заполняются средним по выборке:
// выбрасывать объявление целиком из-за неуказанного этажа расточительно,
// таких объявлений большинство.
var nan = math.NaN()

// features собирает вектор признаков объявления.
func features(o Observation) []float64 {
	v := make([]float64, featureCount)
	v[fRooms] = optF(float64(o.Rooms), o.Rooms > 0)
	v[fArea] = optF(o.Area, o.Area > 0)
	v[fFloor] = optF(float64(o.Floor), o.Floor > 0)
	v[fElevator] = nan
	if o.Elevator != nil {
		v[fElevator] = boolF(*o.Elevator)
	}
	v[fMetro] = optF(float64(o.MetroMinutes), o.MetroMinutes > 0)
	v[fDaily] = boolF(o.Daily)
	return v
}

// queryFeatures — тот же вектор для запроса пользователя. Посуточный
// признак нулевой: спрашивают месячную аренду.
func queryFeatures(q RentQuery) []float64 {
	return features(Observation{
		Rooms:        q.Rooms,
		Area:         q.Area,
		Floor:        q.Floor,
		Elevator:     q.Elevator,
		MetroMinutes: q.MetroMinutes,
		Daily:        false,
	})
}

func optF(v float64, ok bool) float64 {
	if !ok {
		return nan
	}
	return v
}

func boolF(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// imputer заменяет пропуски средним по обучающей выборке. Одни и те же
// средние применяются и к прогнозу — иначе модель получила бы на вход
// не то, на чём училась.
type imputer struct{ mean []float64 }

func fitImputer(x [][]float64) *imputer {
	im := &imputer{mean: make([]float64, featureCount)}
	for j := 0; j < featureCount; j++ {
		sum, n := 0.0, 0
		for i := range x {
			if !math.IsNaN(x[i][j]) {
				sum += x[i][j]
				n++
			}
		}
		if n > 0 {
			im.mean[j] = sum / float64(n)
		}
	}
	return im
}

func (im *imputer) apply(v []float64) []float64 {
	out := make([]float64, len(v))
	for j := range v {
		if math.IsNaN(v[j]) {
			out[j] = im.mean[j]
		} else {
			out[j] = v[j]
		}
	}
	return out
}

func (im *imputer) applyAll(x [][]float64) [][]float64 {
	out := make([][]float64, len(x))
	for i := range x {
		out[i] = im.apply(x[i])
	}
	return out
}

// regressor — общий вид всех трёх моделей, чтобы вызывающий код не знал,
// какая из них выбрана.
type regressor interface {
	predict(v []float64) float64
}

// ── Линейная регрессия ────────────────────────────────────────────
//
// Наименьшие квадраты через нормальные уравнения с гребневым слагаемым:
// на десятке объявлений признаки почти вырождены (площадь и комнаты
// связаны), и без регуляризации система решается неустойчиво.

type linearModel struct {
	w []float64 // свободный член в w[0]
}

func fitLinear(x [][]float64, y []float64, ridge float64) *linearModel {
	p := featureCount + 1
	a := make([][]float64, p)
	for i := range a {
		a[i] = make([]float64, p+1)
	}
	for i := range x {
		row := append([]float64{1}, x[i]...)
		for r := 0; r < p; r++ {
			for c := 0; c < p; c++ {
				a[r][c] += row[r] * row[c]
			}
			a[r][p] += row[r] * y[i]
		}
	}
	// Свободный член не штрафуем: иначе модель занижала бы уровень цен.
	for r := 1; r < p; r++ {
		a[r][r] += ridge
	}
	return &linearModel{w: solve(a, p)}
}

// solve — метод Гаусса с выбором ведущего элемента. Вырожденную систему
// не считаем ошибкой: неопределённые коэффициенты остаются нулевыми, и
// модель вырождается в среднее — это честнее, чем отказ.
func solve(a [][]float64, n int) []float64 {
	for col := 0; col < n; col++ {
		pivot := col
		for r := col + 1; r < n; r++ {
			if math.Abs(a[r][col]) > math.Abs(a[pivot][col]) {
				pivot = r
			}
		}
		if math.Abs(a[pivot][col]) < 1e-9 {
			continue
		}
		a[col], a[pivot] = a[pivot], a[col]
		for r := 0; r < n; r++ {
			if r == col {
				continue
			}
			k := a[r][col] / a[col][col]
			for c := col; c <= n; c++ {
				a[r][c] -= k * a[col][c]
			}
		}
	}
	w := make([]float64, n)
	for i := 0; i < n; i++ {
		if math.Abs(a[i][i]) > 1e-9 {
			w[i] = a[i][n] / a[i][i]
		}
	}
	return w
}

func (m *linearModel) predict(v []float64) float64 {
	s := m.w[0]
	for j := range v {
		s += m.w[j+1] * v[j]
	}
	return s
}

// ── Дерево регрессии ──────────────────────────────────────────────
//
// Общая основа для леса и бустинга. Разбиение выбирается по уменьшению
// суммы квадратов отклонений — обычный CART для регрессии.

type treeNode struct {
	leaf        bool
	value       float64
	feature     int
	threshold   float64
	left, right *treeNode
}

type treeParams struct {
	maxDepth int
	minLeaf  int
	// Сколько признаков перебирать в узле: у леса — часть (иначе все
	// деревья вырастут одинаковыми), у бустинга — все.
	featuresPerSplit int
	rnd              *rand.Rand
}

func buildTree(x [][]float64, y []float64, idx []int, p treeParams, depth int) *treeNode {
	if depth >= p.maxDepth || len(idx) < 2*p.minLeaf {
		return &treeNode{leaf: true, value: meanAt(y, idx)}
	}
	bestGain, bestFeat, bestThr := 0.0, -1, 0.0
	for _, f := range pickFeatures(p) {
		vals := uniqueSorted(x, idx, f)
		if len(vals) < 2 {
			continue
		}
		for _, thr := range thresholds(vals) {
			l, r := splitIdx(x, idx, f, thr)
			if len(l) < p.minLeaf || len(r) < p.minLeaf {
				continue
			}
			gain := sse(y, idx) - sse(y, l) - sse(y, r)
			if gain > bestGain {
				bestGain, bestFeat, bestThr = gain, f, thr
			}
		}
	}
	if bestFeat < 0 {
		return &treeNode{leaf: true, value: meanAt(y, idx)}
	}
	l, r := splitIdx(x, idx, bestFeat, bestThr)
	return &treeNode{
		feature:   bestFeat,
		threshold: bestThr,
		left:      buildTree(x, y, l, p, depth+1),
		right:     buildTree(x, y, r, p, depth+1),
	}
}

// Сколько порогов перебирать в узле. Полный перебор всех значений
// признака превращает обучение в квадрат от размера выборки: на трёх
// сотнях объявлений и двух сотнях деревьев бустинг считался бы минутами.
// Значения берутся равномерно по отсортированному ряду — это те же
// квантили, и разбиение от прореживания практически не меняется.
const maxSplitCandidates = 24

func thresholds(vals []float64) []float64 {
	step := 1
	if len(vals)-1 > maxSplitCandidates {
		step = (len(vals) - 1) / maxSplitCandidates
	}
	out := make([]float64, 0, maxSplitCandidates+1)
	for i := 0; i+1 < len(vals); i += step {
		out = append(out, (vals[i]+vals[i+1])/2)
	}
	return out
}

func pickFeatures(p treeParams) []int {
	all := make([]int, featureCount)
	for i := range all {
		all[i] = i
	}
	if p.featuresPerSplit >= featureCount || p.rnd == nil {
		return all
	}
	p.rnd.Shuffle(len(all), func(i, j int) { all[i], all[j] = all[j], all[i] })
	return all[:p.featuresPerSplit]
}

func (n *treeNode) predict(v []float64) float64 {
	for !n.leaf {
		if v[n.feature] <= n.threshold {
			n = n.left
		} else {
			n = n.right
		}
	}
	return n.value
}

func splitIdx(x [][]float64, idx []int, f int, thr float64) (l, r []int) {
	for _, i := range idx {
		if x[i][f] <= thr {
			l = append(l, i)
		} else {
			r = append(r, i)
		}
	}
	return
}

func uniqueSorted(x [][]float64, idx []int, f int) []float64 {
	vals := make([]float64, 0, len(idx))
	for _, i := range idx {
		vals = append(vals, x[i][f])
	}
	sort.Float64s(vals)
	out := vals[:0]
	for i, v := range vals {
		if i == 0 || v != vals[i-1] {
			out = append(out, v)
		}
	}
	return out
}

func meanAt(y []float64, idx []int) float64 {
	if len(idx) == 0 {
		return 0
	}
	s := 0.0
	for _, i := range idx {
		s += y[i]
	}
	return s / float64(len(idx))
}

func sse(y []float64, idx []int) float64 {
	m := meanAt(y, idx)
	s := 0.0
	for _, i := range idx {
		d := y[i] - m
		s += d * d
	}
	return s
}

// ── Случайный лес ─────────────────────────────────────────────────

type forestModel struct{ trees []*treeNode }

func fitForest(x [][]float64, y []float64, trees int, rnd *rand.Rand) *forestModel {
	m := &forestModel{}
	p := treeParams{maxDepth: 8, minLeaf: 3, featuresPerSplit: 3, rnd: rnd}
	for t := 0; t < trees; t++ {
		// Бутстрэп: выборка того же размера с возвращением.
		idx := make([]int, len(x))
		for i := range idx {
			idx[i] = rnd.Intn(len(x))
		}
		m.trees = append(m.trees, buildTree(x, y, idx, p, 0))
	}
	return m
}

func (m *forestModel) predict(v []float64) float64 {
	s := 0.0
	for _, t := range m.trees {
		s += t.predict(v)
	}
	return s / float64(len(m.trees))
}

// ── Градиентный бустинг ───────────────────────────────────────────

type boostModel struct {
	base  float64
	rate  float64
	trees []*treeNode
}

func fitBoost(x [][]float64, y []float64, rounds int, rate float64, rnd *rand.Rand) *boostModel {
	all := make([]int, len(x))
	for i := range all {
		all[i] = i
	}
	m := &boostModel{base: meanAt(y, all), rate: rate}
	pred := make([]float64, len(y))
	for i := range pred {
		pred[i] = m.base
	}
	// Квадратичная функция потерь: антиградиент — обычный остаток.
	res := make([]float64, len(y))
	p := treeParams{maxDepth: 3, minLeaf: 5, featuresPerSplit: featureCount, rnd: rnd}
	for r := 0; r < rounds; r++ {
		for i := range y {
			res[i] = y[i] - pred[i]
		}
		t := buildTree(x, res, all, p, 0)
		for i := range pred {
			pred[i] += rate * t.predict(x[i])
		}
		m.trees = append(m.trees, t)
	}
	return m
}

func (m *boostModel) predict(v []float64) float64 {
	s := m.base
	for _, t := range m.trees {
		s += m.rate * t.predict(v)
	}
	return s
}

// ── Выбор модели ──────────────────────────────────────────────────

// Пороги выбора модели. Мало данных — линейная регрессия: на десятке
// объявлений дерево запомнит выборку и на новых параметрах промахнётся.
// Умеренно — случайный лес: он терпим к выбросам, которых на досках
// объявлений много. Много — градиентный бустинг: он точнее леса, но на
// малой выборке переобучается.
const (
	forestFrom = 40
	boostFrom  = 300
)

type fitted struct {
	name   string
	reason string
	model  regressor
	im     *imputer
	mae    float64
}

// fitModel обучает модель на объявлениях и меряет ошибку на отложенной
// части выборки. nil — данных слишком мало даже для прямой.
func fitModel(obs []Observation, seed int64) *fitted {
	if len(obs) < 8 {
		return nil
	}
	rnd := rand.New(rand.NewSource(seed))

	rawX := make([][]float64, len(obs))
	y := make([]float64, len(obs))
	for i, o := range obs {
		rawX[i] = features(o)
		y[i] = o.PriceMonth
	}
	im := fitImputer(rawX)
	x := im.applyAll(rawX)

	// Перемешиваем и откладываем пятую часть на проверку: без неё
	// ошибку модели пришлось бы объявлять неизвестной, а по ней человек
	// решает, доверять ли цифре.
	ord := rnd.Perm(len(x))
	cut := len(x) * 4 / 5
	if cut == len(x) {
		cut = len(x) - 1
	}
	trX, trY := gather(x, y, ord[:cut])
	teX, teY := gather(x, y, ord[cut:])

	name, reason, model := chooseModel(len(trX), trX, trY, rnd)

	mae := 0.0
	for i := range teX {
		mae += math.Abs(model.predict(teX[i]) - teY[i])
	}
	if len(teX) > 0 {
		mae /= float64(len(teX))
	}

	// Итоговую модель переобучаем на всей выборке: отложенная часть
	// нужна была только для оценки ошибки.
	_, _, full := chooseModel(len(x), x, y, rand.New(rand.NewSource(seed)))
	return &fitted{name: name, reason: reason, model: full, im: im, mae: mae}
}

func chooseModel(n int, x [][]float64, y []float64, rnd *rand.Rand) (string, string, regressor) {
	switch {
	case n >= boostFrom:
		return "gradient_boosting",
			"объявлений много — градиентный бустинг",
			fitBoost(x, y, 200, 0.05, rnd)
	case n >= forestFrom:
		return "random_forest",
			"объявлений умеренно — случайный лес",
			fitForest(x, y, 60, rnd)
	default:
		return "linear_regression",
			"объявлений мало — линейная регрессия",
			fitLinear(x, y, 1.0)
	}
}

func gather(x [][]float64, y []float64, idx []int) ([][]float64, []float64) {
	xs := make([][]float64, 0, len(idx))
	ys := make([]float64, 0, len(idx))
	for _, i := range idx {
		xs = append(xs, x[i])
		ys = append(ys, y[i])
	}
	return xs, ys
}

// percentile — линейная интерполяция между соседними значениями
// отсортированной выборки.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	pos := p / 100 * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if hi >= len(sorted) {
		return sorted[len(sorted)-1]
	}
	return sorted[lo] + (sorted[hi]-sorted[lo])*(pos-float64(lo))
}
