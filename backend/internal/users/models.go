package users

import "time"

type User struct {
	ID           int        `db:"id"             json:"id"`
	Email        string     `db:"email"          json:"email"`
	FullName     string     `db:"full_name"      json:"full_name"`
	Role         string     `db:"role"           json:"role"`
	Active       bool       `db:"active"         json:"active"`
	FailedAttempts int      `db:"failed_attempts" json:"failed_attempts"`
	LockedUntil  *time.Time `db:"locked_until"   json:"locked_until,omitempty"`
	CreatedAt    time.Time  `db:"created_at"     json:"created_at"`
	CreatedBy    *int       `db:"created_by"     json:"created_by,omitempty"`
}

type CreateRequest struct {
	Email    string `json:"email"     binding:"required,email"`
	Password string `json:"password"  binding:"required"`
	FullName string `json:"full_name" binding:"required"`
	Role     string `json:"role"      binding:"required"`
}

type UpdateRequest struct {
	FullName *string `json:"full_name"`
	Role     *string `json:"role"`
	Active   *bool   `json:"active"`
}

type SetPasswordRequest struct {
	Password string `json:"password" binding:"required"`
}
