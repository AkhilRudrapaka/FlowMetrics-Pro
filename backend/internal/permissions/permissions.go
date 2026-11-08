// Package permissions is the single source of truth for what each org role may do.
// Resource-level rules (e.g. "must be a project member") live in the services, but they
// always start from a check here.
package permissions

import "flowmetrics/internal/model"

type Permission string

const (
	OrgUpdate            Permission = "org:update"
	OrgDelete            Permission = "org:delete"
	MemberAdd            Permission = "member:add"
	MemberRemove         Permission = "member:remove"
	MemberChangeRole     Permission = "member:change_role"
	ProjectCreate        Permission = "project:create"
	ProjectUpdate        Permission = "project:update"
	ProjectManageMembers Permission = "project:manage_members"
	ProjectWriteAny      Permission = "project:write_any" // write tasks in projects you are not a member of
	TaskDeleteAny        Permission = "task:delete_any"   // delete tasks you did not report
	CommentDeleteAny     Permission = "comment:delete_any"
)

const (
	owner  = model.RoleOwner
	admin  = model.RoleAdmin
	member = model.RoleMember
)

var grants = map[Permission][]model.Role{
	OrgUpdate:            {owner, admin},
	OrgDelete:            {owner},
	MemberAdd:            {owner, admin},
	MemberRemove:         {owner, admin},
	MemberChangeRole:     {owner},
	ProjectCreate:        {owner, admin},
	ProjectUpdate:        {owner, admin},
	ProjectManageMembers: {owner, admin},
	ProjectWriteAny:      {owner, admin},
	TaskDeleteAny:        {owner, admin},
	CommentDeleteAny:     {owner, admin},
}

func Can(role model.Role, p Permission) bool {
	for _, r := range grants[p] {
		if r == role {
			return true
		}
	}
	return false
}

// CanManage reports whether an actor may add, remove, or act on a member holding targetRole.
// The owner cannot be managed by anyone; admins manage only plain members.
func CanManage(actor, target model.Role) bool {
	switch actor {
	case owner:
		return target != owner
	case admin:
		return target == member
	}
	return false
}
