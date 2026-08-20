package projects

import "time"

// Статусы проекта
const (
	StatusProspect   = "prospect"   // перспективный
	StatusActive     = "active"     // действующий
	StatusSuspended  = "suspended"  // приостановлен
	StatusCompleted  = "completed"  // завершён
	StatusUnrealized = "unrealized" // не реализован
)

// validTransitions описывает допустимые переходы между статусами проекта
var validTransitions = map[string][]string{
	StatusProspect:   {StatusActive, StatusSuspended, StatusCompleted, StatusUnrealized},
	StatusActive:     {StatusProspect, StatusSuspended, StatusCompleted, StatusUnrealized},
	StatusSuspended:  {StatusProspect, StatusActive, StatusCompleted, StatusUnrealized},
	StatusCompleted:  {StatusProspect, StatusActive},
	StatusUnrealized: {StatusProspect, StatusActive},
}

// Статусы, при которых создание/изменение бюджета запрещено
var frozenForBudget = map[string]bool{
	StatusCompleted:  true,
	StatusUnrealized: true,
}

func CanTransition(from, to string) bool {
	for _, s := range validTransitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

func IsFrozenForBudget(status string) bool {
	return frozenForBudget[status]
}

type Project struct {
	ID             int       `db:"id"               json:"id"`
	Name           string    `db:"name"             json:"name"`
	Customer       string    `db:"customer"         json:"customer"`
	ExecutorID     int       `db:"executor_id"      json:"executor_id"`
	ExecutorName   string    `db:"executor_name"    json:"executor_name,omitempty"`
	Location       string    `db:"location"         json:"location"`
	StartDate      time.Time `db:"start_date"       json:"start_date"`
	DurationMonths int       `db:"duration_months"  json:"duration_months"`
	EndDate        time.Time `db:"end_date"         json:"end_date"`
	Director       string    `db:"director"         json:"director"`
	Manager        string    `db:"manager"          json:"manager"`
	Administrator  string    `db:"administrator"    json:"administrator"`
	Economist      string    `db:"economist"        json:"economist"`
	Status         string    `db:"status"           json:"status"`
	CreatedAt      time.Time `db:"created_at"       json:"created_at"`
	CreatedBy      int       `db:"created_by"       json:"created_by"`
	CreatedByName  string    `db:"created_by_name"  json:"created_by_name,omitempty"`
	UpdatedAt      time.Time `db:"updated_at"       json:"updated_at"`
}

// ProjectListItem — облегчённая запись для реестра проектов
type ProjectListItem struct {
	ID             int      `db:"id"              json:"id"`
	Name           string   `db:"name"            json:"name"`
	Customer       string   `db:"customer"        json:"customer"`
	ExecutorName   string   `db:"executor_name"   json:"executor_name"`
	Director       string   `db:"director"        json:"director"`
	Manager        string   `db:"manager"         json:"manager"`
	Administrator  string   `db:"administrator"   json:"administrator"`
	Economist      string   `db:"economist"       json:"economist"`
	Status         string   `db:"status"          json:"status"`
	BudgetStatus   *string  `db:"budget_status"   json:"budget_status"`
	CostNoVat      *float64 `db:"cost_no_vat"     json:"cost_no_vat"`
	Profitability  *float64 `db:"profitability"   json:"profitability"`
	CreatedAt      string   `db:"created_at"      json:"created_at"`
	CreatedByName  string   `db:"created_by_name" json:"created_by_name"`
}

type CreateRequest struct {
	Name           string `json:"name"            binding:"required"`
	Customer       string `json:"customer"        binding:"required"`
	ExecutorID     int    `json:"executor_id"     binding:"required"`
	Location       string `json:"location"        binding:"required"`
	StartDate      string `json:"start_date"      binding:"required"` // DD.MM.YYYY
	DurationMonths int    `json:"duration_months" binding:"required,min=1"`
	Director       string `json:"director"        binding:"required"`
	Manager        string `json:"manager"         binding:"required"`
	Administrator  string `json:"administrator"   binding:"required"`
	Economist      string `json:"economist"       binding:"required"`
	Status         string `json:"status"          binding:"required"`
}

type UpdateRequest struct {
	Name           *string `json:"name"`
	Customer       *string `json:"customer"`
	ExecutorID     *int    `json:"executor_id"`
	Location       *string `json:"location"`
	StartDate      *string `json:"start_date"`
	DurationMonths *int    `json:"duration_months"`
	Director       *string `json:"director"`
	Manager        *string `json:"manager"`
	Administrator  *string `json:"administrator"`
	Economist      *string `json:"economist"`
}

type ChangeStatusRequest struct {
	Status  string `json:"status"  binding:"required"`
	Comment string `json:"comment" binding:"required"`
}

type ListParams struct {
	Search  string
	Status  string
	Limit   int
	Offset  int
	UserIDs []int // ограничение по доступным проектам (nil = все)
}
