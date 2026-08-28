package access

import (
	"errors"
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	"ibcon-budget/internal/auth"
	"ibcon-budget/internal/middleware"
)

/*
Проверка прав доступа на уровне API.

Матрица ролей лежит в auth/permissions.go — она чистая и покрыта тестом.
Здесь к ней добавляется то, что живёт в базе:

  - назначение на проект (user_project_permissions) — «доступ к проектам»,
    с галкой can_edit;
  - индивидуальные права сверх роли (user_permissions);
  - авторство проекта — для роли инициатора, которая работает только со
    своими карточками.

Порядок разрешения одинаков для всех обработчиков:

	Can(claims, право, проект)
	  1. ScopeAll     у роли               → да
	  2. ScopeOwn     и проект создан им   → да
	  3. ScopeOwn     и право на чтение и он назначен на проект → да
	  4. ScopeAssigned и назначен          → да, для правки нужен can_edit
	  5. индивидуальное право сверх роли   → да
	  6. иначе                             → нет

Фронт прячет кнопки по тем же кодам прав, но отказ выносит только
сервер: ТЗ 3.2 п.6 требует, чтобы запрет работал и при прямом
обращении к API.
*/

type Service struct {
	db *sqlx.DB
}

func NewService(db *sqlx.DB) *Service {
	return &Service{db: db}
}

// ─── Проверки ───────────────────────────────────────────────────────────

// Can — есть ли у пользователя право на действие.
//
// projectID = 0 означает действие вне конкретного проекта (создание
// проекта, справочники, пользователи, реестр целиком).
func (s *Service) Can(cl *auth.Claims, perm string, projectID int) bool {
	if cl == nil {
		return false
	}

	switch auth.RoleScope(cl.Role, perm) {
	case auth.ScopeAll:
		return true

	case auth.ScopeOwn:
		if projectID == 0 {
			// Право «в принципе есть, объём — свои проекты».
			// Для внепроектных действий этого достаточно.
			return true
		}
		if s.ownsProject(cl.UserID, projectID) {
			return true
		}
		// Назначение на чужой проект открывает инициатору просмотр,
		// но не правку: ТЗ разрешает ему менять только свои карточки.
		if auth.IsReadOnly(perm) && s.assigned(cl.UserID, projectID) {
			return true
		}

	case auth.ScopeAssigned:
		if projectID == 0 {
			return true
		}
		assigned, canEdit := s.assignment(cl.UserID, projectID)
		if assigned && (auth.IsReadOnly(perm) || canEdit) {
			return true
		}
	}

	return s.hasIndividual(cl.UserID, perm, projectID)
}

// CanAny — есть ли право хотя бы где-нибудь. Нужно для показа пунктов
// меню и кнопок: сам список потом всё равно фильтруется по проектам.
func (s *Service) CanAny(cl *auth.Claims, perm string) bool {
	if cl == nil {
		return false
	}
	if auth.RoleScope(cl.Role, perm) != auth.ScopeNone {
		return true
	}
	exact, star := grantCodes(perm)
	var n int
	err := s.db.Get(&n,
		`SELECT COUNT(*) FROM user_permissions
		  WHERE user_id=$1 AND permission IN ($2, $3)`,
		cl.UserID, exact, star)
	return err == nil && n > 0
}

// SeesAllProjects — видит ли пользователь весь реестр. Если нет, список
// проектов надо ограничить VisibleProjectIDs.
func (s *Service) SeesAllProjects(cl *auth.Claims, perm string) bool {
	if cl == nil {
		return false
	}
	if auth.RoleScope(cl.Role, perm) == auth.ScopeAll {
		return true
	}
	exact, star := grantCodes(perm)
	var n int
	err := s.db.Get(&n,
		`SELECT COUNT(*) FROM user_permissions
		  WHERE user_id=$1 AND project_id IS NULL AND permission IN ($2, $3)`,
		cl.UserID, exact, star)
	return err == nil && n > 0
}

