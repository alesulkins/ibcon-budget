package auditlog

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/jmoiron/sqlx"
)

type Entry struct {
	UserID     *int
	UserRole   string
	Action     string
	ObjectType string
	ObjectID   *int
	Comment    string
}

type Service struct {
	db *sqlx.DB
}

func NewService(db *sqlx.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Log(e Entry) {
	_, err := s.db.Exec(
		`INSERT INTO audit_log (user_id, user_role, action, object_type, object_id, comment)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		e.UserID, e.UserRole, e.Action, e.ObjectType, e.ObjectID, e.Comment,
	)
	if err != nil {
		slog.Error("audit log write failed", "err", err)
	}
}

type ListParams struct {
	Limit  int
	Offset int

	// ProjectIDs — ограничение выборки проектами, которые пользователю видны.
	// nil означает «все» и ставится только тем, кто видит весь журнал.
	ProjectIDs []int
}

type LogRow struct {
	ID         int64  `db:"id"          json:"id"`
	OccurredAt string `db:"occurred_at" json:"occurred_at"`
	UserID     *int   `db:"user_id"     json:"user_id"`
	UserName   string `db:"user_name"   json:"user_name"`
	UserRole   string `db:"user_role"   json:"user_role"`
	Action     string `db:"action"      json:"action"`
	ObjectType string `db:"object_type" json:"object_type"`
	ObjectID   *int   `db:"object_id"   json:"object_id"`
	Comment    string `db:"comment"     json:"comment"`

	// ProjectID / ProjectName — проект, к которому относится запись.
	// Заполняются и для действий над версией бюджета: по цепочке
	// budget_versions → budgets → projects.
	ProjectID   *int   `db:"project_id"   json:"project_id"`
	ProjectName string `db:"project_name" json:"project_name"`
}

func (s *Service) List(p ListParams) ([]LogRow, int, error) {
	if p.Limit <= 0 {
		p.Limit = 50
	}

	// Проект определяем двумя путями: напрямую (object_type='project') и через
	// версию бюджета (object_type='budget_version').
	from := `
		 FROM audit_log a
		 LEFT JOIN users u ON u.id = a.user_id
		 -- проект напрямую
		 LEFT JOIN projects pd
		        ON a.object_type = 'project' AND pd.id = a.object_id
		 -- проект через версию бюджета
		 LEFT JOIN budget_versions bv
		        ON a.object_type = 'budget_version' AND bv.id = a.object_id
		 LEFT JOIN budgets b  ON b.id = bv.budget_id
		 LEFT JOIN projects pv ON pv.id = b.project_id`

	args := []any{}
	where := ""
	if p.ProjectIDs != nil {
		ph := make([]string, len(p.ProjectIDs))
		for i, id := range p.ProjectIDs {
			ph[i] = fmt.Sprintf("$%d", i+1)
			args = append(args, id)
		}
		where = " WHERE COALESCE(pv.id, pd.id) IN (" + strings.Join(ph, ",") + ")"
	}

	var total int
	if err := s.db.Get(&total, "SELECT COUNT(*)"+from+where, args...); err != nil {
		return nil, 0, err
	}

	var rows []LogRow
	err := s.db.Select(&rows,
		`SELECT a.id,
		        to_char(a.occurred_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS occurred_at,
		        a.user_id,
		        COALESCE(u.full_name, '')      AS user_name,
		        a.user_role,
		        a.action,
		        a.object_type,
		        a.object_id,
		        COALESCE(a.comment,'')         AS comment,
		        COALESCE(pv.id, pd.id)         AS project_id,
		        COALESCE(pv.name, pd.name, '') AS project_name`+
			from+where+
			fmt.Sprintf(` ORDER BY a.occurred_at DESC LIMIT $%d OFFSET $%d`,
				len(args)+1, len(args)+2),
		append(args, p.Limit, p.Offset)...,
	)
	return rows, total, err
}
