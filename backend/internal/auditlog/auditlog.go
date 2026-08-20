package auditlog

import (
	"log/slog"

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
}

type LogRow struct {
	ID         int64   `db:"id"          json:"id"`
	OccurredAt string  `db:"occurred_at" json:"occurred_at"`
	UserID     *int    `db:"user_id"     json:"user_id"`
	UserRole   string  `db:"user_role"   json:"user_role"`
	Action     string  `db:"action"      json:"action"`
	ObjectType string  `db:"object_type" json:"object_type"`
	ObjectID   *int    `db:"object_id"   json:"object_id"`
	Comment    string  `db:"comment"     json:"comment"`
}

func (s *Service) List(p ListParams) ([]LogRow, int, error) {
	if p.Limit <= 0 {
		p.Limit = 50
	}
	var total int
	if err := s.db.Get(&total, `SELECT COUNT(*) FROM audit_log`); err != nil {
		return nil, 0, err
	}
	var rows []LogRow
	err := s.db.Select(&rows,
		`SELECT id, to_char(occurred_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS occurred_at,
		        user_id, user_role, action, object_type, object_id, COALESCE(comment,'') AS comment
		 FROM audit_log ORDER BY occurred_at DESC LIMIT $1 OFFSET $2`,
		p.Limit, p.Offset,
	)
	return rows, total, err
}
