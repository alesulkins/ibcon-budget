package references

import "time"

// Общее для всех справочников:
//   - записи не удаляются физически, только деактивируются (active);
//   - деактивация не трогает уже сохранённые бюджеты — они хранят
//     подставленное значение (snapshot), а не ссылку на справочник;
//   - UpdatedAt/UpdatedBy фиксируют, кто и когда менял запись;
//     UpdatedByName подставляется списком для показа в интерфейсе.

type Executor struct {
	ID       int    `db:"id"        json:"id"`
	Name     string `db:"name"      json:"name"`
	FullName string `db:"full_name" json:"full_name"`

	// Справочные значения — проценты (25 = 25%). В мастере проекта
	// подставляются в ячейки по умолчанию и правятся вручную.
	ProfitTaxRate   float64 `db:"profit_tax_rate"   json:"profit_tax_rate"`
	RefinancingRate float64 `db:"refinancing_rate"  json:"refinancing_rate"`

	Active        bool      `db:"active"          json:"active"`
	UpdatedAt     time.Time `db:"updated_at"      json:"updated_at"`
	UpdatedBy     *int      `db:"updated_by"      json:"updated_by,omitempty"`
	UpdatedByName *string   `db:"updated_by_name" json:"updated_by_name,omitempty"`
}

type Position struct {
	ID   int    `db:"id"   json:"id"`
	Name string `db:"name" json:"name"`

	// Salary — оклад по умолчанию для шага «Сотрудники» мастера.
	// Ручной ввод главного экономиста; экономист проекта может изменить
	// подставленное значение в самой форме, подтверждения не требуется.
	Salary float64 `db:"salary" json:"salary"`

	Active        bool      `db:"active"          json:"active"`
	UpdatedAt     time.Time `db:"updated_at"      json:"updated_at"`
	UpdatedBy     *int      `db:"updated_by"      json:"updated_by,omitempty"`
	UpdatedByName *string   `db:"updated_by_name" json:"updated_by_name,omitempty"`
}

type WorkMode struct {
	ID            int       `db:"id"              json:"id"`
	Code          string    `db:"code"            json:"code"`
	FullName      string    `db:"full_name"       json:"full_name"`
	Active        bool      `db:"active"          json:"active"`
	UpdatedAt     time.Time `db:"updated_at"      json:"updated_at"`
	UpdatedBy     *int      `db:"updated_by"      json:"updated_by,omitempty"`
	UpdatedByName *string   `db:"updated_by_name" json:"updated_by_name,omitempty"`
}

type CostItem struct {
	ID   int    `db:"id"   json:"id"`
	Name string `db:"name" json:"name"`

	// IsCalculated — статья, сумма которой приходит из расчётного листа
	// (4.2–4.12), а не вводится вручную. Такие статьи нельзя ни добавить,
	// ни переименовать, ни деактивировать: за каждой стоит формула.
	IsCalculated bool `db:"is_calculated" json:"is_calculated"`

	Active        bool      `db:"active"          json:"active"`
	SortOrder     int       `db:"sort_order"      json:"sort_order"`
	UpdatedAt     time.Time `db:"updated_at"      json:"updated_at"`
	UpdatedBy     *int      `db:"updated_by"      json:"updated_by,omitempty"`
	UpdatedByName *string   `db:"updated_by_name" json:"updated_by_name,omitempty"`
}
