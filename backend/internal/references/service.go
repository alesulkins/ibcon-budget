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

const positionCols = `t.id, t.name, t.salary, t.is_itr, t.active, t.updated_at, t.updated_by`

func (s *Service) ListPositions(activeOnly bool) ([]Position, error) {
	q := withEditor("positions", positionCols) + activeFilter(activeOnly) + ` ORDER BY t.name`
	rows := []Position{}
	if err := s.db.Select(&rows, q); err != nil {
		return rows, err
	}
	return rows, s.attachCitySalaries(rows)
}

// attachCitySalaries добирает оклады по городам одним запросом на весь
// список: по запросу на должность список из полусотни строк лёг бы.
func (s *Service) attachCitySalaries(rows []Position) error {
	if len(rows) == 0 {
		return nil
	}
	var flat []struct {
		PositionID int     `db:"position_id"`
		CityID     int     `db:"city_id"`
		CityName   string  `db:"city_name"`
		Salary     float64 `db:"salary"`
	}
	err := s.db.Select(&flat, `
		SELECT pcs.position_id, pcs.city_id, c.name AS city_name, pcs.salary
		FROM position_city_salaries pcs
		JOIN cities c ON c.id = pcs.city_id
		ORDER BY c.name`)
	if err != nil {
		return err
	}

	byPosition := make(map[int][]CitySalary, len(rows))
	for _, f := range flat {
		byPosition[f.PositionID] = append(byPosition[f.PositionID],
			CitySalary{CityID: f.CityID, CityName: f.CityName, Salary: f.Salary})
	}
	for i := range rows {
		rows[i].CitySalaries = byPosition[rows[i].ID]
	}
	return nil
}

// SalaryFor — оклад должности в городе location.
func (s *Service) SalaryFor(positionName, location string) (float64, error) {
	var salary float64
	err := s.db.Get(&salary, `
		SELECT COALESCE(
			(SELECT pcs.salary
			   FROM position_city_salaries pcs
			   JOIN cities c ON c.id = pcs.city_id
			  WHERE pcs.position_id = p.id
			    AND lower(btrim(c.name)) = lower(btrim($2))),
			p.salary)
		FROM positions p
		WHERE lower(btrim(p.name)) = lower(btrim($1))`,
		positionName, location)
	return salary, err
}

// ---------- Cities ----------

func (s *Service) ListCities(activeOnly bool) ([]City, error) {
	q := `SELECT id, name, active, updated_at FROM cities`
	if activeOnly {
		q += ` WHERE active = TRUE`
	}
	q += ` ORDER BY name`
	rows := []City{}
	return rows, s.db.Select(&rows, q)
}

// CreateCity добавляет город. Повторное имя не ошибка: возвращаем
// существующий — форма должности добавляет город на лету, и «уже есть»
// там не отказ, а обычный исход.
func (s *Service) CreateCity(name string, by int) (*City, error) {
	var c City
	err := s.db.QueryRowx(`
		INSERT INTO cities (name, updated_by) VALUES ($1, $2)
		ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
		RETURNING id, name, active, updated_at`, name, by).StructScan(&c)
	return &c, err
}

// SetCitySalaries заменяет оклады должности по городам на переданные.
// Город без ставки в списке — оклад для него снимается: иначе снятую
// строку было бы нечем удалить.
func (s *Service) SetCitySalaries(positionID int, salaries []CitySalary, by int) error {
	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(
		`DELETE FROM position_city_salaries WHERE position_id = $1`, positionID); err != nil {
		return err
	}
	for _, cs := range salaries {
		if _, err := tx.Exec(`
			INSERT INTO position_city_salaries (position_id, city_id, salary, updated_by)
			VALUES ($1, $2, $3, $4)`, positionID, cs.CityID, cs.Salary, by); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Service) CreatePosition(name string, salary float64, isITR bool, by int) (*Position, error) {
	var id int
	err := s.db.QueryRowx(
		`INSERT INTO positions (name, salary, is_itr, updated_by) VALUES ($1,$2,$3,$4) RETURNING id`,
		name, salary, isITR, by,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	return s.getPosition(id)
}

func (s *Service) UpdatePosition(id int, name *string, salary *float64, isITR, active *bool, by int) (*Position, error) {
	_, err := s.db.Exec(
		`UPDATE positions SET
		   name   = COALESCE($1, name),
		   salary = COALESCE($2, salary),
		   is_itr = COALESCE($3, is_itr),
		   active = COALESCE($4, active),
		   updated_at = NOW(), updated_by = $5
		 WHERE id=$6`,
		name, salary, isITR, active, by, id,
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

// ITRPositions — названия должностей с признаком ИТР, в нижнем регистре.
func (s *Service) ITRPositions() (map[string]bool, error) {
	var names []string
	if err := s.db.Select(&names,
		`SELECT lower(btrim(name)) FROM positions WHERE is_itr = TRUE`); err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return set, nil
}
