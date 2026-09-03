package market

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

/*
«Почему такая цена» — объяснение методики и ответы на вопросы.

Две части, и они не взаимозаменяемы.

Первая — постоянный текст: как считается цена для бюджета, чем среднее
отличается от медианы, что такое перцентиль и какие объявления
отбрасываются. Он живёт здесь, в коде, а не в модели: методику человек
должен читать одну и ту же каждый раз, а не в новом пересказе.

Вторая — вопрос ассистенту про конкретный расчёт. Здесь без модели не
обойтись: вопросы задают словами, и заранее их не перечислить.
Ассистент отвечает по цифрам ЭТОГО расчёта — они передаются ему в
задании, — и не может ни поменять оценку, ни дописать данные. Модель
разворачивается рядом с платформой, а не в чужом облаке: вопрос про
бюджет проекта не должен уходить за периметр.
*/

// ExplainRequest — вопрос по конкретному расчёту.
type ExplainRequest struct {
	Question string   `json:"question"`
	Estimate Estimate `json:"estimate"`
}

// Methodology — постоянные пояснения к расчёту. Отдаются вместе с
// ответом на вопрос и показываются, даже когда ассистент недоступен.
type Methodology struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

// MethodologyText — то, что платформа объясняет про цену всегда.
//
// Пишется здесь, а не на фронтенде: это описание того, что делает
// расчёт, и оно должно меняться вместе с ним, а не отдельно.
func MethodologyText() []Methodology {
	return []Methodology{
		{
			Title: "Что берётся в бюджет",
			Text: "Среднее по объявлениям, из которых убраны пять процентов " +
				"самых дорогих. Верхние пять процентов отброшены потому, что это " +
				"премиальные объекты и объявления с завышенной ценой — они " +
				"месяцами висят несданными и на цену договора не влияют. На " +
				"оставшейся выборке среднее уже не перекошено: обычная претензия " +
				"к среднему — чувствительность к дорогим объявлениям — снята " +
				"самим отсечением.",
		},
		{
			Title: "Почему среднее, а не медиана",
			Text: "Медиана показывает только середину ряда и не замечает, как " +
				"устроена остальная выборка: рынок, где дорогих квартир много, и " +
				"рынок, где их нет вовсе, дадут одну и ту же медиану. Среднее " +
				"учитывает все объявления сразу. Его слабое место — дорогие " +
				"выбросы, но они уже отсечены по 95-му перцентилю, поэтому " +
				"среднее здесь несмещённое.",
		},
		{
			Title: "Что такое 95-й перцентиль",
			Text: "Цена, дороже которой только пять процентов предложений. " +
				"Показывается как «верх рынка»: по ней видно, сколько будет " +
				"стоить квартира, если брать срочно и из того, что осталось.",
		},
		{
			Title: "Как отбрасываются выбросы",
			Text: "Перед расчётом выборка чистится по правилу полутора " +
				"межквартильных размахов: объявления дешевле первого квартиля " +
				"и дороже третьего больше чем на полтора расстояния между ними " +
				"в расчёт не идут. Так уходят «квартира за 1 ₽» и суточные " +
				"цены, проставленные как месячные.",
		},
		{
			Title: "Как отбираются объявления",
			Text: "По городу проекта, по числу комнат (точно) и по площади " +
				"с допуском ±10 %. Если таких объявлений меньше восьми, допуск " +
				"расширяется до ±20 % и ±35 %, а затем снимается — и об этом " +
				"написано в строке «Объявлений в расчёте». Посуточная аренда " +
				"не учитывается вовсе: суточная цена, умноженная на 30, не " +
				"является ставкой по договору найма.",
		},
	}
}

/*
Ассистент работает на модели, которую разворачивают РЯДОМ с платформой,
а не в чужом облаке.

Почему так: вопросы задаются про конкретный бюджет проекта, и текст
запроса не должен уходить за периметр — это же и есть требование к
отечественному контуру. Плюс открытая модель не требует подписки и не
считает токены.

Разговор идёт по протоколу OpenAI (`/v1/chat/completions`) — его понимают
Ollama, llama.cpp, vLLM, LM Studio и российские шлюзы, поэтому сменить
модель можно переменными окружения, не трогая код:

	ASSISTANT_BASE_URL — адрес сервера модели (по умолчанию локальная Ollama)
	ASSISTANT_MODEL    — имя модели
	ASSISTANT_API_KEY  — ключ, если сервер его спрашивает (локальному не нужен)

Готовый рецепт: `ollama serve` и `ollama pull qwen2.5:7b-instruct`.
*/
const (
	defaultAssistantURL   = "http://localhost:11434/v1"
	defaultAssistantModel = "qwen2.5:7b-instruct"
)

// Потолок ответа. Ассистент отвечает на вопрос по расчёту, а не пишет
// доклад: длинный ответ в узкой панели никто не читает.
const assistantMaxTokens = 700

// Ограничение на вопрос: всё, что длиннее, — это уже не вопрос по
// расчёту, а попытка передать модели посторонний текст.
const maxQuestionLen = 500

