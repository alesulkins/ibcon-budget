package access

import (
	"testing"

	"ibcon-budget/internal/auth"
)

// «Звёздочка» — тумблер «все права на один проект». Она не должна
// открывать управление пользователями: иначе выдача прав на проект
// молча делала бы человека вторым главным экономистом.
//
// Проверка живая: именно на этом месте была дыра — запрос GET /users
// от учётки без роли с «*» на один проект возвращал 200.
func TestStarGrantDoesNotCoverUsersManage(t *testing.T) {
	exact, star := grantCodes(auth.PermUsersManage)
	if star == auth.PermAll {
		t.Error("«*» покрывает управление пользователями")
	}
	if exact != auth.PermUsersManage || star != auth.PermUsersManage {
		t.Errorf("для непередаваемого права ожидали два одинаковых кода, получили %q и %q",
			exact, star)
	}
}

// Для всех остальных прав «звёздочка» работает — ради неё тумблер и есть.
func TestStarGrantCoversGrantablePermissions(t *testing.T) {
	for _, p := range auth.AllPermissions {
		if p == auth.PermUsersManage {
			continue
		}
		exact, star := grantCodes(p)
		if exact != p || star != auth.PermAll {
			t.Errorf("%s: ожидали (%s, %s), получили (%s, %s)",
				p, p, auth.PermAll, exact, star)
		}
	}
}

// Право нельзя привязать к проекту, если оно действует глобально:
// «создание проекта на проекте №14» — бессмыслица.
func TestValidateGrant(t *testing.T) {
	pid := 14

	cases := []struct {
		name    string
		perm    string
		project *int
		wantErr error
	}{
		{"право на все проекты", auth.PermBudgetEdit, nil, nil},
		{"право на один проект", auth.PermBudgetEdit, &pid, nil},
		{"все права на один проект", auth.PermAll, &pid, nil},
		{"управление пользователями", auth.PermUsersManage, nil, ErrNotGrantable},
		{"справочники на один проект", auth.PermReferencesEdit, &pid, ErrProjectNotAllowed},
		{"создание проекта на один проект", auth.PermProjectCreate, &pid, ErrProjectNotAllowed},
		{"несуществующее право", "project.destroy", nil, ErrUnknownPermission},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := validateGrant(c.perm, c.project); got != c.wantErr {
				t.Errorf("получили %v, ожидали %v", got, c.wantErr)
			}
		})
	}
}
