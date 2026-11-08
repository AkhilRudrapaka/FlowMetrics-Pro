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

type ProjectService struct {
	r   *repository.Repos
	log *activityLogger
}

type ProjectView struct {
	model.Project
	TaskCount int64 `json:"task_count"`
	DoneCount int64 `json:"done_count"`
}

type ProjectListQuery struct {
	Q, Status, Sort string
	Page            model.PageQuery
}

type CreateProjectInput struct {
	Name, Description string
	MemberIDs         []string
}

type UpdateProjectInput struct{ Name, Description, Status *string }

func (s *ProjectService) List(ctx context.Context, a Actor, q ProjectListQuery) ([]ProjectView, int64, error) {
	v := validate.Errors{}
	if q.Status != "" {
		v.OneOf("status", q.Status, model.ProjectStatuses...)
	}
	field, desc := parseSort(v, q.Sort, "-updated_at", "name", "created_at", "updated_at")
	if err := v.Err(); err != nil {
		return nil, 0, err
	}
	items, total, err := s.r.Projects.List(ctx, a.OrgID, repository.ProjectFilter{Q: q.Q, Status: q.Status}, field, desc, q.Page)
	if err != nil {
		return nil, 0, err
	}
	views, err := s.withCounts(ctx, a.OrgID, items)
	return views, total, err
}

func (s *ProjectService) Get(ctx context.Context, a Actor, id bson.ObjectID) (*ProjectView, error) {
	p, err := s.r.Projects.Get(ctx, a.OrgID, id)
	if err != nil {
		return nil, err
	}
	views, err := s.withCounts(ctx, a.OrgID, []model.Project{*p})
	if err != nil {
		return nil, err
	}
	return &views[0], nil
}

func (s *ProjectService) Create(ctx context.Context, a Actor, in CreateProjectInput) (*ProjectView, error) {
	if err := a.require(permissions.ProjectCreate); err != nil {
		return nil, err
	}
	v := validate.Errors{}
	name := v.Text("name", in.Name, 1, 100)
	desc := v.Text("description", in.Description, 0, 2000)
	members := s.parseMembers(ctx, v, a.OrgID, in.MemberIDs)
	if err := v.Err(); err != nil {
		return nil, err
	}
	p := &model.Project{
		OrgID: a.OrgID, Name: name, Description: desc, Status: model.ProjectActive,
		MemberIDs: appendUnique(members, a.UserID), CreatedBy: a.UserID,
	}
	if err := s.r.Projects.Create(ctx, p); err != nil {
		return nil, err
	}
	s.log.record(ctx, a, model.Activity{ProjectID: &p.ID, Action: "project.created", EntityTitle: p.Name})
	return &ProjectView{Project: *p}, nil
}

func (s *ProjectService) Update(ctx context.Context, a Actor, id bson.ObjectID, in UpdateProjectInput) (*ProjectView, error) {
	if err := a.require(permissions.ProjectUpdate); err != nil {
		return nil, err
	}
	p, err := s.r.Projects.Get(ctx, a.OrgID, id)
	if err != nil {
		return nil, err
	}
	v := validate.Errors{}
	set := bson.M{}
	if in.Name != nil {
		set["name"] = v.Text("name", *in.Name, 1, 100)
	}
	if in.Description != nil {
		set["description"] = v.Text("description", *in.Description, 0, 2000)
	}
	if in.Status != nil {
		v.OneOf("status", *in.Status, model.ProjectStatuses...)
	}
	if err := v.Err(); err != nil {
		return nil, err
	}
	// An archived project is frozen: the only permitted edit is restoring it.
	if p.Status == model.ProjectArchived && (in.Name != nil || in.Description != nil) && (in.Status == nil || *in.Status == model.ProjectArchived) {
		return nil, apperr.Conflict("project is archived; restore it before editing")
	}
	action := "project.updated"
	if in.Status != nil && *in.Status != p.Status {
		set["status"] = *in.Status
		if *in.Status == model.ProjectArchived {
			set["archived_at"] = time.Now().UTC()
			action = "project.archived"
		} else {
			set["archived_at"] = nil
		}
	}
	if len(set) == 0 {
		return s.Get(ctx, a, id)
	}
	updated, err := s.r.Projects.Update(ctx, a.OrgID, id, set)
	if err != nil {
		return nil, err
	}
	s.log.record(ctx, a, model.Activity{ProjectID: &id, Action: action, EntityTitle: updated.Name})
	return s.Get(ctx, a, id)
}

