package service

import (
	"context"
	"time"

	"flowmetrics/internal/apperr"
	"flowmetrics/internal/model"
	"flowmetrics/internal/permissions"
	"flowmetrics/internal/repository"
	"flowmetrics/internal/validate"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type OrgService struct {
	r   *repository.Repos
	log *activityLogger
}

type OrgView struct {
	model.Organization
	Role model.Role `json:"role"` // the caller's role in this org
}

type MemberView struct {
	UserID   bson.ObjectID `json:"user_id"`
	Name     string        `json:"name"`
	Email    string        `json:"email"`
	Role     model.Role    `json:"role"`
	JoinedAt time.Time     `json:"joined_at"`
}

// Resolve is the tenancy gate used by the HTTP middleware. A non-member gets "not found",
// not "forbidden", so the API never confirms that someone else's organization exists.
func (s *OrgService) Resolve(ctx context.Context, userID, orgID bson.ObjectID) (*Actor, error) {
	m, err := s.r.Members.Find(ctx, orgID, userID)
	if err != nil {
		if isNotFound(err) {
			return nil, apperr.NotFound("organization")
		}
		return nil, err
	}
	return &Actor{UserID: userID, OrgID: orgID, Role: m.Role}, nil
}

func (s *OrgService) Create(ctx context.Context, userID bson.ObjectID, name string) (*OrgView, error) {
	v := validate.Errors{}
	name = v.Text("name", name, 2, 80)
	if err := v.Err(); err != nil {
		return nil, err
	}
	org := &model.Organization{Name: name, OwnerID: userID}
	if err := s.r.Orgs.Create(ctx, org); err != nil {
		return nil, err
	}
	if err := s.r.Members.Create(ctx, &model.Membership{OrgID: org.ID, UserID: userID, Role: model.RoleOwner}); err != nil {
		_ = s.r.Orgs.Delete(ctx, org.ID) // compensate: no transactions on standalone MongoDB
		return nil, err
	}
	return &OrgView{*org, model.RoleOwner}, nil
}

func (s *OrgService) ListForUser(ctx context.Context, userID bson.ObjectID) ([]OrgView, error) {
	ms, err := s.r.Members.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	roles := make(map[bson.ObjectID]model.Role, len(ms))
	orgIDs := make([]bson.ObjectID, len(ms))
	for i, m := range ms {
		roles[m.OrgID], orgIDs[i] = m.Role, m.OrgID
	}
	orgs, err := s.r.Orgs.FindByIDs(ctx, orgIDs)
	if err != nil {
		return nil, err
	}
	out := make([]OrgView, len(orgs))
	for i, o := range orgs {
		out[i] = OrgView{o, roles[o.ID]}
	}
	return out, nil
}

func (s *OrgService) Get(ctx context.Context, a Actor) (*OrgView, error) {
	o, err := s.r.Orgs.FindByID(ctx, a.OrgID)
	if err != nil {
		return nil, err
	}
	return &OrgView{*o, a.Role}, nil
}

func (s *OrgService) Rename(ctx context.Context, a Actor, name string) (*OrgView, error) {
	if err := a.require(permissions.OrgUpdate); err != nil {
		return nil, err
	}
	v := validate.Errors{}
	name = v.Text("name", name, 2, 80)
	if err := v.Err(); err != nil {
		return nil, err
	}
	o, err := s.r.Orgs.Rename(ctx, a.OrgID, name)
	if err != nil {
		return nil, err
	}
	return &OrgView{*o, a.Role}, nil
}

// Delete removes the organization and everything in it. Memberships go first so access
// is cut off immediately, even if a later step fails part-way.
func (s *OrgService) Delete(ctx context.Context, a Actor) error {
	if err := a.require(permissions.OrgDelete); err != nil {
		return err
	}
	steps := []func(context.Context, bson.ObjectID) error{
		s.r.Members.DeleteByOrg, s.r.Tasks.DeleteByOrg, s.r.Comments.DeleteByOrg,
		s.r.Activity.DeleteByOrg, s.r.Projects.DeleteByOrg, s.r.Orgs.Delete,
	}
	for _, step := range steps {
		if err := step(ctx, a.OrgID); err != nil {
			return err
		}
	}
	return nil
}

func (s *OrgService) ListMembers(ctx context.Context, a Actor) ([]MemberView, error) {
	ms, err := s.r.Members.ListByOrg(ctx, a.OrgID)
	if err != nil {
		return nil, err
	}
	userIDs := make([]bson.ObjectID, len(ms))
	for i, m := range ms {
		userIDs[i] = m.UserID
	}
	users, err := s.r.Users.FindByIDs(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	byID := make(map[bson.ObjectID]model.User, len(users))
	for _, u := range users {
		byID[u.ID] = u
	}
	out := make([]MemberView, 0, len(ms))
	for _, m := range ms {
		if u, ok := byID[m.UserID]; ok {
			out = append(out, MemberView{m.UserID, u.Name, u.Email, m.Role, m.CreatedAt})
		}
	}
	return out, nil
}

// AddMember adds an already-registered user by email. (An invitation flow with emailed
// tokens is the natural next step; it needs mail infrastructure this project avoids.)
func (s *OrgService) AddMember(ctx context.Context, a Actor, email, role string) (*MemberView, error) {
	if err := a.require(permissions.MemberAdd); err != nil {
		return nil, err
	}
	v := validate.Errors{}
	email = v.Email("email", email)
	v.OneOf("role", role, string(model.RoleAdmin), string(model.RoleMember))
	if err := v.Err(); err != nil {
		return nil, err
	}
	if !permissions.CanManage(a.Role, model.Role(role)) {
		return nil, apperr.Forbidden("your role cannot add members with this role")
	}
	u, err := s.r.Users.FindByEmail(ctx, email)
	if err != nil {
		if isNotFound(err) {
			return nil, apperr.Validation(map[string]string{"email": "no registered user has this email"})
		}
		return nil, err
	}
	m := &model.Membership{OrgID: a.OrgID, UserID: u.ID, Role: model.Role(role)}
	if err := s.r.Members.Create(ctx, m); err != nil {
		return nil, err
	}
	s.log.record(ctx, a, model.Activity{Action: "member.added", EntityTitle: u.Name, Meta: map[string]any{"role": role}})
	return &MemberView{u.ID, u.Name, u.Email, m.Role, m.CreatedAt}, nil
}

func (s *OrgService) UpdateMemberRole(ctx context.Context, a Actor, targetHex, role string) error {
	if err := a.require(permissions.MemberChangeRole); err != nil {
		return err
	}
	v := validate.Errors{}
	target, ok := parseID(v, "user_id", targetHex)
	v.OneOf("role", role, string(model.RoleAdmin), string(model.RoleMember))
	if err := v.Err(); err != nil || !ok {
		return err
	}
	m, err := s.r.Members.Find(ctx, a.OrgID, target)
	if err != nil {
		return err
	}
	if !permissions.CanManage(a.Role, m.Role) {
		return apperr.Forbidden("the owner's role cannot be changed")
	}
	if err := s.r.Members.UpdateRole(ctx, a.OrgID, target, model.Role(role)); err != nil {
		return err
	}
	if u, err := s.r.Users.FindByID(ctx, target); err == nil {
		s.log.record(ctx, a, model.Activity{Action: "member.role_changed", EntityTitle: u.Name, Meta: map[string]any{"from": m.Role, "to": role}})
	}
	return nil
}

// RemoveMember covers both "remove someone" and "leave" (target == actor).
func (s *OrgService) RemoveMember(ctx context.Context, a Actor, targetHex string) error {
	v := validate.Errors{}
	target, ok := parseID(v, "user_id", targetHex)
	if !ok {
		return apperr.NotFound("member")
	}
	m, err := s.r.Members.Find(ctx, a.OrgID, target)
	if err != nil {
		return err
	}
	if m.UserID == a.UserID {
		if a.Role == model.RoleOwner {
			return apperr.Conflict("the owner cannot leave; delete the organization instead")
		}
	} else if err := a.require(permissions.MemberRemove); err != nil {
		return err
	} else if !permissions.CanManage(a.Role, m.Role) {
		return apperr.Forbidden("your role cannot remove this member")
	}
	if err := s.r.Members.Delete(ctx, a.OrgID, target); err != nil {
		return err
	}
	// Keep referential integrity: drop the user from projects and unassign their tasks.
	if err := s.r.Projects.RemoveUserFromAll(ctx, a.OrgID, target); err != nil {
		return err
	}
	if err := s.r.Tasks.Unassign(ctx, a.OrgID, target, nil); err != nil {
		return err
	}
	if u, err := s.r.Users.FindByID(ctx, target); err == nil {
		s.log.record(ctx, a, model.Activity{Action: "member.removed", EntityTitle: u.Name})
	}
	return nil
}
