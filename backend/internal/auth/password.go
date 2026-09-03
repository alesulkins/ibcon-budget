package auth

import (
	"errors"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

// minPasswordLen — требование ТЗ 3.8: не менее 10 СИМВОЛОВ (не байт).
const minPasswordLen = 10

func HashPassword(plain string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// ValidatePassword проверяет пароль по ТЗ 3.8: не менее 10 символов, минимум
// 1 заглавная буква, минимум 1 цифра.
func ValidatePassword(plain string) error {
	if utf8.RuneCountInString(plain) < minPasswordLen {
		return errors.New("пароль должен содержать не менее 10 символов")
	}

	var hasUpper, hasDigit bool
	for _, r := range plain {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}

	if !hasUpper {
		return errors.New("пароль должен содержать минимум 1 заглавную букву")
	}
	if !hasDigit {
		return errors.New("пароль должен содержать минимум 1 цифру")
	}
	return nil
}