// VisibleProjectIDs — проекты, в которых у пользователя есть указанное
// право. Вызывать только когда SeesAllProjects вернул false.
//
// Пустой список означает «не видно ничего»: обработчик обязан отдать
// пустой ответ, а не весь реестр.
func (s *Service) VisibleProjectIDs(cl *auth.Claims, perm string) []int {
	if cl == nil {
		return nil
	}
	set := map[int]bool{}

	switch auth.RoleScope(cl.Role, perm) {
	case auth.ScopeOwn:
		for _, id := range s.ownProjectIDs(cl.UserID) {
			set[id] = true
		}
		if auth.IsReadOnly(perm) {
			for _, id := range s.assignedProjectIDs(cl.UserID, false) {
				set[id] = true
			}
		}
	case auth.ScopeAssigned:
		// Для прав на изменение засчитываем только назначения с
		// галкой «может редактировать».
		for _, id := range s.assignedProjectIDs(cl.UserID, !auth.IsReadOnly(perm)) {
			set[id] = true
		}
	}

	exact, star := grantCodes(perm)
	var ids []int
	err := s.db.Select(&ids,
		`SELECT project_id FROM user_permissions
		  WHERE user_id=$1 AND project_id IS NOT NULL AND permission IN ($2, $3)`,
		cl.UserID, exact, star)
	if err == nil {
		for _, id := range ids {
			set[id] = true
		}
	}

	out := make([]int, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Ints(out)
	return out
}

// Permissions — что пользователь может в данном проекте.
//
// projectID = 0 — список «может хотя бы где-нибудь»: им фронт решает,
// показывать ли пункт меню и кнопку создания. Права на конкретный
// проект приходят вместе с карточкой проекта.
func (s *Service) Permissions(cl *auth.Claims, projectID int) []string {
	out := make([]string, 0, len(auth.AllPermissions))
	for _, p := range auth.AllPermissions {
		ok := s.Can(cl, p, projectID)
		if projectID == 0 {
			ok = s.CanAny(cl, p)
		}
		if ok {
			out = append(out, p)
		}
	}
	return out
}

// ─── Middleware для внепроектных маршрутов ──────────────────────────────

// Require — 403, если у пользователя нет права. Годится для маршрутов,
// не привязанных к проекту: справочники, пользователи, история.
// Проектные маршруты проверяют доступ внутри обработчика, потому что
// им нужен id проекта.
func (s *Service) Require(perm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		cl := middleware.GetClaims(c)
		if !s.Can(cl, perm, 0) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "недостаточно прав: " + auth.PermissionLabels[perm],
				"code":  "forbidden",
			})
			return
		}
		c.Next()
	}
}

// Deny — единый ответ на нехватку прав из обработчика.
func Deny(c *gin.Context, perm string) {
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
		"error": "недостаточно прав: " + auth.PermissionLabels[perm],
		"code":  "forbidden",
	})
}

// ─── Индивидуальные права: чтение и выдача ──────────────────────────────

// Grant — строка индивидуального права.
type Grant struct {
	ID          int     `db:"id"           json:"id"`
	UserID      int     `db:"user_id"      json:"user_id"`
	Permission  string  `db:"permission"   json:"permission"`
	ProjectID   *int    `db:"project_id"   json:"project_id"`
	ProjectName *string `db:"project_name" json:"project_name"`
	GrantedBy   int     `db:"granted_by"   json:"granted_by"`
	GrantedAt   string  `db:"granted_at"   json:"granted_at"`
}

func (s *Service) ListGrants(userID int) ([]Grant, error) {
	var rows []Grant
	err := s.db.Select(&rows,
		`SELECT up.id, up.user_id, up.permission, up.project_id,
		        p.name AS project_name, up.granted_by,
		        to_char(up.granted_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS granted_at
		   FROM user_permissions up
		   LEFT JOIN projects p ON p.id = up.project_id
		  WHERE up.user_id = $1
		  ORDER BY up.project_id NULLS FIRST, up.permission`,
		userID)
	return rows, err
}

// GrantsForAll — индивидуальные права всех пользователей одним запросом.
// Нужен экрану управления пользователями: иначе на каждую строку
// таблицы приходился бы отдельный поход в базу.
func (s *Service) GrantsForAll() (map[int][]Grant, error) {
	var rows []Grant
	err := s.db.Select(&rows,
		`SELECT up.id, up.user_id, up.permission, up.project_id,
		        p.name AS project_name, up.granted_by,
		        to_char(up.granted_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS granted_at
		   FROM user_permissions up
		   LEFT JOIN projects p ON p.id = up.project_id
		  ORDER BY up.user_id, up.project_id NULLS FIRST, up.permission`)
	if err != nil {
		return nil, err
	}
	out := map[int][]Grant{}
	for _, r := range rows {
		out[r.UserID] = append(out[r.UserID], r)
	}
	return out, nil
}

var (
	ErrUnknownPermission = errors.New("неизвестное право")
	ErrNotGrantable      = errors.New(
		"это право нельзя выдать индивидуально: управление пользователями " +
			"даётся только ролью главного экономиста")
	ErrProjectRequired = errors.New(
		"право действует только внутри проекта — укажите проект или выдайте его на все проекты")
	ErrProjectNotAllowed = errors.New("это право действует глобально, привязать его к проекту нельзя")
)

