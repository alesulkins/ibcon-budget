package references

import "time"

type Executor struct {
	ID        int       `db:"id"         json:"id"`
	Name      string    `db:"name"       json:"name"`
	FullName  string    `db:"full_name"  json:"full_name"`
	Active    bool      `db:"active"     json:"active"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
	UpdatedBy *int      `db:"updated_by" json:"updated_by,omitempty"`
}

type Position struct {
	ID        int       `db:"id"         json:"id"`
	Name      string    `db:"name"       json:"name"`
	Active    bool      `db:"active"     json:"active"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
	UpdatedBy *int      `db:"updated_by" json:"updated_by,omitempty"`
}

type WorkMode struct {
	ID        int       `db:"id"         json:"id"`
	Code      string    `db:"code"       json:"code"`
	FullName  string    `db:"full_name"  json:"full_name"`
	Active    bool      `db:"active"     json:"active"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
	UpdatedBy *int      `db:"updated_by" json:"updated_by,omitempty"`
}

type CostItem struct {
	ID           int       `db:"id"            json:"id"`
	Name         string    `db:"name"          json:"name"`
	IsCalculated bool      `db:"is_calculated" json:"is_calculated"`
	Active       bool      `db:"active"        json:"active"`
	SortOrder    int       `db:"sort_order"    json:"sort_order"`
	UpdatedAt    time.Time `db:"updated_at"    json:"updated_at"`
	UpdatedBy    *int      `db:"updated_by"    json:"updated_by,omitempty"`
}
