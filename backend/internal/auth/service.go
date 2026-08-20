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
)

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
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
	ID           int        `db:"id"`
	Email        string     `db:"email"`
	PasswordHash string     `db:"password_hash"`
	FullName     string     `db:"full_name"`
	Role         string     `db:"role"`
	Active       bool       `db:"active"`
	FailedAttempts int      `db:"failed_attempts"`
	LockedUntil  *time.Time `db:"locked_until"`
	LastActivity *time.Time `db:"last_activity"`
}

func (s *Service) Login(req LoginRequest) (*LoginResponse, error) {
	var u userRow
	err := s.db.Get(&u, `SELECT id, email, password_hash, full_name, role, active, failed_attempts, locked_until, last_activity FROM users WHERE email=$1`, req.Email)
	if err != nil {
		return nil, errors.New("неверный email или пароль")
	}

	if !u.Active {
		return nil, errors.New("учётная запись отключена")
	}

	if u.LockedUntil != nil && time.Now().Before(*u.LockedUntil) {
		remaining := time.Until(*u.LockedUntil).Round(time.Minute)
		return nil, fmt.Errorf("учётная запись заблокирована, попробуйте через %v", remaining)
	}

	if !CheckPassword(u.PasswordHash, req.Password) {
		newFailed := u.FailedAttempts + 1
		if newFailed >= maxFailedAttempts {
			locked := time.Now().Add(lockDuration)
			_, _ = s.db.Exec(`UPDATE users SET failed_attempts=$1, locked_until=$2 WHERE id=$3`, newFailed, locked, u.ID)
		} else {
			_, _ = s.db.Exec(`UPDATE users SET failed_attempts=$1 WHERE id=$2`, newFailed, u.ID)
		}
		return nil, errors.New("неверный email или пароль")
	}

	now := time.Now()
	_, _ = s.db.Exec(`UPDATE users SET failed_attempts=0, locked_until=NULL, last_activity=$1 WHERE id=$2`, now, u.ID)

	token, err := GenerateToken(Claims{
		UserID:   u.ID,
		Email:    u.Email,
		Role:     u.Role,
		FullName: u.FullName,
	}, s.jwtSecret, s.expiryHours)
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

func (s *Service) UpdateActivity(userID int) {
	_, _ = s.db.Exec(`UPDATE users SET last_activity=NOW() WHERE id=$1`, userID)
}
