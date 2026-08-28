package auth

import "testing"

// Ролевая матрица — предмет договорённости с заказчиком (ТЗ, таблица 1),
// а не деталь реализации. Поэтому она проверяется целиком: любая правка
// roleMatrix, не согласованная с этой таблицей, роняет тест.
//
// Читается как таблица: строка — право, колонки — роли в порядке
// ГЭ, ЭП, ИП, РП, АП, Руководство.
func TestRoleMatrixMatchesSpec(t *testing.T) {
	const (
		x = ScopeNone
		o = ScopeOwn
		a = ScopeAssigned
		A = ScopeAll
	)
	roles := []string{RoleGE, RoleEP, RoleIP, RoleRP, RoleAP, RoleManagement}

	want := map[string][6]Scope{
		//                     ГЭ ЭП ИП РП АП Рук
		PermProjectView:    {A, a, o, a, a, A},
		PermProjectCreate:  {A, x, A, x, x, x},
		PermProjectEdit:    {A, a, o, x, x, x},
		PermProjectStatus:  {A, a, o, x, x, x},
		PermBudgetView:     {A, a, o, a, a, A},
		PermBudgetCreate:   {A, x, o, x, x, x},
		PermBudgetVersion:  {A, a, o, x, x, x},
		PermBudgetEdit:     {A, a, o, x, x, x},
		PermBudgetStatus:   {A, a, o, x, x, x},
		PermBudgetApprove:  {A, x, x, x, x, x},
		PermBudgetExport:   {A, a, o, a, a, A},
		PermReferencesEdit: {A, x, x, x, x, x},
		PermAuditView:      {A, a, o, x, x, x},
		PermUsersManage:    {A, x, x, x, x, x},
	}

	if len(want) != len(AllPermissions) {
		t.Fatalf("в таблице %d прав, в AllPermissions %d — список разошёлся",
			len(want), len(AllPermissions))
	}

	for perm, row := range want {
		for i, role := range roles {
			if got := RoleScope(role, perm); got != row[i] {
				t.Errorf("%s / %s: получили %v, ожидали %v",
					RoleLabels[role], PermissionLabels[perm], got, row[i])
			}
		}
	}
}

// Пустая роль не даёт ничего: ТЗ требует, чтобы после создания учётки
// пользователь не видел функциональности до назначения роли.
func TestRoleNoneHasNoPermissions(t *testing.T) {
	for _, p := range AllPermissions {
		if RoleScope(RoleNone, p) != ScopeNone {
			t.Errorf("роль не назначена, но %s разрешено", p)
		}
	}
	if got := RolePermissions(RoleNone); len(got) != 0 {
		t.Errorf("RolePermissions(без роли) = %v, ожидали пусто", got)
	}
}

func TestUnknownRoleHasNoPermissions(t *testing.T) {
	for _, p := range AllPermissions {
		if RoleScope("ADMIN", p) != ScopeNone {
			t.Errorf("неизвестной роли разрешено %s", p)
		}
	}
}

// Управление пользователями нельзя выдать индивидуальным правом: иначе
// вместо «права сверх роли» получился бы второй главный экономист.
func TestUsersManageIsNotGrantable(t *testing.T) {
	if IsGrantable(PermUsersManage) {
		t.Error("users.manage выдаётся индивидуально, а не должно")
	}
	for _, p := range AllPermissions {
		if p == PermUsersManage {
			continue
		}
		if !IsGrantable(p) {
			t.Errorf("%s нельзя выдать индивидуально, а должно быть можно", p)
		}
	}
	if !IsGrantable(PermAll) {
		t.Error("«все права» должны выдаваться (тумблер на один проект)")
	}
	if IsGrantable("нет.такого.права") {
		t.Error("выдалось несуществующее право")
	}
}

// Права на чтение открываются назначением на проект без галки
// «может редактировать»; всё остальное — только с ней.
func TestReadOnlySet(t *testing.T) {
	readable := map[string]bool{
		PermProjectView: true, PermBudgetView: true,
		PermBudgetExport: true, PermAuditView: true,
	}
	for _, p := range AllPermissions {
		if IsReadOnly(p) != readable[p] {
			t.Errorf("%s: IsReadOnly=%v, ожидали %v", p, IsReadOnly(p), readable[p])
		}
	}
}

// Внепроектные права нельзя привязать к одному проекту.
func TestProjectScopedSet(t *testing.T) {
	global := map[string]bool{
		PermProjectCreate: true, PermReferencesEdit: true, PermUsersManage: true,
	}
	for _, p := range AllPermissions {
		if IsProjectScoped(p) == global[p] {
			t.Errorf("%s: IsProjectScoped=%v — противоречит ожиданию",
				p, IsProjectScoped(p))
		}
	}
}

// Урезанная выгрузка (строки 178-214 листа «2.Бюджет») — только у
// администратора проекта.
func TestLimitedExportOnlyForAP(t *testing.T) {
	for _, r := range AllRoles {
		if want := r == RoleAP; LimitedExport(r) != want {
			t.Errorf("LimitedExport(%s) = %v, ожидали %v", r, LimitedExport(r), want)
		}
	}
}

func TestIsKnownRole(t *testing.T) {
	for _, r := range AllRoles {
		if !IsKnownRole(r) {
			t.Errorf("роль %s не признана известной", r)
		}
	}
	if !IsKnownRole(RoleNone) {
		t.Error("пустая роль должна приниматься: учётка без роли допустима")
	}
	if IsKnownRole("ROOT") {
		t.Error("принята несуществующая роль")
	}
}
