package users

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"ibcon-budget/internal/auth"
)

// maxAvatarLen ограничивает размер аватара. Эмодзи занимает единицы байт,
// картинка приходит как data:-URL — ~1.4 МБ base64 соответствует примерно
// мегабайту исходного файла. Больше в поле профиля не нужно.
const maxAvatarLen = 1_400_000

// maxNotesLen — рабочие заметки. Это блокнот, а не хранилище документов.
const maxNotesLen = 20_000

// Profile — данные личного кабинета. Пароль и служебные поля наружу
// не отдаются.
type Profile struct {
	ID       int    `db:"id"        json:"id"`
	Email    string `db:"email"     json:"email"`
	FullName string `db:"full_name" json:"full_name"`
	Role     string `db:"role"      json:"role"`
	Avatar   string `db:"avatar"    json:"avatar"`
	Notes    string `db:"notes"     json:"notes"`

	// Permissions — что пользователь может хотя бы где-нибудь.
	// По этому списку фронт решает, показывать ли пункты меню и кнопки
	// создания. Заполняется обработчиком, в таблице такой колонки нет.
	Permissions []string `db:"-" json:"permissions"`
}

type UpdateProfileRequest struct {
	// Оба поля указательные: nil означает «не менять».
	// ФИО через профиль не меняется — им управляет главный экономист.
	Avatar *string `json:"avatar"`
	Notes  *string `json:"notes"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password"     binding:"required"`
}

// GetProfile возвращает данные личного кабинета пользователя.
func (s *Service) GetProfile(userID int) (*Profile, error) {
	var p Profile
	err := s.db.Get(&p,
		`SELECT id, email, full_name, role, avatar, notes FROM users WHERE id=$1`,
		userID)
	if err != nil {
		return nil, errors.New("пользователь не найден")
	}
	return &p, nil
}

// UpdateProfile меняет аватар и заметки. ФИО и роль здесь не трогаются
// сознательно — они в ведении главного экономиста.
func (s *Service) UpdateProfile(userID int, req UpdateProfileRequest) (*Profile, error) {
	if req.Avatar != nil && len(*req.Avatar) > maxAvatarLen {
		return nil, fmt.Errorf("изображение слишком большое (%d байт, максимум %d)",
			len(*req.Avatar), maxAvatarLen)
	}
	if req.Notes != nil && utf8.RuneCountInString(*req.Notes) > maxNotesLen {
		return nil, fmt.Errorf("заметки слишком длинные (максимум %d символов)", maxNotesLen)
	}

	_, err := s.db.Exec(`
		UPDATE users
		SET avatar     = COALESCE($1, avatar),
		    notes      = COALESCE($2, notes),
		    updated_at = NOW()
		WHERE id=$3`,
		req.Avatar, req.Notes, userID)
	if err != nil {
		return nil, err
	}
	return s.GetProfile(userID)
}

// ChangeOwnPassword меняет пароль пользователю по его собственной просьбе.
//
// В отличие от SetPassword (её вызывает главный экономист), здесь
// обязательна проверка текущего пароля: иначе перехваченная сессия
// позволила бы сменить пароль и закрепиться в системе.
func (s *Service) ChangeOwnPassword(userID int, req ChangePasswordRequest) error {
	var hash string
	if err := s.db.Get(&hash, `SELECT password_hash FROM users WHERE id=$1 AND active=TRUE`, userID); err != nil {
		return errors.New("пользователь не найден")
	}

	if !auth.CheckPassword(hash, req.CurrentPassword) {
		return errors.New("текущий пароль указан неверно")
	}

	if strings.TrimSpace(req.NewPassword) == req.CurrentPassword {
		return errors.New("новый пароль совпадает с текущим")
	}

	// Требования ТЗ 3.8: 10+ символов, минимум 1 заглавная и 1 цифра.
	if err := auth.ValidatePassword(req.NewPassword); err != nil {
		return err
	}

	newHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		return err
	}

	// Сбрасываем счётчик неудачных попыток и блокировку: пароль сменён
	// осознанно, держать учётку запертой старой блокировкой незачем.
	_, err = s.db.Exec(`
		UPDATE users
		SET password_hash=$1, failed_attempts=0, locked_until=NULL, updated_at=NOW()
		WHERE id=$2`, newHash, userID)
	return err
}
