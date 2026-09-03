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

	// Salary — оклад ПО УМОЛЧАНИЮ: применяется там, где для города проекта
	// своей ставки не задали.
	Salary float64 `db:"salary" json:"salary"`

	// IsITR — инженерно-технический работник.
	IsITR bool `db:"is_itr" json:"is_itr"`

	// CitySalaries — оклад по городам: в разных городах за одну и ту же работу
	// платят по-разному, и в мастер подставляется ставка города проекта
	// (projects.location).
	CitySalaries []CitySalary `db:"-" json:"city_salaries,omitempty"`

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

// City — город из справочника городов. Список пополняется прямо в форме
// должности: держать город свободной строкой нельзя, опечатка создала бы
// «второй Петербург» с отдельными окладами.
type City struct {
	ID        int       `db:"id"         json:"id"`
	Name      string    `db:"name"       json:"name"`
	Active    bool      `db:"active"     json:"active"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// CitySalary — оклад должности в конкретном городе.
type CitySalary struct {
	CityID   int     `db:"city_id"   json:"city_id"`
	CityName string  `db:"city_name" json:"city_name"`
	Salary   float64 `db:"salary"    json:"salary"`
}
