package users

import "time"

type User struct {
	ID       int    `db:"id"             json:"id"`
	Email    string `db:"email"          json:"email"`
	FullName string `db:"full_name"      json:"full_name"`
	// Role пустая — роль не назначена. Такой пользователь входит в
	// систему, но не видит функциональности (ТЗ, раздел о доступах).
	Role           string     `db:"role"           json:"role"`
	Active         bool       `db:"active"         json:"active"`
	FailedAttempts int        `db:"failed_attempts" json:"failed_attempts"`
	LockedUntil    *time.Time `db:"locked_until"   json:"locked_until,omitempty"`
	CreatedAt      time.Time  `db:"created_at"     json:"created_at"`
	CreatedBy      *int       `db:"created_by"     json:"created_by,omitempty"`

	// Projects и Grants заполняет обработчик списка: экран управления
	// пользователями показывает доступ к проектам и дополнительные
	// права прямо в таблице.
	Projects []ProjectAccess `db:"-" json:"projects,omitempty"`
	Grants   []Grant         `db:"-" json:"grants,omitempty"`
}

// ProjectAccess — назначение пользователя на проект.
type ProjectAccess struct {
	UserID    int    `db:"user_id"     json:"user_id"`
	ProjectID int    `db:"project_id"  json:"project_id"`
	Name      string `db:"name"        json:"name"`
	Status    string `db:"status"      json:"status"`
	CanEdit   bool   `db:"can_edit"    json:"can_edit"`
}

// Grant — индивидуальное право сверх роли. Дублирует access.Grant,
// чтобы пакет users не тянул зависимость ради одной структуры ответа.
type Grant struct {
	Permission  string  `json:"permission"`
	ProjectID   *int    `json:"project_id"`
	ProjectName *string `json:"project_name"`
}

type CreateRequest struct {
	Email    string `json:"email"     binding:"required,email"`
	Password string `json:"password"  binding:"required"`
	FullName string `json:"full_name" binding:"required"`
	// Role не обязательна: ТЗ допускает создание учётки без роли —
	// доступа к функциональности у неё не будет до её назначения.
	Role string `json:"role"`
}

type UpdateRequest struct {
	FullName *string `json:"full_name"`
	Role     *string `json:"role"`
	Active   *bool   `json:"active"`
}

type SetPasswordRequest struct {
	Password string `json:"password" binding:"required"`
}

// SetProjectsRequest — полный список проектов пользователя.
// Приходит из множественного выбора на экране управления: что не
// пришло, то отзывается.
type SetProjectsRequest struct {
	Projects []struct {
		ProjectID int  `json:"project_id"`
		CanEdit   bool `json:"can_edit"`
	} `json:"projects"`
}

// GrantRequest — выдача или отзыв индивидуального права.
// ProjectID == nil — право на все проекты.
type GrantRequest struct {
	Permission string `json:"permission" binding:"required"`
	ProjectID  *int   `json:"project_id"`
}