func (s *ProjectService) Archive(ctx context.Context, a Actor, id bson.ObjectID) (*ProjectView, error) {
	return s.Update(ctx, a, id, UpdateProjectInput{Status: ptr(model.ProjectArchived)})
}

func (s *ProjectService) AddMember(ctx context.Context, a Actor, projectID bson.ObjectID, userHex string) (*ProjectView, error) {
	if err := a.require(permissions.ProjectManageMembers); err != nil {
		return nil, err
	}
	p, err := s.r.Projects.Get(ctx, a.OrgID, projectID)
	if err != nil {
		return nil, err
	}
	if p.Status == model.ProjectArchived {
		return nil, apperr.Conflict("project is archived")
	}
	v := validate.Errors{}
	uid, ok := parseID(v, "user_id", userHex)
	if !ok {
		return nil, v.Err()
	}
	if _, err := s.r.Members.Find(ctx, a.OrgID, uid); err != nil { // must already belong to this org
		if isNotFound(err) {
			return nil, apperr.Validation(map[string]string{"user_id": "is not a member of this organization"})
		}
		return nil, err
	}
	if _, err := s.r.Projects.AddMember(ctx, a.OrgID, projectID, uid); err != nil {
		return nil, err
	}
	if u, err := s.r.Users.FindByID(ctx, uid); err == nil {
		s.log.record(ctx, a, model.Activity{ProjectID: &projectID, Action: "project.member_added", EntityTitle: p.Name, Meta: map[string]any{"member": u.Name}})
	}
	return s.Get(ctx, a, projectID)
}

func (s *ProjectService) RemoveMember(ctx context.Context, a Actor, projectID bson.ObjectID, userHex string) (*ProjectView, error) {
	if err := a.require(permissions.ProjectManageMembers); err != nil {
		return nil, err
	}
	p, err := s.r.Projects.Get(ctx, a.OrgID, projectID)
	if err != nil {
		return nil, err
	}
	if p.Status == model.ProjectArchived {
		return nil, apperr.Conflict("project is archived")
	}
	v := validate.Errors{}
	uid, ok := parseID(v, "user_id", userHex)
	if !ok {
		return nil, v.Err()
	}
	if !p.HasMember(uid) {
		return nil, apperr.NotFound("project member")
	}
	if _, err := s.r.Projects.RemoveMember(ctx, a.OrgID, projectID, uid); err != nil {
		return nil, err
	}
	if err := s.r.Tasks.Unassign(ctx, a.OrgID, uid, &projectID); err != nil {
		return nil, err
	}
	if u, err := s.r.Users.FindByID(ctx, uid); err == nil {
		s.log.record(ctx, a, model.Activity{ProjectID: &projectID, Action: "project.member_removed", EntityTitle: p.Name, Meta: map[string]any{"member": u.Name}})
	}
	return s.Get(ctx, a, projectID)
}

func (s *ProjectService) withCounts(ctx context.Context, orgID bson.ObjectID, items []model.Project) ([]ProjectView, error) {
	pids := make([]bson.ObjectID, len(items))
	for i, p := range items {
		pids[i] = p.ID
	}
	counts, err := s.r.Tasks.CountsByProject(ctx, orgID, pids)
	if err != nil {
		return nil, err
	}
	out := make([]ProjectView, len(items))
	for i, p := range items {
		c := counts[p.ID]
		out[i] = ProjectView{p, c.Total, c.Done}
	}
	return out, nil
}

// parseMembers validates hex ids and that every one belongs to the org (a user from another
// tenant can never be attached to this tenant's project).
func (s *ProjectService) parseMembers(ctx context.Context, v validate.Errors, orgID bson.ObjectID, hexes []string) []bson.ObjectID {
	out := []bson.ObjectID{}
	for _, h := range hexes {
		id, err := bson.ObjectIDFromHex(h)
		if err != nil {
			v["member_ids"] = "contains an invalid id"
			return out
		}
		if _, err := s.r.Members.Find(ctx, orgID, id); err != nil {
			v["member_ids"] = "contains a user who is not a member of this organization"
			return out
		}
		out = appendUnique(out, id)
	}
	return out
}

func appendUnique(list []bson.ObjectID, id bson.ObjectID) []bson.ObjectID {
	for _, x := range list {
		if x == id {
			return list
		}
	}
	return append(list, id)
}
