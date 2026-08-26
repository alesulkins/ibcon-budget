package budgets

import (
	"database/sql"
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

// getOrCreateBudget возвращает или создаёт запись в таблице budgets для проекта
func (s *Service) getOrCreateBudget(tx *sqlx.Tx, projectID, userID int) (int, error) {
	var budgetID int
	err := tx.Get(&budgetID, `SELECT id FROM budgets WHERE project_id=$1`, projectID)
	if err == nil {
		return budgetID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	// Создаём бюджет
	err = tx.QueryRowx(
		`INSERT INTO budgets (project_id, created_by) VALUES ($1,$2) RETURNING id`,
		projectID, userID,
	).Scan(&budgetID)
	return budgetID, err
}

// CreateVersion создаёт новую версию бюджета (первую или следующую)
func (s *Service) CreateVersion(projectID, userID int, req CreateVersionRequest) (*BudgetVersion, error) {
	tx, err := s.db.Beginx()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	budgetID, err := s.getOrCreateBudget(tx, projectID, userID)
	if err != nil {
		return nil, fmt.Errorf("init budget: %w", err)
	}

	// Проверяем: нет ли уже активной (не архивной) версии
	var activeCount int
	_ = tx.Get(&activeCount, `
		SELECT COUNT(*) FROM budget_versions
		WHERE budget_id=$1 AND status NOT IN ('archive')`, budgetID,
	)
	if activeCount > 0 {
		return nil, errors.New("уже существует активная версия бюджета — создайте новую через смену статуса или архивируйте текущую")
	}

	comment := req.Comment
	var copiedFrom *int

	if req.CopyFromID != nil {
		// Проверяем что исходная версия принадлежит этому бюджету
		var srcBudgetID int
		err = tx.Get(&srcBudgetID, `SELECT budget_id FROM budget_versions WHERE id=$1`, *req.CopyFromID)
		if err != nil || srcBudgetID != budgetID {
			return nil, errors.New("исходная версия не найдена в этом бюджете")
		}
		copiedFrom = req.CopyFromID
		prefix := fmt.Sprintf("Создано на основании бюджета %d", *req.CopyFromID)
		if comment != "" {
			comment = prefix + ". " + comment
		} else {
			comment = prefix
		}
	}

	// Порядковый номер версии внутри проекта. Считаем по всем версиям
	// проекта, а не одного budget-а: у проекта может быть несколько
	// записей budgets, а нумерация в ТЗ сквозная (1.1, 1.2, …).
	var nextNo int
	err = tx.Get(&nextNo, `
		SELECT COALESCE(MAX(bv.version_no), 0) + 1
		FROM budget_versions bv
		JOIN budgets b ON b.id = bv.budget_id
		WHERE b.project_id = $1`, projectID)
	if err != nil {
		return nil, err
	}

	var vID int
	err = tx.QueryRowx(`
		INSERT INTO budget_versions (budget_id, version_no, status, comment, copied_from, created_by)
		VALUES ($1,$2,'draft',$3,$4,$5)
		RETURNING id`,
		budgetID, nextNo, nullStr(comment), copiedFrom, userID,
	).Scan(&vID)
	if err != nil {
		return nil, err
	}

	// Если копируем — дублируем input-данные
	if req.CopyFromID != nil {
		_, err = tx.Exec(`
			INSERT INTO budget_inputs (budget_version_id, input_type, data, updated_by)
			SELECT $1, input_type, data, $2
			FROM budget_inputs WHERE budget_version_id=$3`,
			vID, userID, *req.CopyFromID,
		)
		if err != nil {
			return nil, err
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetVersion(vID)
}

// ListVersions возвращает все версии бюджета проекта в нужном порядке
func (s *Service) ListVersions(projectID int) ([]BudgetVersion, error) {
	var versions []BudgetVersion
	err := s.db.Select(&versions, `
		SELECT bv.id, bv.budget_id, b.project_id, bv.version_no, bv.version_label, bv.status,
		       bv.comment, bv.cost_no_vat, bv.profitability, bv.cost_override,
		       bv.created_at, bv.created_by,
		       u.full_name AS created_by_name,
		       bv.updated_at, bv.approved_at, bv.copied_from
		FROM budget_versions bv
		JOIN budgets b ON b.id = bv.budget_id
		JOIN users u ON u.id = bv.created_by
		WHERE b.project_id=$1
		ORDER BY
			CASE bv.status
				WHEN 'approved'     THEN 1
				WHEN 'under_review' THEN 2
				WHEN 'draft'        THEN 3
				WHEN 'archive'      THEN 4
				ELSE 5
			END,
			bv.id DESC`,
		projectID,
	)
	return versions, err
}

func (s *Service) GetVersion(id int) (*BudgetVersion, error) {
	var v BudgetVersion
	err := s.db.QueryRowx(`
		SELECT bv.id, bv.budget_id, b.project_id, bv.version_no, bv.version_label, bv.status,
		       bv.comment, bv.cost_no_vat, bv.profitability, bv.cost_override,
		       bv.created_at, bv.created_by,
		       u.full_name AS created_by_name,
		       bv.updated_at, bv.approved_at, bv.copied_from
		FROM budget_versions bv
		JOIN budgets b ON b.id = bv.budget_id
		JOIN users u ON u.id = bv.created_by
		WHERE bv.id=$1`, id,
	).StructScan(&v)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// ChangeStatus меняет статус версии с бизнес-логикой
func (s *Service) ChangeStatus(versionID, userID int, req ChangeStatusRequest) (*BudgetVersion, error) {
	v, err := s.GetVersion(versionID)
	if err != nil {
		return nil, errors.New("версия бюджета не найдена")
	}

	if !CanTransition(v.Status, req.Status) {
		return nil, fmt.Errorf("переход %s → %s недопустим", v.Status, req.Status)
	}

	tx, err := s.db.Beginx()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	comment := req.Comment
	var approvedAt *time.Time

	if req.Status == StatusApproved {
		now := time.Now()
		approvedAt = &now
		dateStr := now.Format("02.01.2006")
		prefix := "Согласовано от " + dateStr
		if !strings.HasPrefix(comment, prefix) {
			comment = prefix + ". " + comment
		}

		// Архивируем текущую согласованную версию (если есть)
		archiveSeq := 1
		var maxSeq *int
		_ = tx.Get(&maxSeq, `
			SELECT MAX(CAST(REPLACE(version_label,'в.','') AS INTEGER))
			FROM budget_versions
			WHERE budget_id=$1 AND status='archive' AND version_label LIKE 'в.%'`,
			v.BudgetID,
		)
		if maxSeq != nil {
			archiveSeq = *maxSeq + 1
		}

		label := fmt.Sprintf("в.%d", archiveSeq)
		_, err = tx.Exec(`
			UPDATE budget_versions
			SET status='archive', version_label=$1, updated_at=NOW(), updated_by=$2
			WHERE budget_id=$3 AND status='approved'`,
			label, userID, v.BudgetID,
		)
		if err != nil {
			return nil, err
		}
	}

	_, err = tx.Exec(`
		UPDATE budget_versions
		SET status=$1, comment=$2, approved_at=$3, updated_at=NOW(), updated_by=$4
		WHERE id=$5`,
		req.Status, nullStr(comment), approvedAt, userID, versionID,
	)
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetVersion(versionID)
}

// NewVersion создаёт следующую версию поверх существующей (архивирует текущую и создаёт новую)
func (s *Service) NewVersion(projectID, userID int, req CreateVersionRequest) (*BudgetVersion, error) {
	tx, err := s.db.Beginx()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var budgetID int
	if err = tx.Get(&budgetID, `SELECT id FROM budgets WHERE project_id=$1`, projectID); err != nil {
		return nil, errors.New("бюджет не найден")
	}

	// Определяем следующий номер архивной версии
	archiveSeq := 1
	var maxSeq *int
	_ = tx.Get(&maxSeq, `
		SELECT MAX(CAST(REPLACE(version_label,'в.','') AS INTEGER))
		FROM budget_versions
		WHERE budget_id=$1 AND status='archive' AND version_label LIKE 'в.%'`,
		budgetID,
	)
	if maxSeq != nil {
		archiveSeq = *maxSeq + 1
	}

	// Архивируем активную (не архивную) версию
	label := fmt.Sprintf("в.%d", archiveSeq)
	_, err = tx.Exec(`
		UPDATE budget_versions
		SET status='archive', version_label=$1, updated_at=NOW(), updated_by=$2
		WHERE budget_id=$3 AND status NOT IN ('archive')`,
		label, userID, budgetID,
	)
	if err != nil {
		return nil, err
	}

	comment := req.Comment
	var copiedFrom *int

	if req.CopyFromID != nil {
		copiedFrom = req.CopyFromID
		prefix := fmt.Sprintf("Создано на основании бюджета %d", *req.CopyFromID)
		if comment != "" {
			comment = prefix + ". " + comment
		} else {
			comment = prefix
		}
	}

	// Порядковый номер версии внутри проекта. Считаем по всем версиям
	// проекта, а не одного budget-а: у проекта может быть несколько
	// записей budgets, а нумерация в ТЗ сквозная (1.1, 1.2, …).
	var nextNo int
	err = tx.Get(&nextNo, `
		SELECT COALESCE(MAX(bv.version_no), 0) + 1
		FROM budget_versions bv
		JOIN budgets b ON b.id = bv.budget_id
		WHERE b.project_id = $1`, projectID)
	if err != nil {
		return nil, err
	}

	var vID int
	err = tx.QueryRowx(`
		INSERT INTO budget_versions (budget_id, version_no, status, comment, copied_from, created_by)
		VALUES ($1,$2,'draft',$3,$4,$5)
		RETURNING id`,
		budgetID, nextNo, nullStr(comment), copiedFrom, userID,
	).Scan(&vID)
	if err != nil {
		return nil, err
	}

	if req.CopyFromID != nil {
		_, err = tx.Exec(`
			INSERT INTO budget_inputs (budget_version_id, input_type, data, updated_by)
			SELECT $1, input_type, data, $2
			FROM budget_inputs WHERE budget_version_id=$3`,
			vID, userID, *req.CopyFromID,
		)
		if err != nil {
			return nil, err
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetVersion(vID)
}

// SaveInput сохраняет исходные данные по типу (upsert)
func (s *Service) SaveInput(versionID, userID int, inputType string, data []byte) error {
	v, err := s.GetVersion(versionID)
	if err != nil {
		return errors.New("версия не найдена")
	}
	if v.Status == StatusApproved || v.Status == StatusArchive {
		return errors.New("нельзя редактировать согласованную или архивную версию")
	}
	_, err = s.db.Exec(`
		INSERT INTO budget_inputs (budget_version_id, input_type, data, updated_by)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (budget_version_id, input_type)
		DO UPDATE SET data=$3, updated_at=NOW(), updated_by=$4`,
		versionID, inputType, data, userID,
	)
	return err
}

// GetInput возвращает сохранённые данные по типу
func (s *Service) GetInput(versionID int, inputType string) ([]byte, error) {
	var data []byte
	err := s.db.Get(&data, `
		SELECT data FROM budget_inputs WHERE budget_version_id=$1 AND input_type=$2`,
		versionID, inputType,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return []byte("{}"), nil
	}
	return data, err
}

// GetAllInputs возвращает все input-данные версии (map тип → данные)
func (s *Service) GetAllInputs(versionID int) (map[string][]byte, error) {
	rows, err := s.db.Queryx(`
		SELECT input_type, data FROM budget_inputs WHERE budget_version_id=$1`, versionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string][]byte)
	for rows.Next() {
		var t string
		var d []byte
		if err = rows.Scan(&t, &d); err != nil {
			return nil, err
		}
		result[t] = d
	}
	return result, rows.Err()
}

// UpdateCostOverride обновляет ручную корректировку стоимости
func (s *Service) UpdateCostOverride(versionID int, override *float64, userID int) (*BudgetVersion, error) {
	v, err := s.GetVersion(versionID)
	if err != nil {
		return nil, errors.New("версия не найдена")
	}
	if v.Status == StatusApproved || v.Status == StatusArchive {
		return nil, errors.New("нельзя редактировать согласованную или архивную версию")
	}
	_, err = s.db.Exec(`
		UPDATE budget_versions SET cost_override=$1, updated_at=NOW(), updated_by=$2 WHERE id=$3`,
		override, userID, versionID,
	)
	if err != nil {
		return nil, err
	}
	return s.GetVersion(versionID)
}

// UpdateCachedResults обновляет кэшированные результаты расчёта
func (s *Service) UpdateCachedResults(versionID int, costNoVat, profitability *float64) error {
	_, err := s.db.Exec(`
		UPDATE budget_versions SET cost_no_vat=$1, profitability=$2, updated_at=NOW()
		WHERE id=$3`,
		costNoVat, profitability, versionID,
	)
	return err
}

// VersionOwner возвращает created_by версии
func (s *Service) VersionOwner(versionID int) (int, error) {
	var uid int
	err := s.db.Get(&uid, `SELECT created_by FROM budget_versions WHERE id=$1`, versionID)
	return uid, err
}

func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
