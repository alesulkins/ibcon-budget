package references

import (
	"fmt"

	"github.com/jmoiron/sqlx"
)

type Service struct {
	db *sqlx.DB
}

func NewService(db *sqlx.DB) *Service {
	return &Service{db: db}
}

// ---------- Executors ----------

func (s *Service) ListExecutors(activeOnly bool) ([]Executor, error) {
	q := `SELECT id, name, full_name, active, updated_at, updated_by FROM executors`
	if activeOnly {
		q += ` WHERE active=TRUE`
	}
	q += ` ORDER BY name`
	var rows []Executor
	return rows, s.db.Select(&rows, q)
}

func (s *Service) CreateExecutor(name, fullName string, by int) (*Executor, error) {
	var e Executor
	err := s.db.QueryRowx(
		`INSERT INTO executors (name, full_name, updated_by) VALUES ($1,$2,$3)
		 RETURNING id, name, full_name, active, updated_at, updated_by`,
		name, fullName, by,
	).StructScan(&e)
	return &e, err
}

func (s *Service) UpdateExecutor(id int, name, fullName *string, active *bool, by int) (*Executor, error) {
	_, err := s.db.Exec(
		`UPDATE executors SET
		   name      = COALESCE($1, name),
		   full_name = COALESCE($2, full_name),
		   active    = COALESCE($3, active),
		   updated_at = NOW(), updated_by = $4
		 WHERE id=$5`,
		name, fullName, active, by, id,
	)
	if err != nil {
		return nil, err
	}
	var e Executor
	err = s.db.QueryRowx(
		`SELECT id, name, full_name, active, updated_at, updated_by FROM executors WHERE id=$1`, id,
	).StructScan(&e)
	return &e, err
}

// ---------- Positions ----------

func (s *Service) ListPositions(activeOnly bool) ([]Position, error) {
	q := `SELECT id, name, active, updated_at, updated_by FROM positions`
	if activeOnly {
		q += ` WHERE active=TRUE`
	}
	q += ` ORDER BY name`
	var rows []Position
	return rows, s.db.Select(&rows, q)
}

func (s *Service) CreatePosition(name string, by int) (*Position, error) {
	var p Position
	err := s.db.QueryRowx(
		`INSERT INTO positions (name, updated_by) VALUES ($1,$2)
		 RETURNING id, name, active, updated_at, updated_by`,
		name, by,
	).StructScan(&p)
	return &p, err
}

func (s *Service) UpdatePosition(id int, name *string, active *bool, by int) (*Position, error) {
	_, err := s.db.Exec(
		`UPDATE positions SET
		   name    = COALESCE($1, name),
		   active  = COALESCE($2, active),
		   updated_at = NOW(), updated_by = $3
		 WHERE id=$4`,
		name, active, by, id,
	)
	if err != nil {
		return nil, err
	}
	var p Position
	err = s.db.QueryRowx(
		`SELECT id, name, active, updated_at, updated_by FROM positions WHERE id=$1`, id,
	).StructScan(&p)
	return &p, err
}

// ---------- WorkModes ----------

func (s *Service) ListWorkModes(activeOnly bool) ([]WorkMode, error) {
	q := `SELECT id, code, full_name, active, updated_at, updated_by FROM work_modes`
	if activeOnly {
		q += ` WHERE active=TRUE`
	}
	q += ` ORDER BY code`
	var rows []WorkMode
	return rows, s.db.Select(&rows, q)
}

func (s *Service) CreateWorkMode(code, fullName string, by int) (*WorkMode, error) {
	var wm WorkMode
	err := s.db.QueryRowx(
		`INSERT INTO work_modes (code, full_name, updated_by) VALUES ($1,$2,$3)
		 RETURNING id, code, full_name, active, updated_at, updated_by`,
		code, fullName, by,
	).StructScan(&wm)
	return &wm, err
}

func (s *Service) UpdateWorkMode(id int, code, fullName *string, active *bool, by int) (*WorkMode, error) {
	_, err := s.db.Exec(
		`UPDATE work_modes SET
		   code      = COALESCE($1, code),
		   full_name = COALESCE($2, full_name),
		   active    = COALESCE($3, active),
		   updated_at = NOW(), updated_by = $4
		 WHERE id=$5`,
		code, fullName, active, by, id,
	)
	if err != nil {
		return nil, err
	}
	var wm WorkMode
	err = s.db.QueryRowx(
		`SELECT id, code, full_name, active, updated_at, updated_by FROM work_modes WHERE id=$1`, id,
	).StructScan(&wm)
	return &wm, err
}

// ---------- CostItems ----------

func (s *Service) ListCostItems(activeOnly bool) ([]CostItem, error) {
	q := `SELECT id, name, is_calculated, active, sort_order, updated_at, updated_by FROM cost_items`
	if activeOnly {
		q += ` WHERE active=TRUE`
	}
	q += ` ORDER BY sort_order, name`
	var rows []CostItem
	return rows, s.db.Select(&rows, q)
}

func (s *Service) CreateCostItem(name string, isCalculated bool, by int) (*CostItem, error) {
	if isCalculated {
		return nil, fmt.Errorf("расчётные статьи нельзя добавлять через справочник")
	}
	var ci CostItem
	err := s.db.QueryRowx(
		`INSERT INTO cost_items (name, is_calculated, updated_by)
		 VALUES ($1, FALSE, $2)
		 RETURNING id, name, is_calculated, active, sort_order, updated_at, updated_by`,
		name, by,
	).StructScan(&ci)
	return &ci, err
}

func (s *Service) UpdateCostItem(id int, name *string, active *bool, by int) (*CostItem, error) {
	// Расчётные статьи нельзя деактивировать
	var isCalc bool
	_ = s.db.Get(&isCalc, `SELECT is_calculated FROM cost_items WHERE id=$1`, id)
	if isCalc && active != nil && !*active {
		return nil, fmt.Errorf("расчётные статьи нельзя деактивировать")
	}
	_, err := s.db.Exec(
		`UPDATE cost_items SET
		   name    = COALESCE($1, name),
		   active  = COALESCE($2, active),
		   updated_at = NOW(), updated_by = $3
		 WHERE id=$4`,
		name, active, by, id,
	)
	if err != nil {
		return nil, err
	}
	var ci CostItem
	err = s.db.QueryRowx(
		`SELECT id, name, is_calculated, active, sort_order, updated_at, updated_by FROM cost_items WHERE id=$1`, id,
	).StructScan(&ci)
	return &ci, err
}