// AddGrant выдаёт индивидуальное право. projectID = nil — на все проекты.
func (s *Service) AddGrant(userID int, perm string, projectID *int, grantedBy int) error {
	if err := validateGrant(perm, projectID); err != nil {
		return err
	}
	_, err := s.db.Exec(
		`INSERT INTO user_permissions (user_id, permission, project_id, granted_by)
		 VALUES ($1,$2,$3,$4)
		 ON CONFLICT DO NOTHING`,
		userID, perm, projectID, grantedBy)
	return err
}

// RemoveGrant отзывает индивидуальное право.
func (s *Service) RemoveGrant(userID int, perm string, projectID *int) error {
	if projectID == nil {
		_, err := s.db.Exec(
			`DELETE FROM user_permissions
			  WHERE user_id=$1 AND permission=$2 AND project_id IS NULL`,
			userID, perm)
		return err
	}
	_, err := s.db.Exec(
		`DELETE FROM user_permissions
		  WHERE user_id=$1 AND permission=$2 AND project_id=$3`,
		userID, perm, *projectID)
	return err
}

// RemoveGrantsForProject убирает все индивидуальные права пользователя
// на проект. Вызывается при отзыве доступа к проекту: иначе доступ
// «отозван», а право на правку бюджета этого проекта осталось висеть.
func (s *Service) RemoveGrantsForProject(userID, projectID int) error {
	_, err := s.db.Exec(
		`DELETE FROM user_permissions WHERE user_id=$1 AND project_id=$2`,
		userID, projectID)
	return err
}

func validateGrant(perm string, projectID *int) error {
	if perm != auth.PermAll && auth.PermissionLabels[perm] == "" {
		return ErrUnknownPermission
	}
	if !auth.IsGrantable(perm) {
		return ErrNotGrantable
	}
	if projectID != nil && perm != auth.PermAll && !auth.IsProjectScoped(perm) {
		return ErrProjectNotAllowed
	}
	return nil
}

// ─── Запросы к базе ─────────────────────────────────────────────────────

func (s *Service) ownsProject(userID, projectID int) bool {
	var owner int
	err := s.db.Get(&owner, `SELECT created_by FROM projects WHERE id=$1`, projectID)
	if err != nil {
		return false
	}
	return owner == userID
}

func (s *Service) ownProjectIDs(userID int) []int {
	var ids []int
	_ = s.db.Select(&ids, `SELECT id FROM projects WHERE created_by=$1`, userID)
	return ids
}

func (s *Service) assignment(userID, projectID int) (assigned, canEdit bool) {
	err := s.db.Get(&canEdit,
		`SELECT can_edit FROM user_project_permissions WHERE user_id=$1 AND project_id=$2`,
		userID, projectID)
	if err != nil {
		return false, false
	}
	return true, canEdit
}

func (s *Service) assigned(userID, projectID int) bool {
	a, _ := s.assignment(userID, projectID)
	return a
}

func (s *Service) assignedProjectIDs(userID int, editOnly bool) []int {
	q := `SELECT project_id FROM user_project_permissions WHERE user_id=$1`
	if editOnly {
		q += ` AND can_edit = TRUE`
	}
	var ids []int
	_ = s.db.Select(&ids, q, userID)
	return ids
}

func (s *Service) hasIndividual(userID int, perm string, projectID int) bool {
	exact, star := grantCodes(perm)
	var n int
	if projectID == 0 {
		// Действие вне проекта. Право, выданное НА ПРОЕКТ, здесь не
		// считается: иначе «все права на проект №14» открывали бы
		// справочники и управление пользователями всей системы.
		err := s.db.Get(&n,
			`SELECT COUNT(*) FROM user_permissions
			  WHERE user_id=$1 AND permission IN ($2, $3) AND project_id IS NULL`,
			userID, exact, star)
		return err == nil && n > 0
	}
	err := s.db.Get(&n,
		`SELECT COUNT(*) FROM user_permissions
		  WHERE user_id=$1 AND permission IN ($2, $3)
		    AND (project_id IS NULL OR project_id = $4)`,
		userID, exact, star, projectID)
	return err == nil && n > 0
}

// grantCodes — какие записи user_permissions дают запрошенное право.
//
// Обычно это само право и «звёздочка». Но «звёздочка» покрывает только
// то, что главный экономист вправе выдать индивидуально: управление
// пользователями через неё пройти не должно, иначе тумблер «все права»
// обходил бы запрет auth.IsGrantable. Для таких прав вторым кодом
// возвращается то же самое право — совпасть может только точная запись,
// а её выдача отклоняется при создании.
func grantCodes(perm string) (exact, star string) {
	if auth.IsGrantable(perm) {
		return perm, auth.PermAll
	}
	return perm, perm
}
