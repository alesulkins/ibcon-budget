package auth

// Роли системы
const (
	RoleGE         = "GE"         // Главный экономист
	RoleEP         = "EP"         // Экономист проекта
	RoleIP         = "IP"         // Инициатор проекта
	RoleRP         = "RP"         // Руководитель проекта
	RoleAP         = "AP"         // Администратор проекта
	RoleManagement = "MANAGEMENT" // Руководство
)

var AllRoles = []string{RoleGE, RoleEP, RoleIP, RoleRP, RoleAP, RoleManagement}

type Claims struct {
	UserID   int    `json:"user_id"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	FullName string `json:"full_name"`
}
