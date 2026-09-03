package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

const (
	maxFailedAttempts = 5
	lockDuration      = 15 * time.Minute
	sessionTimeout    = 60 * time.Minute

	// rememberExpiryHours — срок жизни токена при «Запомнить меня»: 90 дней.
	// Решение владельца. Обычный срок берётся из JWT_EXPIRY_HOURS.
	rememberExpiryHours = 90 * 24
)

// errInvalidLogin — единственный ответ на любую неудачу входа.
var errInvalidLogin = errors.New("неверный email или пароль")

type LoginRequest struct {
	Email      string `json:"email" binding:"required,email"`
	Password   string `json:"password" binding:"required"`
	RememberMe bool   `json:"remember_me"`
}

type LoginResponse struct {
	Token    string `json:"token"`
	UserID   int    `json:"user_id"`
	FullName string `json:"full_name"`
	Role     string `json:"role"`
}

type Service struct {
	db          *sqlx.DB
	jwtSecret   string
	expiryHours int
}

func NewService(db *sqlx.DB, jwtSecret string, expiryHours int) *Service {
	return &Service{db: db, jwtSecret: jwtSecret, expiryHours: expiryHours}
}

type userRow struct {
	ID             int        `db:"id"`
	Email          string     `db:"email"`
	PasswordHash   string     `db:"password_hash"`
	FullName       string     `db:"full_name"`
	Role           string     `db:"role"`
	Active         bool       `db:"active"`
	FailedAttempts int        `db:"failed_attempts"`
	LockedUntil    *time.Time `db:"locked_until"`
	LastActivity   *time.Time `db:"last_activity"`
}

func (s *Service) Login(req LoginRequest) (*LoginResponse, error) {
	var u userRow
	err := s.db.Get(&u, `SELECT id, email, password_hash, full_name, role, active, failed_attempts, locked_until, last_activity FROM users WHERE email=$1`, req.Email)
	if err != nil {
		return nil, errInvalidLogin
	}

	if !u.Active {
		return nil, errInvalidLogin
	}

	if u.LockedUntil != nil && time.Now().Before(*u.LockedUntil) {
		return nil, errInvalidLogin
	}

	// Блокировка истекла — счётчик обнуляется, иначе следующая же ошибка
	// пароля снова упрётся в порог и заблокирует учётку повторно.
	// ТЗ требует блокировку после 5 неуспешных попыток ПОДРЯД.
	if u.LockedUntil != nil {
		u.FailedAttempts = 0
	}

	if !CheckPassword(u.PasswordHash, req.Password) {
		newFailed := u.FailedAttempts + 1
		if newFailed >= maxFailedAttempts {
			locked := time.Now().Add(lockDuration)
			_, _ = s.db.Exec(`UPDATE users SET failed_attempts=$1, locked_until=$2 WHERE id=$3`, newFailed, locked, u.ID)
			return nil, errInvalidLogin
		}
		_, _ = s.db.Exec(`UPDATE users SET failed_attempts=$1, locked_until=NULL WHERE id=$2`, newFailed, u.ID)
		return nil, errInvalidLogin
	}

	now := time.Now()
	_, _ = s.db.Exec(`UPDATE users SET failed_attempts=0, locked_until=NULL, last_activity=$1 WHERE id=$2`, now, u.ID)

	expiry := s.expiryHours
	if req.RememberMe {
		expiry = rememberExpiryHours
	}

	token, err := GenerateToken(Claims{
		UserID:   u.ID,
		Email:    u.Email,
		Role:     u.Role,
		FullName: u.FullName,
	}, s.jwtSecret, expiry)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	return &LoginResponse{
		Token:    token,
		UserID:   u.ID,
		FullName: u.FullName,
		Role:     u.Role,
	}, nil
}

// humanMinutes округляет длительность вверх до минут и склоняет слово
// «минута» по-русски: 1 минуту, 3 минуты, 15 минут.
func humanMinutes(d time.Duration) string {
	m := int(d.Minutes())
	if d > time.Duration(m)*time.Minute {
		m++ // 14 мин 30 с → «15 минут», а не «14»
	}
	if m < 1 {
		m = 1
	}

	word := "минут"
	switch {
	case m%100 >= 11 && m%100 <= 14: // 11–14 минут
	case m%10 == 1:
		word = "минуту"
	case m%10 >= 2 && m%10 <= 4:
		word = "минуты"
	}
	return fmt.Sprintf("%d %s", m, word)
}

func (s *Service) UpdateActivity(userID int) {
	_, _ = s.db.Exec(`UPDATE users SET last_activity=NOW() WHERE id=$1`, userID)
}
