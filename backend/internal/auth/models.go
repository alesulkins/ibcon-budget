package auth

// Роли и права доступа объявлены в permissions.go — там же лежит
// ролевая матрица, чтобы код ролей и их полномочия не расходились.

type Claims struct {
	UserID   int    `json:"user_id"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	FullName string `json:"full_name"`
}
