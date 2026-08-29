package budgets

// Шкала рентабельности — та же, что на экранах платформы
// (frontend/src/utils/profitability.ts). Границы заданы владельцем
// 2026-08-27; проверяются сверху вниз, нижняя граница включается: 20 % —
// уже «высокорентабельный», 5 % — «среднерентабельный».
//
// ДУБЛИРОВАНИЕ ОСОЗНАННОЕ. Экран берёт шкалу из TypeScript, книга — из
// Go, и разойтись они не должны: вердикт в выгрузке обязан совпадать с
// цветом в реестре проектов. Меняя пороги или цвета здесь, поправить и
// там — файлы ссылаются друг на друга этими комментариями.
//
// Цвета без решётки: excelize ждёт RGB шестью символами.

type profitabilityLevel struct {
	// Min — нижняя граница ступени, %.
	Min float64
	// Label — вердикт, который печатается в книге.
	Label string
	// Color — заливка строки-вывода, RGB без «#».
	Color string
}

// profitabilityScale — от самой доходной ступени к самой слабой.
var profitabilityScale = []profitabilityLevel{
	{Min: 30, Label: "Сверхприбыльный", Color: "2F7D3A"},
	{Min: 20, Label: "Высокорентабельный", Color: "4E9455"},
	{Min: 5, Label: "Среднерентабельный", Color: "B07A12"},
	{Min: 1, Label: "Низкорентабельный", Color: "A8621A"},
	{Min: 0, Label: "Порог рентабельности", Color: "8E4A2A"},
}

var profitabilityLoss = profitabilityLevel{Label: "Убыточный", Color: "9C2B2B"}

// profitabilityGrade — ступень шкалы для рентабельности n (в процентах).
func profitabilityGrade(n float64) profitabilityLevel {
	// «> 30 %» — строго больше, остальные пороги включают нижнюю границу.
	if n > 30 {
		return profitabilityScale[0]
	}
	for _, lvl := range profitabilityScale[1:] {
		if n >= lvl.Min {
			return lvl
		}
	}
	return profitabilityLoss
}
