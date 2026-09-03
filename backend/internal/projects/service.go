package projects

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

type Service struct {
	db *sqlx.DB
}

func NewService(db *sqlx.DB) *Service {
	return &Service{db: db}
}

func parseDate(s string) (time.Time, error) {
	// Принимаем DD.MM.YYYY и YYYY-MM-DD
	for _, layout := range []string{"02.01.2006", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("неверный формат даты: %s (ожидается ДД.ММ.ГГГГ)", s)
}

func (s *Service) Create(req CreateRequest, createdBy int) (*Project, error) {
	if _, ok := validTransitions[req.Status]; !ok {
		if req.Status != StatusProspect && req.Status != StatusActive {
			return nil, fmt.Errorf("недопустимый статус: %s", req.Status)
		}
	}
	startDate, err := parseDate(req.StartDate)
	if err != nil {
		return nil, err
	}
	endDate := startDate.AddDate(0, req.DurationMonths, 0)

	// Приводим текстовые поля к единому виду: названия — с заглавной,
	// ФИО — к формату «Фамилия И.О.». Делаем это на сервере, чтобы
	// правило действовало независимо от того, откуда пришли данные.
	req.Name = capitalizeFirst(req.Name)
	req.Customer = capitalizeFirst(req.Customer)
	req.Location = capitalizeFirst(req.Location)
	req.Director = normalizeFullName(req.Director)
	req.Manager = normalizeFullName(req.Manager)
	req.Administrator = normalizeFullName(req.Administrator)
	req.Economist = normalizeFullName(req.Economist)

	var p Project
	err = s.db.QueryRowx(`
		INSERT INTO projects
		  (name, customer, executor_id, location, start_date, duration_months, end_date,
		   director, manager, administrator, economist, status, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		RETURNING id, name, customer, executor_id, location, start_date, duration_months, end_date,
		          director, manager, administrator, economist, status, created_at, created_by, updated_at`,
		req.Name, req.Customer, req.ExecutorID, req.Location,
		startDate, req.DurationMonths, endDate,
		req.Director, req.Manager, req.Administrator, req.Economist,
		req.Status, createdBy,
	).StructScan(&p)
	return &p, err
}

func (s *Service) Get(id int) (*Project, error) {
	var p Project
	err := s.db.QueryRowx(`
		SELECT p.id, p.name, p.customer, p.executor_id, e.name AS executor_name,
		       p.location, p.start_date, p.duration_months, p.end_date,
		       p.director, p.manager, p.administrator, p.economist, p.status,
		       p.created_at, p.created_by, u.full_name AS created_by_name, p.updated_at
		FROM projects p
		JOIN executors e ON e.id = p.executor_id
		JOIN users u ON u.id = p.created_by
		WHERE p.id=$1`, id,
	).StructScan(&p)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Service) List(params ListParams) ([]ProjectListItem, int, error) {
	if params.Limit <= 0 {
		params.Limit = 50
	}

	var conditions []string
	args := []any{}
	argN := 1

	if params.Search != "" {
		conditions = append(conditions, fmt.Sprintf(
			"(p.name ILIKE $%d OR p.customer ILIKE $%d OR p.director ILIKE $%d OR p.manager ILIKE $%d)",
			argN, argN, argN, argN,
		))
		args = append(args, "%"+params.Search+"%")
		argN++
	}
	if params.Status != "" {
		conditions = append(conditions, fmt.Sprintf("p.status=$%d", argN))
		args = append(args, params.Status)
		argN++
	}
	if len(params.UserIDs) > 0 {
		placeholders := make([]string, len(params.UserIDs))
		for i, id := range params.UserIDs {
			placeholders[i] = fmt.Sprintf("$%d", argN)
			args = append(args, id)
			argN++
		}
		conditions = append(conditions, fmt.Sprintf("p.id IN (%s)", strings.Join(placeholders, ",")))
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Подзапрос: приоритетный статус бюджета и кэши стоимости/рентабельности
	baseQuery := fmt.Sprintf(`
		FROM projects p
		JOIN executors e ON e.id = p.executor_id
		JOIN users u ON u.id = p.created_by
		LEFT JOIN LATERAL (
			SELECT bv.status AS budget_status,
			       bv.cost_no_vat,
			       bv.profitability
			FROM budgets b
			JOIN budget_versions bv ON bv.budget_id = b.id
			WHERE b.project_id = p.id
			ORDER BY
				CASE bv.status
					WHEN 'approved'      THEN 1
					WHEN 'under_review'  THEN 2
					WHEN 'draft'         THEN 3
					WHEN 'archive'       THEN 4
					ELSE 5
				END
			LIMIT 1
		) latest ON TRUE
		%s`, where)

	var total int
	countArgs := make([]any, len(args))
	copy(countArgs, args)
	if err := s.db.Get(&total, "SELECT COUNT(*) "+baseQuery, countArgs...); err != nil {
		return nil, 0, err
	}

	limitArgs := append(args, params.Limit, params.Offset)
	rows := []ProjectListItem{}
	err := s.db.Select(&rows, fmt.Sprintf(`
		SELECT p.id, p.name, p.customer, e.name AS executor_name,
		       p.director, p.manager, p.administrator, p.economist, p.status,
		       latest.budget_status,
		       latest.cost_no_vat,
		       latest.profitability,
		       p.created_at,
		       u.full_name AS created_by_name
		%s ORDER BY p.id DESC LIMIT $%d OFFSET $%d`,
		baseQuery, argN, argN+1,
	), limitArgs...)
	return rows, total, err
}

func (s *Service) Update(id int, req UpdateRequest, updatedBy int) (*Project, error) {
	var startDate *time.Time
	var endDate *time.Time
	if req.StartDate != nil {
		d, err := parseDate(*req.StartDate)
		if err != nil {
			return nil, err
		}
		startDate = &d
	}

	// Читаем текущие значения для пересчёта end_date
	var cur struct {
		StartDate      time.Time `db:"start_date"`
		DurationMonths int       `db:"duration_months"`
	}
	if err := s.db.Get(&cur, `SELECT start_date, duration_months FROM projects WHERE id=$1`, id); err != nil {
		return nil, errors.New("проект не найден")
	}

	effStart := cur.StartDate
	if startDate != nil {
		effStart = *startDate
	}
	effDuration := cur.DurationMonths
	if req.DurationMonths != nil {
		effDuration = *req.DurationMonths
	}
	calc := effStart.AddDate(0, effDuration, 0)
	endDate = &calc

	// Те же правила нормализации, что и при создании. Поля тут
	// указательные: nil означает «не менять», трогаем только заданные.
	applyStr := func(p *string, f func(string) string) {
		if p != nil {
			v := f(*p)
			*p = v
		}
	}
	applyStr(req.Name, capitalizeFirst)
	applyStr(req.Customer, capitalizeFirst)
	applyStr(req.Location, capitalizeFirst)
	applyStr(req.Director, normalizeFullName)
	applyStr(req.Manager, normalizeFullName)
	applyStr(req.Administrator, normalizeFullName)
	applyStr(req.Economist, normalizeFullName)

	_, err := s.db.Exec(`
		UPDATE projects SET
		  name            = COALESCE($1, name),
		  customer        = COALESCE($2, customer),
		  executor_id     = COALESCE($3, executor_id),
		  location        = COALESCE($4, location),
		  start_date      = COALESCE($5, start_date),
		  duration_months = COALESCE($6, duration_months),
		  end_date        = $7,
		  director        = COALESCE($8, director),
		  manager         = COALESCE($9, manager),
		  administrator   = COALESCE($10, administrator),
		  economist       = COALESCE($11, economist),
		  updated_at      = NOW(),
		  updated_by      = $12
		WHERE id=$13`,
		req.Name, req.Customer, req.ExecutorID, req.Location,
		startDate, req.DurationMonths, endDate,
		req.Director, req.Manager, req.Administrator, req.Economist,
		updatedBy, id,
	)
	if err != nil {
		return nil, err
	}
	return s.Get(id)
}

func (s *Service) ChangeStatus(id int, req ChangeStatusRequest, updatedBy int) (*Project, error) {
	var cur struct {
		Status string `db:"status"`
	}
	if err := s.db.Get(&cur, `SELECT status FROM projects WHERE id=$1`, id); err != nil {
		return nil, errors.New("проект не найден")
	}
	if !CanTransition(cur.Status, req.Status) {
		return nil, fmt.Errorf("переход %s → %s недопустим", cur.Status, req.Status)
	}
	_, err := s.db.Exec(
		`UPDATE projects SET status=$1, updated_at=NOW(), updated_by=$2 WHERE id=$3`,
		req.Status, updatedBy, id,
	)
	if err != nil {
		return nil, err
	}
	return s.Get(id)
}

// ProjectStatus возвращает текущий статус проекта
func (s *Service) ProjectStatus(projectID int) (string, error) {
	var st string
	err := s.db.Get(&st, `SELECT status FROM projects WHERE id=$1`, projectID)
	return st, err
}
