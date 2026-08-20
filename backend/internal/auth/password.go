package auth

import (
	"errors"
	"regexp"

	"golang.org/x/crypto/bcrypt"
)

var (
	reUppercase = regexp.MustCompile(`[A-Z]`)
	reDigit     = regexp.MustCompile(`[0-9]`)
)

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

func ValidatePassword(plain string) error {
	if len(plain) < 10 {
		return errors.New("пароль должен содержать не менее 10 символов")
	}
	if !reUppercase.MatchString(plain) {
		return errors.New("пароль должен содержать минимум 1 заглавную букву")
	}
	if !reDigit.MatchString(plain) {
		return errors.New("пароль должен содержать минимум 1 цифру")
	}
	return nil
}
