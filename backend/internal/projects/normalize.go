package projects

import (
	"strings"
	"unicode"
)

// legalForms — организационно-правовые формы, которые пишутся заглавными
// всегда, как бы их ни ввели. Ключ — слово в нижнем регистре.
var legalForms = map[string]string{
	"ооо": "ООО",
	"оао": "ОАО",
	"ао":  "АО",
	"зао": "ЗАО",
	"ип":  "ИП",
	"пао": "ПАО",
	"нко": "НКО",
}

// capitalizeWord поднимает только первую букву слова. Без правил про
// организационно-правовые формы — их нельзя применять к ФИО, иначе
// фамилия вроде «Ип» превратилась бы в «ИП».
func capitalizeWord(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// capitalizeFirst делает первую букву заглавной, остальное не трогает, но
// организационно-правовые формы (ООО, АО, ИП…) поднимает целиком в любом
// месте строки: «ооо ромашка» → «ООО Ромашка».
func capitalizeFirst(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}

	words := strings.Fields(s)
	firstMeaningful := true
	for i, w := range words {
		// Правовую форму сверяем без окружающих кавычек и знаков.
		trimmed := strings.Trim(w, `"«»',.`)
		if up, ok := legalForms[strings.ToLower(trimmed)]; ok {
			words[i] = strings.Replace(w, trimmed, up, 1)
			continue
		}
		// Первое «обычное» слово — с заглавной. Правовая форма впереди
		// не считается: в «ооо ромашка» подниматься должна «Ромашка».
		if firstMeaningful {
			r := []rune(words[i])
			r[0] = unicode.ToUpper(r[0])
			words[i] = string(r)
			firstMeaningful = false
		}
	}
	return strings.Join(words, " ")
}

// normalizeFullName приводит ФИО к формату «Фамилия И.О.». Экономисты вводят
// по-разному: «иванов и.и.», «Иванов и. и.», «иванов иван иванович».
func normalizeFullName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}

	// Разделяем по пробелам; точки убираем, они восстановятся по формату.
	fields := strings.Fields(strings.ReplaceAll(s, ".", ". "))
	if len(fields) == 0 {
		return capitalizeWord(s)
	}

	surname := capitalizeWord(strings.TrimSuffix(fields[0], "."))
	if surname == "" {
		// Разобрать не вышло (например, ввели одни точки) — не теряем ввод.
		return capitalizeWord(s)
	}

	// Собираем инициалы из всех оставшихся кусков: берём первую букву
	// каждого, независимо от того, было это «и.» или «Иван».
	var initials []rune
	for _, f := range fields[1:] {
		f = strings.Trim(f, ".")
		if f == "" {
			continue
		}
		r := []rune(f)
		initials = append(initials, unicode.ToUpper(r[0]))
	}

	if len(initials) == 0 {
		return surname
	}

	var b strings.Builder
	b.WriteString(surname)
	b.WriteByte(' ')
	for _, r := range initials {
		b.WriteRune(r)
		b.WriteByte('.')
	}
	return b.String()
}
