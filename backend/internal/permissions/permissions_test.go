package permissions

import (
	"testing"

	"flowmetrics/internal/model"
)

func TestMatrix(t *testing.T) {
	cases := []struct {
		role model.Role
		perm Permission
		want bool
	}{
		{owner, OrgDelete, true}, {admin, OrgDelete, false}, {member, OrgDelete, false},
		{owner, ProjectCreate, true}, {admin, ProjectCreate, true}, {member, ProjectCreate, false},
		{admin, MemberChangeRole, false}, {owner, MemberChangeRole, true},
		{member, TaskDeleteAny, false}, {admin, TaskDeleteAny, true},
		{"", ProjectCreate, false}, {"superuser", OrgDelete, false},
	}
	for _, c := range cases {
		if got := Can(c.role, c.perm); got != c.want {
			t.Errorf("Can(%q, %q) = %v, want %v", c.role, c.perm, got, c.want)
		}
	}
}

func TestCanManage(t *testing.T) {
	cases := []struct {
		actor, target model.Role
		want          bool
	}{
		{owner, admin, true}, {owner, member, true}, {owner, owner, false},
		{admin, member, true}, {admin, admin, false}, {admin, owner, false},
		{member, member, false},
	}
	for _, c := range cases {
		if got := CanManage(c.actor, c.target); got != c.want {
			t.Errorf("CanManage(%s, %s) = %v, want %v", c.actor, c.target, got, c.want)
		}
	}
}