// Локальная модель на процессоре отвечает не мгновенно — ждём до минуты.
const assistantTimeout = 60 * time.Second

// ErrNoAssistant — сервер модели не отвечает, ассистент недоступен.
var ErrNoAssistant = fmt.Errorf("ии-ассистент не отвечает: не запущен сервер модели")

func assistantBaseURL() string {
	if v := os.Getenv("ASSISTANT_BASE_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return defaultAssistantURL
}

func assistantModelName() string {
	if v := os.Getenv("ASSISTANT_MODEL"); v != "" {
		return v
	}
	return defaultAssistantModel
}

// Ask отвечает на вопрос человека по конкретному расчёту.
func Ask(ctx context.Context, req ExplainRequest) (string, error) {
	q := strings.TrimSpace(req.Question)
	if q == "" {
		return "", fmt.Errorf("вопрос пустой")
	}
	if len([]rune(q)) > maxQuestionLen {
		return "", fmt.Errorf("вопрос длиннее %d символов", maxQuestionLen)
	}

	body, _ := json.Marshal(map[string]any{
		"model": assistantModelName(),
		"messages": []map[string]string{
			{"role": "system", "content": assistantSystem(req.Estimate)},
			{"role": "user", "content": q},
		},
		"max_tokens": assistantMaxTokens,
		// Ответ про цифры должен быть одинаковым от раза к разу: человек
		// перечитывает его, а не собирает варианты.
		"temperature": 0.2,
		"stream":      false,
	})

	ctx, cancel := context.WithTimeout(ctx, assistantTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		assistantBaseURL()+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if key := os.Getenv("ASSISTANT_API_KEY"); key != "" {
		httpReq.Header.Set("Authorization", "Bearer "+key)
	}

	resp, err := (&http.Client{Timeout: assistantTimeout}).Do(httpReq)
	if err != nil {
		return "", ErrNoAssistant
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode >= 500 {
		return "", ErrNoAssistant
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("модель ответила %d", resp.StatusCode)
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("ответ модели не разобран")
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("модель вернула пустой ответ")
	}
	answer := strings.TrimSpace(out.Choices[0].Message.Content)
	if answer == "" {
		return "", fmt.Errorf("модель вернула пустой ответ")
	}
	return answer, nil
}

/*
assistantSystem — задание ассистенту.

Цифры расчёта передаются целиком: без них ассистент отвечал бы общими
словами про среднее, а спрашивают про конкретную квартиру. Считать он
ничего не должен — расчёт уже сделан, и второй его вариант в ответе
только запутает.
*/
func assistantSystem(e Estimate) string {
	var b strings.Builder
	b.WriteString(
		"Ты помогаешь экономисту строительной компании разобраться в оценке " +
			"рыночной стоимости аренды квартиры, которую посчитала платформа.\n\n" +
			"Правила:\n" +
			"— отвечай коротко, по-русски, обычными словами, без формул и списков, " +
			"если человек сам не попросил подробностей;\n" +
			"— опирайся только на цифры расчёта ниже; своих не выдумывай и заново " +
			"ничего не считай;\n" +
			"— если данных для ответа нет, так и скажи;\n" +
			"— на вопросы не про эту оценку отвечай, что помогаешь только с ней.\n\n")

	b.WriteString("Как платформа считает цену:\n")
	for _, m := range MethodologyText() {
		b.WriteString("— " + m.Title + ". " + m.Text + "\n")
	}

	b.WriteString("\nЦифры этого расчёта:\n")
	fmt.Fprintf(&b, "— город: %s\n", e.Query.City)
	if e.Query.Rooms > 0 {
		fmt.Fprintf(&b, "— комнат в запросе: %d\n", e.Query.Rooms)
	}
	if e.Query.Area > 0 {
		fmt.Fprintf(&b, "— площадь в запросе: %.0f м²\n", e.Query.Area)
	}
	fmt.Fprintf(&b, "— объявлений в расчёте: %d\n", e.Sample)
	if e.Matched != "" {
		fmt.Fprintf(&b, "— отбор: %s\n", e.Matched)
	}
	fmt.Fprintf(&b, "— цена для бюджета (среднее без верхних 5 %%): %.0f ₽/мес\n", e.Recommended)
	fmt.Fprintf(&b, "— медиана всей выборки: %.0f ₽/мес\n", e.P50)
	fmt.Fprintf(&b, "— верх рынка (95-й перцентиль): %.0f ₽/мес\n", e.P95)
	for _, s := range e.Sources {
		if s.Error != "" {
			fmt.Fprintf(&b, "— площадка %s: не ответила (%s)\n", s.Source, s.Error)
			continue
		}
		fmt.Fprintf(&b, "— площадка %s: объявлений %d, медиана %.0f ₽/мес\n",
			s.Source, s.Count, s.Median)
	}
	if len(e.Histogram) > 0 {
		b.WriteString("— распределение цен по диапазонам:\n")
		for _, bin := range e.Histogram {
			fmt.Fprintf(&b, "  %.0f–%.0f ₽: %d\n", bin.From, bin.To, bin.Count)
		}
	}
	return b.String()
}
