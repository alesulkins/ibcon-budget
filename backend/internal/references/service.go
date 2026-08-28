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

// withEditor добавляет к запросу имя того, кто последним менял запись.
// LEFT JOIN, потому что у сидовых записей updated_by пуст.
func withEditor(table, cols string) string {
	return `SELECT ` + cols + `, u.full_name AS updated_by_name
	        FROM ` + table + ` t
	        LEFT JOIN users u ON u.id = t.updated_by`
}

// activeFilter — «только активные» для выпадающих списков форм.
// Списки справочника в интерфейсе управления показывают всё.
func activeFilter(activeOnly bool) string {
	if activeOnly {
		return ` WHERE t.active = TRUE`
	}
	return ``
}

// ---------- Executors ----------

const executorCols = `t.id, t.name, t.full_name, t.profit_tax_rate, t.refinancing_rate,
                      t.active, t.updated_at, t.updated_by`

func (s *Service) ListExecutors(activeOnly bool) ([]Executor, error) {
	q := withEditor("executors", executorCols) + activeFilter(activeOnly) + ` ORDER BY t.name`
	rows := []Executor{}
	return rows, s.db.Select(&rows, q)
}

func (s *Service) GetExecutorByName(name string) (*Executor, error) {
	var e Executor
	err := s.db.QueryRowx(
		withEditor("executors", executorCols)+` WHERE lower(t.name) = lower($1)`, name,
	).StructScan(&e)
	return &e, err
}

func (s *Service) CreateExecutor(name, fullName string, profitTax, refinancing float64, by int) (*Executor, error) {
	var id int
	err := s.db.QueryRowx(
		`INSERT INTO executors (name, full_name, profit_tax_rate, refinancing_rate, updated_by)
		 VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		name, fullName, profitTax, refinancing, by,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	return s.getExecutor(id)
}

func (s *Service) UpdateExecutor(id int, name, fullName *string, profitTax, refinancing *float64, active *bool, by int) (*Executor, error) {
	_, err := s.db.Exec(
		`UPDATE executors SET
		   name             = COALESCE($1, name),
		   full_name        = COALESCE($2, full_name),
		   profit_tax_rate  = COALESCE($3, profit_tax_rate),
		   refinancing_rate = COALESCE($4, refinancing_rate),
		   active           = COALESCE($5, active),
		   updated_at = NOW(), updated_by = $6
		 WHERE id=$7`,
		name, fullName, profitTax, refinancing, active, by, id,
	)
	if err != nil {
		return nil, err
	}
	return s.getExecutor(id)
}

func (s *Service) getExecutor(id int) (*Executor, error) {
	var e Executor
	err := s.db.QueryRowx(withEditor("executors", executorCols)+` WHERE t.id=$1`, id).StructScan(&e)
	return &e, err
}

// ---------- Positions ----------

const positionCols = `t.id, t.name, t.salary, t.active, t.updated_at, t.updated_by`

func (s *Service) ListPositions(activeOnly bool) ([]Position, error) {
	q := withEditor("positions", positionCols) + activeFilter(activeOnly) + ` ORDER BY t.name`
	rows := []Position{}
	return rows, s.db.Select(&rows, q)
}

func (s *Service) CreatePosition(name string, salary float64, by int) (*Position, error) {
	var id int
	err := s.db.QueryRowx(
		`INSERT INTO positions (name, salary, updated_by) VALUES ($1,$2,$3) RETURNING id`,
		name, salary, by,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	return s.getPosition(id)
}

func (s *Service) UpdatePosition(id int, name *string, salary *float64, active *bool, by int) (*Position, error) {
	_, err := s.db.Exec(
		`UPDATE positions SET
		   name   = COALESCE($1, name),
		   salary = COALESCE($2, salary),
		   active = COALESCE($3, active),
		   updated_at = NOW(), updated_by = $4
		 WHERE id=$5`,
		name, salary, active, by, id,
	)
	if err != nil {
		return nil, err
	}
	return s.getPosition(id)
}

func (s *Service) getPosition(id int) (*Position, error) {
	var p Position
	err := s.db.QueryRowx(withEditor("positions", positionCols)+` WHERE t.id=$1`, id).StructScan(&p)
	return &p, err
}

// ---------- WorkModes ----------

const workModeCols = `t.id, t.code, t.full_name, t.active, t.updated_at, t.updated_by`

func (s *Service) ListWorkModes(activeOnly bool) ([]WorkMode, error) {
	q := withEditor("work_modes", workModeCols) + activeFilter(activeOnly) + ` ORDER BY t.code`
	rows := []WorkMode{}
	return rows, s.db.Select(&rows, q)
}

func (s *Service) CreateWorkMode(code, fullName string, by int) (*WorkMode, error) {
	var id int
	err := s.db.QueryRowx(
		`INSERT INTO work_modes (code, full_name, updated_by) VALUES ($1,$2,$3) RETURNING id`,
		code, fullName, by,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	return s.getWorkMode(id)
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
	return s.getWorkMode(id)
}

func (s *Service) getWorkMode(id int) (*WorkMode, error) {
	var wm WorkMode
	err := s.db.QueryRowx(withEditor("work_modes", workModeCols)+` WHERE t.id=$1`, id).StructScan(&wm)
	return &wm, err
}

// ---------- CostItems ----------

const costItemCols = `t.id, t.name, t.is_calculated, t.active, t.sort_order, t.updated_at, t.updated_by`

func (s *Service) ListCostItems(activeOnly bool) ([]CostItem, error) {
	q := withEditor("cost_items", costItemCols) + activeFilter(activeOnly) + ` ORDER BY t.sort_order, t.name`
	rows := []CostItem{}
	return rows, s.db.Select(&rows, q)
}

// CreateCostItem — только нерасчётная статья: за расчётной стоит формула
// листа 4.x, добавить такую через справочник нельзя.
func (s *Service) CreateCostItem(name string, by int) (*CostItem, error) {
	var id int
	err := s.db.QueryRowx(
		`INSERT INTO cost_items (name, is_calculated, sort_order, updated_by)
		 VALUES ($1, FALSE, (SELECT COALESCE(MAX(sort_order),0)+1 FROM cost_items), $2)
		 RETURNING id`,
		name, by,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	return s.getCostItem(id)
}

// UpdateCostItem — расчётные статьи неприкосновенны целиком: их нельзя ни
// переименовать, ни деактивировать. Имя статьи связывает лист расчёта со
// строкой 2.Бюджет, а деактивация убрала бы из бюджета посчитанную сумму.
func (s *Service) UpdateCostItem(id int, name *string, active *bool, by int) (*CostItem, error) {
	var isCalc bool
	if err := s.db.Get(&isCalc, `SELECT is_calculated FROM cost_items WHERE id=$1`, id); err != nil {
		return nil, err
	}
	if isCalc {
		return nil, fmt.Errorf("расчётную статью затрат нельзя изменить или деактивировать")
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
	return s.getCostItem(id)
}

func (s *Service) getCostItem(id int) (*CostItem, error) {
	var ci CostItem
	err := s.db.QueryRowx(withEditor("cost_items", costItemCols)+` WHERE t.id=$1`, id).StructScan(&ci)
	return &ci, err
}
