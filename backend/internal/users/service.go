package users

import (
	"errors"
	"fmt"
	"strings"

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
	// Пустая роль допустима: учётку заводят до назначения роли, доступа
	// к функциональности у неё при этом нет.
	if !auth.IsKnownRole(req.Role) {
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
	if req.Role != nil && !auth.IsKnownRole(*req.Role) {
		return nil, fmt.Errorf("неизвестная роль: %s", *req.Role)
	}
	// role передаём указателем: NULL означает «не меняем», пустая
	// строка — «снять роль». COALESCE различает их правильно, потому
	// что пустая строка не NULL.
	if req.Email != nil {
		email := strings.ToLower(strings.TrimSpace(*req.Email))
		if email == "" || !strings.Contains(email, "@") {
			return nil, errors.New("укажите корректный email")
		}
		// Адрес — логин: два одинаковых сделали бы вход неоднозначным.
		// Проверяем заранее, чтобы вернуть внятную ошибку вместо
		// нарушения уникального индекса.
		var busy int
		_ = s.db.Get(&busy,
			`SELECT COUNT(*) FROM users WHERE lower(email)=$1 AND id<>$2`, email, id)
		if busy > 0 {
			return nil, errors.New("этот email уже занят другим пользователем")
		}
		req.Email = &email
	}

	_, err := s.db.Exec(
		`UPDATE users SET
		   full_name = COALESCE($1, full_name),
		   email     = COALESCE($2, email),
		   role      = COALESCE($3, role),
		   active    = COALESCE($4, active),
		   updated_at = NOW()
		 WHERE id=$5`,
		req.FullName, req.Email, req.Role, req.Active, id,
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

// ─── Доступ к проектам ──────────────────────────────────────────────────

// ProjectAccessFor возвращает проекты пользователя с признаком «может
// редактировать» и названием проекта.
func (s *Service) ProjectAccessFor(userID int) ([]ProjectAccess, error) {
	var rows []ProjectAccess
	err := s.db.Select(&rows,
		`SELECT upp.user_id, upp.project_id, p.name, p.status, upp.can_edit
		   FROM user_project_permissions upp
		   JOIN projects p ON p.id = upp.project_id
		  WHERE upp.user_id = $1
		  ORDER BY p.name`, userID)
	return rows, err
}

// ProjectAccessAll — доступы всех пользователей одним запросом.
// Экран управления показывает их в таблице; без этого на каждую строку
// приходился бы отдельный поход в базу.
func (s *Service) ProjectAccessAll() (map[int][]ProjectAccess, error) {
	var rows []ProjectAccess
	err := s.db.Select(&rows,
		`SELECT upp.user_id, upp.project_id, p.name, p.status, upp.can_edit
		   FROM user_project_permissions upp
		   JOIN projects p ON p.id = upp.project_id
		  ORDER BY upp.user_id, p.name`)
	if err != nil {
		return nil, err
	}
	out := map[int][]ProjectAccess{}
	for _, r := range rows {
		out[r.UserID] = append(out[r.UserID], r)
	}
	return out, nil
}

// SetProjectAccess приводит список проектов пользователя к переданному: чего
// нет в списке — отзывается, что есть — выдаётся или обновляется.
func (s *Service) SetProjectAccess(userID int, req SetProjectsRequest, grantedBy int) ([]int, error) {
	tx, err := s.db.Beginx()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var before []int
	if err = tx.Select(&before,
		`SELECT project_id FROM user_project_permissions WHERE user_id=$1`, userID); err != nil {
		return nil, err
	}

	keep := map[int]bool{}
	for _, it := range req.Projects {
		keep[it.ProjectID] = true
		if _, err = tx.Exec(
			`INSERT INTO user_project_permissions (user_id, project_id, can_edit, granted_by)
			 VALUES ($1,$2,$3,$4)
			 ON CONFLICT (user_id, project_id)
			 DO UPDATE SET can_edit=$3, granted_by=$4, granted_at=NOW()`,
			userID, it.ProjectID, it.CanEdit, grantedBy); err != nil {
			return nil, err
		}
	}

	var revoked []int
	for _, id := range before {
		if !keep[id] {
			revoked = append(revoked, id)
		}
	}
	if len(revoked) > 0 {
		q, args, qerr := sqlx.In(
			`DELETE FROM user_project_permissions WHERE user_id=? AND project_id IN (?)`,
			userID, revoked)
		if qerr != nil {
			return nil, qerr
		}
		if _, err = tx.Exec(tx.Rebind(q), args...); err != nil {
			return nil, err
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return revoked, nil
}
