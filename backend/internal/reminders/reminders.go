// Package reminders — напоминания личного кабинета.
//
// Напоминание это заметка со сроком: когда срок наступил, платформа
// показывает её всплывающим уведомлением на любой странице. Рассылки
// писем нет — решение владельца 2026-08-30.
package reminders

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jmoiron/sqlx"
)

// maxTextLen — напоминание это строка-другая, а не документ.
const maxTextLen = 2_000

// Reminder — одно напоминание.
type Reminder struct {
	ID       int       `db:"id"        json:"id"`
	UserID   int       `db:"user_id"   json:"user_id"`
	Text     string    `db:"text"      json:"text"`
	RemindAt time.Time `db:"remind_at" json:"remind_at"`

	// ShownAt — когда напоминание всплыло на экране. По нему оно
	// перестаёт попадать в выборку наступивших.
	ShownAt *time.Time `db:"shown_at" json:"shown_at,omitempty"`

	// Done — закрыто пользователем. Записи не удаляем: список сделанного
	// — тоже история работы.
	Done      bool      `db:"done"       json:"done"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type Service struct {
	db *sqlx.DB
}

func NewService(db *sqlx.DB) *Service {
	return &Service{db: db}
}

const cols = `id, user_id, text, remind_at, shown_at, done, created_at`

// List — все напоминания пользователя, ближайшие по сроку сверху.
// Выполненные уходят вниз: работают с активными.
func (s *Service) List(userID int) ([]Reminder, error) {
	rows := []Reminder{}
	return rows, s.db.Select(&rows,
		`SELECT `+cols+` FROM reminders WHERE user_id=$1 ORDER BY done, remind_at`, userID)
}

func validate(text string, remindAt time.Time) error {
	if strings.TrimSpace(text) == "" {
		return errors.New("текст напоминания не может быть пустым")
	}
	if utf8.RuneCountInString(text) > maxTextLen {
		return errors.New("текст напоминания слишком длинный")
	}
	if remindAt.IsZero() {
		return errors.New("укажите дату и время напоминания")
	}
	return nil
}

func (s *Service) Create(userID int, text string, remindAt time.Time) (*Reminder, error) {
	if err := validate(text, remindAt); err != nil {
		return nil, err
	}
	var r Reminder
	err := s.db.QueryRowx(
		`INSERT INTO reminders (user_id, text, remind_at) VALUES ($1,$2,$3)
		 RETURNING `+cols,
		userID, strings.TrimSpace(text), remindAt,
	).StructScan(&r)
	return &r, err
}

// Update меняет текст, срок или отметку «выполнено». nil — не менять.
//
// Перенос срока вперёд снимает отметку о показе: напоминание, которое
// передвинули на будущее, должно всплыть заново — иначе перенос молча
// превращал бы его в невидимое.
func (s *Service) Update(userID, id int, text *string, remindAt *time.Time, done *bool) (*Reminder, error) {
	if text != nil || remindAt != nil {
		t := ""
		if text != nil {
			t = *text
		}
		var at time.Time
		if remindAt != nil {
			at = *remindAt
		}
		// Проверяем только то, что пришло: пустое поле означает «не менять».
		if text != nil {
			if err := validate(t, time.Now()); err != nil {
				return nil, err
			}
		}
		if remindAt != nil && at.IsZero() {
			return nil, errors.New("укажите дату и время напоминания")
		}
	}

	var r Reminder
	err := s.db.QueryRowx(`
		UPDATE reminders SET
		  text      = COALESCE($1, text),
		  remind_at = COALESCE($2, remind_at),
		  done      = COALESCE($3, done),
		  shown_at  = CASE WHEN $2 IS NOT NULL AND $2 > NOW() THEN NULL ELSE shown_at END,
		  updated_at = NOW()
		WHERE id=$4 AND user_id=$5
		RETURNING `+cols,
		text, remindAt, done, id, userID,
	).StructScan(&r)
	if err != nil {
		return nil, errors.New("напоминание не найдено")
	}
	return &r, nil
}

// Delete — напоминание удаляется насовсем, в отличие от записей
// справочников: это личная заметка пользователя, а не общие данные.
func (s *Service) Delete(userID, id int) error {
	res, err := s.db.Exec(`DELETE FROM reminders WHERE id=$1 AND user_id=$2`, id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errors.New("напоминание не найдено")
	}
	return nil
}

// Due — напоминания, которым пора всплыть: срок наступил, не выполнены и
// ещё не показывались.
func (s *Service) Due(userID int) ([]Reminder, error) {
	rows := []Reminder{}
	return rows, s.db.Select(&rows, `
		SELECT `+cols+` FROM reminders
		WHERE user_id=$1 AND done=FALSE AND shown_at IS NULL AND remind_at <= NOW()
		ORDER BY remind_at`, userID)
}

// MarkShown отмечает напоминания показанными, чтобы они не всплывали на
// каждой странице заново.
func (s *Service) MarkShown(userID int, ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	q, args, err := sqlx.In(
		`UPDATE reminders SET shown_at=NOW(), updated_at=NOW()
		 WHERE user_id=? AND id IN (?)`, userID, ids)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(s.db.Rebind(q), args...)
	return err
}
