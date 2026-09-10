package auth

import (
	"crypto/rand"
	"fmt"
	"log/slog"
	"math/big"
	"strings"

	"github.com/jmoiron/sqlx"
)

// EnsureFirstUser создаёт первую учётную запись главного экономиста, если
// пользователей нет вовсе.
//
// Без неё в свежую систему невозможно войти: миграции наполняют
// справочники, но не пользователей. Пароль берётся из окружения, а если
// он не задан — генерируется и один раз печатается в журнал: записывать
// пароль в код или в миграцию нельзя, он тогда одинаков у всех установок.
//
// Существующие учётные записи функция не трогает: при повторном запуске
// она просто ничего не делает.
func EnsureFirstUser(db *sqlx.DB, email, password, fullName string) error {
	var count int
	if err := db.Get(&count, `SELECT COUNT(*) FROM users`); err != nil {
		return fmt.Errorf("проверка пользователей: %w", err)
	}
	if count > 0 {
		return nil
	}

	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		email = "admin@ibcon.ru"
	}
	if fullName == "" {
		fullName = "Администратор"
	}

	generated := false
	if password == "" {
		var err error
		if password, err = randomPassword(); err != nil {
			return err
		}
		generated = true
	}
	if err := ValidatePassword(password); err != nil {
		return fmt.Errorf("пароль первого пользователя: %w", err)
	}

	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	_, err = db.Exec(
		`INSERT INTO users (email, password_hash, full_name, role, active)
		 VALUES ($1, $2, $3, $4, TRUE)`,
		email, hash, fullName, RoleGE)
	if err != nil {
		return fmt.Errorf("создание первого пользователя: %w", err)
	}

	if generated {
		// Пароль печатается один раз и только для сгенерированного:
		// заданный вручную в журнал не попадает.
		slog.Warn("создана первая учётная запись — смените пароль после входа",
			"email", email, "пароль", password)
	} else {
		slog.Info("создана первая учётная запись", "email", email)
	}
	return nil
}

// randomPassword — пароль, удовлетворяющий требованиям ValidatePassword.
func randomPassword() (string, error) {
	const (
		lower  = "abcdefghijkmnopqrstuvwxyz"
		upper  = "ABCDEFGHJKLMNPQRSTUVWXYZ"
		digits = "23456789"
		marks  = "!@#$%*-_"
	)
	// По одному символу каждого вида, остальное — вперемешку: так пароль
	// заведомо проходит проверку, а не «обычно проходит».
	groups := []string{lower, upper, digits, marks}
	all := lower + upper + digits + marks

	out := make([]byte, 0, 16)
	for _, g := range groups {
		c, err := pick(g)
		if err != nil {
			return "", err
		}
		out = append(out, c)
	}
	for len(out) < 16 {
		c, err := pick(all)
		if err != nil {
			return "", err
		}
		out = append(out, c)
	}
	// Перемешиваем, чтобы виды символов не стояли в предсказуемом порядке.
	for i := len(out) - 1; i > 0; i-- {
		j, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return "", err
		}
		k := j.Int64()
		out[i], out[k] = out[k], out[i]
	}
	return string(out), nil
}

func pick(set string) (byte, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(set))))
	if err != nil {
		return 0, err
	}
	return set[n.Int64()], nil
}
