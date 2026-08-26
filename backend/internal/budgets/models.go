package budgets

import "time"

// Статусы версии бюджета
const (
	StatusDraft       = "draft"        // черновик
	StatusUnderReview = "under_review" // на согласовании
	StatusApproved    = "approved"     // согласован
	StatusArchive     = "archive"      // архив
)

// validTransitions — допустимые переходы статусов бюджета
var validTransitions = map[string][]string{
	StatusDraft:       {StatusUnderReview},
	StatusUnderReview: {StatusApproved, StatusDraft},
	StatusApproved:    {}, // только через создание новой версии → авто-архивация
	StatusArchive:     {}, // необратимо
}

func CanTransition(from, to string) bool {
	for _, s := range validTransitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

// Версия бюджета
type BudgetVersion struct {
	ID        int `db:"id"            json:"id"`
	BudgetID  int `db:"budget_id"     json:"budget_id"`
	ProjectID int `db:"project_id"    json:"project_id"`
	// VersionNo — порядковый номер версии внутри проекта (1, 2, 3…).
	// Присваивается при создании, есть у каждой версии.
	// Вместе с ID проекта даёт «ID бюджета» вида 1.2 (ТЗ, таблица 4).
	VersionNo int `db:"version_no"    json:"version_no"`
	// VersionLabel — историческая метка «в.N», проставлялась только при
	// уходе в архив. Оставлена для совместимости, в интерфейсе не нужна.
	VersionLabel  *string    `db:"version_label" json:"version_label"`
	Status        string     `db:"status"        json:"status"`
	Comment       *string    `db:"comment"       json:"comment"`
	CostNoVat     *float64   `db:"cost_no_vat"   json:"cost_no_vat"`
	Profitability *float64   `db:"profitability" json:"profitability"`
	CostOverride  *float64   `db:"cost_override" json:"cost_override"`
	CreatedAt     time.Time  `db:"created_at"    json:"created_at"`
	CreatedBy     int        `db:"created_by"    json:"created_by"`
	CreatedByName string     `db:"created_by_name" json:"created_by_name,omitempty"`
	UpdatedAt     time.Time  `db:"updated_at"    json:"updated_at"`
	ApprovedAt    *time.Time `db:"approved_at"   json:"approved_at,omitempty"`
	CopiedFrom    *int       `db:"copied_from"   json:"copied_from,omitempty"`
}

type CreateVersionRequest struct {
	CopyFromID *int   `json:"copy_from_id"` // если задан — копируем данные
	Comment    string `json:"comment"`
}

type ChangeStatusRequest struct {
	Status  string `json:"status"  binding:"required"`
	Comment string `json:"comment" binding:"required"`
}

type UpdateCostOverrideRequest struct {
	CostOverride *float64 `json:"cost_override"`
}
