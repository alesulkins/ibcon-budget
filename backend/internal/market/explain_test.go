package market

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Ассистент разговаривает по протоколу OpenAI, поэтому проверяется он
// поднятым рядом сервером-заглушкой: так видно и что уходит в модель, и
// что платформа делает с ответом. Настоящая модель для этого не нужна.
func TestAskTalksToLocalModel(t *testing.T) {
	var gotPath string
	var body map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"Цена ниже верха рынка, потому что верхние пять процентов отброшены."}}]}`))
	}))
	defer srv.Close()

	t.Setenv("ASSISTANT_BASE_URL", srv.URL+"/v1")
	t.Setenv("ASSISTANT_MODEL", "тестовая-модель")

	est := Estimate{
		Query:       RentQuery{City: "Норильск", Area: 45},
		Sample:      38,
		Recommended: 40000,
		P50:         40000,
		P95:         60000,
		Matched:     "площадь 40–50 м²",
	}
	answer, err := Ask(context.Background(), ExplainRequest{
		Question: "почему цена ниже верха рынка?",
		Estimate: est,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(answer, "верхние пять процентов") {
		t.Errorf("ответ модели потерян: %q", answer)
	}
	if gotPath != "/v1/chat/completions" {
		t.Errorf("запрос ушёл по адресу %q", gotPath)
	}
	if body["model"] != "тестовая-модель" {
		t.Errorf("модель в запросе: %v", body["model"])
	}

	// В задании модели должны стоять цифры именно этого расчёта: без них
	// она отвечала бы общими словами про среднее и перцентили.
	msgs, _ := body["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("сообщений в запросе: %d", len(msgs))
	}
	system, _ := msgs[0].(map[string]any)["content"].(string)
	for _, want := range []string{"Норильск", "40000", "60000", "площадь 40–50 м²"} {
		if !strings.Contains(system, want) {
			t.Errorf("в задании модели нет %q", want)
		}
	}
}

// Сервера модели нет — платформа говорит об этом отдельной ошибкой, а не
// «что-то пошло не так»: это единственное, что человек может починить.
func TestAskWithoutServer(t *testing.T) {
	t.Setenv("ASSISTANT_BASE_URL", "http://127.0.0.1:1")
	_, err := Ask(context.Background(), ExplainRequest{Question: "почему?"})
	if err != ErrNoAssistant {
		t.Errorf("ошибка: %v, ожидалась «ассистент не отвечает»", err)
	}
}

func TestAskRejectsEmptyAndLongQuestions(t *testing.T) {
	if _, err := Ask(context.Background(), ExplainRequest{Question: "   "}); err == nil {
		t.Error("пустой вопрос принят")
	}
	long := strings.Repeat("а", maxQuestionLen+1)
	if _, err := Ask(context.Background(), ExplainRequest{Question: long}); err == nil {
		t.Error("слишком длинный вопрос принят")
	}
}
