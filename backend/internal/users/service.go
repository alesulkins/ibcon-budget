package users

import (
	"errors"
	"fmt"
	"slices"

	"github.com/jmoiron/sqlx"

	"ibcon-budget/internal/auth"
)

type Service struct {
	db *sqlx.DB
}

func NewService(db *sqlx.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Create(req CreateRequest, createdBy int) (*User, error) {
	if !slices.Contains(auth.AllRoles, req.Role) {
		return nil, fmt.Errorf("неизвестная роль: %s", req.Role)
	}
	if err := auth.ValidatePassword(req.Password); err != nil {
		return nil, err
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}
	var u User
	err = s.db.QueryRowx(
		`INSERT INTO users (email, password_hash, full_name, role, created_by)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, email, full_name, role, active, failed_attempts, locked_until, created_at, created_by`,
		req.Email, hash, req.FullName, req.Role, createdBy,
	).StructScan(&u)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &u, nil
}

func (s *Service) List() ([]User, error) {
	var users []User
	err := s.db.Select(&users,
		`SELECT id, email, full_name, role, active, failed_attempts, locked_until, created_at, created_by
		 FROM users ORDER BY id`,
	)
	return users, err
}

func (s *Service) Get(id int) (*User, error) {
	var u User
	err := s.db.QueryRowx(
		`SELECT id, email, full_name, role, active, failed_attempts, locked_until, created_at, created_by
		 FROM users WHERE id=$1`, id,
	).StructScan(&u)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Service) Update(id int, req UpdateRequest) (*User, error) {
	if req.Role != nil && !slices.Contains(auth.AllRoles, *req.Role) {
		return nil, fmt.Errorf("неизвестная роль: %s", *req.Role)
	}
	_, err := s.db.Exec(
		`UPDATE users SET
		   full_name = COALESCE($1, full_name),
		   role      = COALESCE($2, role),
		   active    = COALESCE($3, active),
		   updated_at = NOW()
		 WHERE id=$4`,
		req.FullName, req.Role, req.Active, id,
	)
	if err != nil {
		return nil, err
	}
	return s.Get(id)
}

func (s *Service) SetPassword(id int, req SetPasswordRequest) error {
	if err := auth.ValidatePassword(req.Password); err != nil {
		return err
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE users SET password_hash=$1, updated_at=NOW() WHERE id=$2`, hash, id)
	return err
}

// GrantProjectAccess выдаёт право редактирования конкретного бюджета
func (s *Service) GrantProjectAccess(userID, projectID, grantedBy int, canEdit bool) error {
	_, err := s.db.Exec(
		`INSERT INTO user_project_permissions (user_id, project_id, can_edit, granted_by)
		 VALUES ($1,$2,$3,$4)
		 ON CONFLICT (user_id, project_id) DO UPDATE SET can_edit=$3, granted_by=$4, granted_at=NOW()`,
		userID, projectID, canEdit, grantedBy,
	)
	return err
}

// RevokeProjectAccess отзывает доступ к проекту
func (s *Service) RevokeProjectAccess(userID, projectID int) error {
	_, err := s.db.Exec(
		`DELETE FROM user_project_permissions WHERE user_id=$1 AND project_id=$2`,
		userID, projectID,
	)
	return err
}

// HasProjectAccess проверяет, есть ли у пользователя доступ к проекту (индивидуальный)
func (s *Service) HasProjectAccess(userID, projectID int) (exists bool, canEdit bool, err error) {
	var row struct {
		CanEdit bool `db:"can_edit"`
	}
	e := s.db.Get(&row,
		`SELECT can_edit FROM user_project_permissions WHERE user_id=$1 AND project_id=$2`,
		userID, projectID,
	)
	if e != nil {
		return false, false, nil
	}
	return true, row.CanEdit, nil
}

// ProjectsForUser возвращает project_id, к которым у пользователя есть доступ
func (s *Service) ProjectsForUser(userID int) ([]int, error) {
	var ids []int
	err := s.db.Select(&ids,
		`SELECT project_id FROM user_project_permissions WHERE user_id=$1`, userID,
	)
	return ids, err
}

func (s *Service) UnlockUser(id int) error {
	res, err := s.db.Exec(
		`UPDATE users SET failed_attempts=0, locked_until=NULL WHERE id=$1`, id,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("пользователь не найден")
	}
	return nil
}
