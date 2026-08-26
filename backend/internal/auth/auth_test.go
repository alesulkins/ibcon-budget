package auth

import (
	"testing"
	"time"
)

// TestHumanMinutes — склонение «минута» в сообщении о блокировке.
// Длительность округляется ВВЕРХ: пользователю нельзя обещать разблокировку
// раньше, чем она произойдёт.
func TestHumanMinutes(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{15 * time.Minute, "15 минут"},
		{time.Minute, "1 минуту"},
		{2 * time.Minute, "2 минуты"},
		{4 * time.Minute, "4 минуты"},
		{5 * time.Minute, "5 минут"},
		{11 * time.Minute, "11 минут"},
		{12 * time.Minute, "12 минут"},
		{14 * time.Minute, "14 минут"},
		{21 * time.Minute, "21 минуту"},
		{22 * time.Minute, "22 минуты"},
		{25 * time.Minute, "25 минут"},
		// Округление вверх до целой минуты
		{14*time.Minute + 30*time.Second, "15 минут"},
		{90 * time.Second, "2 минуты"},
		// Хвост меньше минуты не должен давать «0 минут»
		{30 * time.Second, "1 минуту"},
		{0, "1 минуту"},
		{-5 * time.Second, "1 минуту"},
	}
	for _, tt := range tests {
		if got := humanMinutes(tt.d); got != tt.want {
			t.Errorf("humanMinutes(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

// TestValidatePassword — требования ТЗ 3.8: не менее 10 символов,
// минимум 1 заглавная буква, минимум 1 цифра.
func TestValidatePassword(t *testing.T) {
	valid := []string{
		"Parol12345",
		"AAAAAAAAA1",     // ровно 10 символов
		"очень_Длинный1", // кириллица допускается
		"P@ssw0rd!!!",    // спецсимволы не мешают
	}
	for _, p := range valid {
		if err := ValidatePassword(p); err != nil {
			t.Errorf("пароль %q должен проходить, got: %v", p, err)
		}
	}

	invalid := []struct {
		pass string
		why  string
	}{
		{"Parol1234", "9 символов — на один меньше порога"},
		{"", "пустой"},
		{"parol12345", "нет заглавной буквы"},
		{"ParolParol", "нет цифры"},
		{"1234567890", "нет заглавной буквы"},
		// Длина считается в символах, а не в байтах: 7 кириллических
		// символов = 13 байт, но порог не пройден.
		{"Пароль1", "7 символов, хотя 13 байт"},
	}
	for _, tt := range invalid {
		if err := ValidatePassword(tt.pass); err == nil {
			t.Errorf("пароль %q (%s) должен отклоняться, got nil", tt.pass, tt.why)
		}
	}
}

// TestRememberExpiry — «Запомнить меня» продлевает токен до 90 дней,
// обычный вход — до срока из конфигурации.
func TestRememberExpiry(t *testing.T) {
	if rememberExpiryHours != 90*24 {
		t.Errorf("срок «Запомнить меня» должен быть 90 дней (%d ч), got %d ч",
			90*24, rememberExpiryHours)
	}

	const secret = "test_secret_key_at_least_32_chars!!"
	claims := Claims{UserID: 1, Email: "a@b.ru", Role: RoleGE, FullName: "Тест"}

	tok, err := GenerateToken(claims, secret, rememberExpiryHours)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	parsed, err := ParseToken(tok, secret)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if parsed.UserID != claims.UserID || parsed.Role != claims.Role {
		t.Errorf("claims не пережили round-trip: %+v", parsed)
	}

	// Просроченный токен обязан отклоняться
	expired, err := GenerateToken(claims, secret, -1)
	if err != nil {
		t.Fatalf("GenerateToken(-1): %v", err)
	}
	if _, err := ParseToken(expired, secret); err == nil {
		t.Error("просроченный токен должен отклоняться, got nil")
	}

	// Чужая подпись обязана отклоняться
	if _, err := ParseToken(tok, secret+"x"); err == nil {
		t.Error("токен с чужой подписью должен отклоняться, got nil")
	}
}

// TestSessionTimeoutConst — автовыход по бездействию 60 минут (ТЗ 3.8 п.6).
func TestSessionTimeoutConst(t *testing.T) {
	if sessionTimeout != 60*time.Minute {
		t.Errorf("автовыход должен быть 60 минут, got %v", sessionTimeout)
	}
	if maxFailedAttempts != 5 {
		t.Errorf("порог блокировки должен быть 5 попыток, got %d", maxFailedAttempts)
	}
	if lockDuration != 15*time.Minute {
		t.Errorf("блокировка должна быть 15 минут, got %v", lockDuration)
	}
}
