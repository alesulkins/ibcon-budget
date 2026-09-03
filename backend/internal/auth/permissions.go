package auth

import "slices"

/*
Ролевая модель доступа — базовые права ролей (ТЗ, таблица 1).

Здесь только МАТРИЦА: какая роль что может и в каком объёме проектов.
Проверка «а этот конкретный пользователь может ли вот в этом проекте»
живёт в пакете internal/access — она ходит в базу за назначенными
проектами и индивидуальными правами.

Матрица вынесена данными, а не разбросана по хендлерам условиями
`if role == ...`: иначе право проверялось бы в каждом обработчике
по-своему, и добавление роли требовало бы обхода всего кода.
*/

// Роли системы
const (
	RoleGE         = "GE"         // Главный экономист
	RoleEP         = "EP"         // Экономист проекта
	RoleIP         = "IP"         // Инициатор проекта
	RoleRP         = "RP"         // Руководитель проекта
	RoleAP         = "AP"         // Администратор проекта
	RoleManagement = "MANAGEMENT" // Руководство

	// RoleNone — учётка создана, роль ещё не назначена.
	RoleNone = ""
)

var AllRoles = []string{RoleGE, RoleEP, RoleIP, RoleRP, RoleAP, RoleManagement}

// RoleLabels — подписи ролей, те же, что в интерфейсе.
var RoleLabels = map[string]string{
	RoleGE:         "Главный экономист",
	RoleEP:         "Экономист проекта",
	RoleIP:         "Инициатор проекта",
	RoleRP:         "Руководитель проекта",
	RoleAP:         "Администратор проекта",
	RoleManagement: "Руководство",
	RoleNone:       "Без роли",
}

// IsKnownRole — принимаем известные роли и пустую («роль не назначена»).
func IsKnownRole(role string) bool {
	return role == RoleNone || slices.Contains(AllRoles, role)
}

// ─── Действия ───────────────────────────────────────────────────────────

// Коды действий. Проверяются на каждом API-запросе; фронт использует те
// же коды, чтобы прятать кнопки — но прячет он их для удобства, а
// отказывает всегда бэкенд.
const (
	PermProjectView   = "project.view"   // видеть проект и реестр
	PermProjectCreate = "project.create" // создать карточку проекта
	PermProjectEdit   = "project.edit"   // изменить карточку проекта
	PermProjectStatus = "project.status" // сменить статус проекта

	PermBudgetView    = "budget.view"    // видеть версии и расчёт
	PermBudgetCreate  = "budget.create"  // создать ПЕРВУЮ версию бюджета
	PermBudgetVersion = "budget.version" // создать новую версию
	PermBudgetEdit    = "budget.edit"    // менять данные версии
	PermBudgetStatus  = "budget.status"  // менять статус версии
	PermBudgetApprove = "budget.approve" // ставить статус «Согласован»
	PermBudgetExport  = "budget.export"  // выгрузка xlsx

	PermReferencesEdit = "references.edit" // правка справочников
	PermAuditView      = "audit.view"      // история изменений
	PermUsersManage    = "users.manage"    // пользователи, роли, доступы
)

// AllPermissions — порядок важен: в таком виде права показываются в
// панели выдачи прав.
var AllPermissions = []string{
	PermProjectView, PermProjectCreate, PermProjectEdit, PermProjectStatus,
	PermBudgetView, PermBudgetCreate, PermBudgetVersion, PermBudgetEdit,
	PermBudgetStatus, PermBudgetApprove, PermBudgetExport,
	PermReferencesEdit, PermAuditView, PermUsersManage,
}

var PermissionLabels = map[string]string{
	PermProjectView:    "Просмотр проекта",
	PermProjectCreate:  "Создание карточки проекта",
	PermProjectEdit:    "Изменение карточки проекта",
	PermProjectStatus:  "Смена статуса проекта",
	PermBudgetView:     "Просмотр бюджета",
	PermBudgetCreate:   "Создание первой версии бюджета",
	PermBudgetVersion:  "Создание новой версии бюджета",
	PermBudgetEdit:     "Изменение версии бюджета",
	PermBudgetStatus:   "Смена статуса бюджета",
	PermBudgetApprove:  "Согласование бюджета",
	PermBudgetExport:   "Выгрузка бюджета в xlsx",
	PermReferencesEdit: "Изменение справочников",
	PermAuditView:      "Просмотр истории изменений",
	PermUsersManage:    "Управление пользователями",
}

// PermAll — псевдоправо «все действия», значение в user_permissions при
// выдаче всех прав на один проект (тумблер в панели выдачи прав).
const PermAll = "*"

// projectScoped — права, у которых есть привязка к конкретному проекту.
// Остальные (создание проекта, справочники, пользователи) действуют
// глобально, и выдавать их «на один проект» бессмысленно.
var projectScoped = map[string]bool{
	PermProjectView:   true,
	PermProjectEdit:   true,
	PermProjectStatus: true,
	PermBudgetView:    true,
	PermBudgetCreate:  true,
	PermBudgetVersion: true,
	PermBudgetEdit:    true,
	PermBudgetStatus:  true,
	PermBudgetApprove: true,
	PermBudgetExport:  true,
	PermAuditView:     true,
}

func IsProjectScoped(perm string) bool { return projectScoped[perm] }

