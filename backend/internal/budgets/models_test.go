package budgets

import (
	"testing"

	"ibcon-budget/internal/auth"
)

func TestIsFrozen(t *testing.T) {
	cases := map[string]bool{
		StatusDraft:       false,
		StatusUnderReview: false,
		StatusApproved:    true,
		StatusArchive:     true,
	}
	for status, want := range cases {
		if got := IsFrozen(status); got != want {
			t.Errorf("IsFrozen(%q) = %v, ожидалось %v", status, got, want)
		}
	}
}

// Правку согласованной и архивной версии открываем только автору и
// главному экономисту — правило владельца 2026-08-27.
func TestCanEditFrozenVersion(t *testing.T) {
	const author, other = 10, 20

	cases := []struct {
		name      string
		role      string
		userID    int
		createdBy int
		want      bool
	}{
		{"главный экономист, чужая версия", auth.RoleGE, other, author, true},
		{"главный экономист, своя версия", auth.RoleGE, author, author, true},
		{"автор версии", auth.RoleEP, author, author, true},
		{"экономист проекта, чужая версия", auth.RoleEP, other, author, false},
		{"инициатор, чужая версия", auth.RoleIP, other, author, false},
		{"инициатор, своя версия", auth.RoleIP, author, author, true},
		{"администратор проекта, чужая версия", auth.RoleAP, other, author, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := CanEditFrozenVersion(c.role, c.userID, c.createdBy)
			if got != c.want {
				t.Errorf("CanEditFrozenVersion(%q, %d, %d) = %v, ожидалось %v",
					c.role, c.userID, c.createdBy, got, c.want)
			}
		})
	}
}