// readOnly — права, которые только показывают данные. Назначение на
// проект (user_project_permissions) с галкой «только просмотр» открывает
// их, но не открывает права из списка на изменение.
var readOnly = map[string]bool{
	PermProjectView:  true,
	PermBudgetView:   true,
	PermBudgetExport: true,
	PermAuditView:    true,
}

func IsReadOnly(perm string) bool { return readOnly[perm] }

// IsGrantable — можно ли выдать право индивидуально сверх роли. ТЗ:
// «дополнительные права не могут превышать полномочия главного экономиста».
func IsGrantable(perm string) bool {
	if perm == PermAll {
		return true
	}
	if perm == PermUsersManage {
		return false
	}
	return slices.Contains(AllPermissions, perm)
}

// ─── Объём права ────────────────────────────────────────────────────────

// Scope — на какие проекты распространяется базовое право роли.
type Scope int

const (
	ScopeNone     Scope = iota // права нет
	ScopeOwn                   // только проекты, которые пользователь создал сам
	ScopeAssigned              // только назначенные проекты
	ScopeAll                   // все проекты
)

/*
roleMatrix — таблица 1 ТЗ в коде.

Пустая клетка (роль не упомянута) означает ScopeNone.

Расхождения с таблицей 1, сделанные осознанно:

  - «Смена статуса ПРОЕКТА» в таблице 1 отдельной строкой не значится.
    Считаем её частью «Внести изменения в карточке проекта» и даём тем
    же ролям в том же объёме. Так работало и до появления матрицы.
  - Право «Согласование бюджета» выделено из «Смены статусов бюджета»:
    ставить статус «Согласован» может только главный экономист.
  - Просмотр истории у ЭП и ИП ограничен их проектами. В таблице 1
    стоит просто «✔», но история — сквозной журнал по всем проектам, а
    доступ к чужим проектам у этих ролей закрыт.
*/
var roleMatrix = map[string]map[string]Scope{
	// Главный экономист — всё и везде.
	RoleGE: {
		PermProjectView: ScopeAll, PermProjectCreate: ScopeAll,
		PermProjectEdit: ScopeAll, PermProjectStatus: ScopeAll,
		PermBudgetView: ScopeAll, PermBudgetCreate: ScopeAll,
		PermBudgetVersion: ScopeAll, PermBudgetEdit: ScopeAll,
		PermBudgetStatus: ScopeAll, PermBudgetApprove: ScopeAll,
		PermBudgetExport: ScopeAll, PermReferencesEdit: ScopeAll,
		PermAuditView: ScopeAll, PermUsersManage: ScopeAll,
	},

	// Экономист проекта — работа с бюджетами назначенных проектов.
	// Первую версию бюджета не создаёт (таблица 1) и не согласовывает.
	RoleEP: {
		PermProjectView: ScopeAssigned, PermProjectEdit: ScopeAssigned,
		PermProjectStatus: ScopeAssigned,
		PermBudgetView:    ScopeAssigned, PermBudgetVersion: ScopeAssigned,
		PermBudgetEdit: ScopeAssigned, PermBudgetStatus: ScopeAssigned,
		PermBudgetExport: ScopeAssigned, PermAuditView: ScopeAssigned,
	},

	// Инициатор проекта — только то, что создал сам.
	RoleIP: {
		PermProjectView: ScopeOwn, PermProjectCreate: ScopeAll,
		PermProjectEdit: ScopeOwn, PermProjectStatus: ScopeOwn,
		PermBudgetView: ScopeOwn, PermBudgetCreate: ScopeOwn,
		PermBudgetVersion: ScopeOwn, PermBudgetEdit: ScopeOwn,
		PermBudgetStatus: ScopeOwn, PermBudgetExport: ScopeOwn,
		PermAuditView: ScopeOwn,
	},

	// Руководитель проекта — просмотр и выгрузка по своим проектам.
	// Смену статуса проекта получает индивидуальным правом (пример из ТЗ).
	RoleRP: {
		PermProjectView: ScopeAssigned, PermBudgetView: ScopeAssigned,
		PermBudgetExport: ScopeAssigned,
	},

	// Администратор проекта — то же, но выгрузка урезана до строк
	// 178-214 листа «2.Бюджет» (см. budgets/export.go).
	RoleAP: {
		PermProjectView: ScopeAssigned, PermBudgetView: ScopeAssigned,
		PermBudgetExport: ScopeAssigned,
	},

	// Руководство — видит все проекты, не меняет ничего.
	RoleManagement: {
		PermProjectView: ScopeAll, PermBudgetView: ScopeAll,
		PermBudgetExport: ScopeAll,
	},
}

// RoleScope — объём базового права роли. Для неизвестной роли и для
// «роль не назначена» возвращает ScopeNone.
func RoleScope(role, perm string) Scope {
	return roleMatrix[role][perm]
}

// RolePermissions — какие права даёт роль (без учёта индивидуальных).
// Порядок — как в AllPermissions, чтобы вывод был стабильным.
func RolePermissions(role string) []string {
	m := roleMatrix[role]
	out := make([]string, 0, len(m))
	for _, p := range AllPermissions {
		if m[p] != ScopeNone {
			out = append(out, p)
		}
	}
	return out
}

// LimitedExport — роль, которой выгрузка отдаётся в урезанном виде.
// ТЗ: «в выгрузке отчета администратор проекта видит информацию только
// по строкам 178-214 листа "2.Бюджет" Формы».
func LimitedExport(role string) bool { return role == RoleAP }
