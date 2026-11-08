package service

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"flowmetrics/internal/apperr"
	"flowmetrics/internal/model"
	"flowmetrics/internal/permissions"
	"flowmetrics/internal/repository"
	"flowmetrics/internal/validate"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type TaskService struct {
	r   *repository.Repos
	log *activityLogger
}

type TaskView struct {
	model.Task
	ProjectName string `json:"project_name"`
}

type CreateTaskInput struct {
	Title, Description, Status, Priority string
	AssigneeID, DueDate                  *string
	Labels                               []string
}

// UpdateTaskInput is a partial update: nil / unset fields are left unchanged.
// AssigneeID and DueDate use Nullable so that an explicit JSON null clears them.
type UpdateTaskInput struct {
	Title, Description, Status, Priority *string
	AssigneeID, DueDate                  model.Nullable[string]
	Labels                               *[]string
}

type TaskListQuery struct {
	ProjectID      string
	Statuses       []string
	Priorities     []string
	Assignee       string // user id, "me", or "unassigned"
	Label, Q, Sort string
	Overdue        bool
	Page           model.PageQuery
}

func today() time.Time {
	n := time.Now().UTC()
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
}

// projectForWrite is the single gate for every task/comment mutation: the project must live in the
// actor's org (else 404), must not be archived, and the actor must be a project member or an admin.
func (s *TaskService) projectForWrite(ctx context.Context, a Actor, projectID bson.ObjectID) (*model.Project, error) {
	p, err := s.r.Projects.Get(ctx, a.OrgID, projectID)
	if err != nil {
		return nil, err
	}
	if p.Status == model.ProjectArchived {
		return nil, apperr.Conflict("project is archived and read-only")
	}
	if !a.Can(permissions.ProjectWriteAny) && !p.HasMember(a.UserID) {
		return nil, apperr.Forbidden("you are not a member of this project")
	}
	return p, nil
}

func (s *TaskService) Create(ctx context.Context, a Actor, projectID bson.ObjectID, in CreateTaskInput) (*TaskView, error) {
	p, err := s.projectForWrite(ctx, a, projectID)
	if err != nil {
		return nil, err
	}
	if in.Status == "" {
		in.Status = model.StatusTodo
	}
	if in.Priority == "" {
		in.Priority = "medium"
	}
	v := validate.Errors{}
	title := v.Text("title", in.Title, 1, 200)
	desc := v.Text("description", in.Description, 0, 10000)
	v.OneOf("status", in.Status, model.TaskStatuses...)
	v.OneOf("priority", in.Priority, model.Priorities...)
	labels := normalizeLabels(v, in.Labels)
	var assignee *bson.ObjectID
	if in.AssigneeID != nil {
		assignee = checkAssignee(v, p, *in.AssigneeID)
	}
	var due *time.Time
	if in.DueDate != nil {
		due = v.Date("due_date", *in.DueDate)
	}
	if err := v.Err(); err != nil {
		return nil, err
	}
	t := &model.Task{
		OrgID: a.OrgID, ProjectID: projectID, Title: title, Description: desc, Status: in.Status,
		Priority: in.Priority, AssigneeID: assignee, ReporterID: a.UserID, DueDate: due, Labels: labels,
	}
	if t.Status == model.StatusDone {
		t.CompletedAt = ptr(time.Now().UTC())
	}
	if err := s.r.Tasks.Create(ctx, t); err != nil {
		return nil, err
	}
	s.log.record(ctx, a, model.Activity{ProjectID: &projectID, TaskID: &t.ID, Action: "task.created", EntityTitle: t.Title})
	return &TaskView{*t, p.Name}, nil
}

func (s *TaskService) Get(ctx context.Context, a Actor, id bson.ObjectID) (*TaskView, error) {
	t, err := s.r.Tasks.Get(ctx, a.OrgID, id)
	if err != nil {
		return nil, err
	}
	views, err := s.hydrate(ctx, a.OrgID, []model.Task{*t})
	if err != nil {
		return nil, err
	}
	return &views[0], nil
}

func (s *TaskService) List(ctx context.Context, a Actor, q TaskListQuery) ([]TaskView, int64, error) {
	v := validate.Errors{}
	f := repository.TaskFilter{Statuses: q.Statuses, Priorities: q.Priorities, Label: strings.ToLower(strings.TrimSpace(q.Label))}
	f.Q = v.Text("q", q.Q, 0, 100)
	for _, st := range q.Statuses {
		v.OneOf("status", st, model.TaskStatuses...)
	}
	for _, pr := range q.Priorities {
		v.OneOf("priority", pr, model.Priorities...)
	}
	switch q.Assignee {
	case "":
	case "unassigned":
		f.Unassigned = true
	case "me":
		f.AssigneeID = &a.UserID
	default:
		if id, ok := parseID(v, "assignee", q.Assignee); ok {
			f.AssigneeID = &id
		}
	}
	if q.Overdue {
		f.OverdueBefore = ptr(today())
	}
	field, desc := parseSort(v, q.Sort, "-updated_at", "created_at", "updated_at", "due_date", "title")
	var projectID *bson.ObjectID
	if q.ProjectID != "" {
		if id, ok := parseID(v, "project_id", q.ProjectID); ok {
			projectID = &id
		}
	}
	if err := v.Err(); err != nil {
		return nil, 0, err
	}
	if projectID != nil {
		// 404 for a project outside this org, exactly like any other tenant-crossing lookup.
		if _, err := s.r.Projects.Get(ctx, a.OrgID, *projectID); err != nil {
			return nil, 0, err
		}
		f.ProjectID = projectID
	} else {
		archived, err := s.r.Projects.ArchivedIDs(ctx, a.OrgID)
		if err != nil {
			return nil, 0, err
		}
		f.ExcludeProjects = archived
	}
	tasks, total, err := s.r.Tasks.List(ctx, a.OrgID, f, field, desc, q.Page)
	if err != nil {
		return nil, 0, err
	}
	views, err := s.hydrate(ctx, a.OrgID, tasks)
	return views, total, err
}

func (s *TaskService) Update(ctx context.Context, a Actor, id bson.ObjectID, in UpdateTaskInput) (*TaskView, error) {
	t, err := s.r.Tasks.Get(ctx, a.OrgID, id)
	if err != nil {
		return nil, err
	}
	p, err := s.projectForWrite(ctx, a, t.ProjectID)
	if err != nil {
		return nil, err
	}

	v := validate.Errors{}
	set := bson.M{}
	var acts []model.Activity
	var changed []string
	act := func(action string, meta map[string]any) {
		acts = append(acts, model.Activity{ProjectID: &t.ProjectID, TaskID: &t.ID, Action: action, EntityTitle: t.Title, Meta: meta})
	}

	if in.Title != nil {
		if title := v.Text("title", *in.Title, 1, 200); title != t.Title {
			set["title"] = title
			changed = append(changed, "title")
		}
	}
	if in.Description != nil {
		if d := v.Text("description", *in.Description, 0, 10000); d != t.Description {
			set["description"] = d
			changed = append(changed, "description")
		}
	}
	if in.Status != nil {
		v.OneOf("status", *in.Status, model.TaskStatuses...)
		if v["status"] == "" && *in.Status != t.Status {
			set["status"] = *in.Status
			switch {
			case *in.Status == model.StatusDone:
				set["completed_at"] = time.Now().UTC()
			case t.Status == model.StatusDone:
				set["completed_at"] = nil
			}
			act("task.status_changed", map[string]any{"from": t.Status, "to": *in.Status})
		}
	}
	if in.Priority != nil {
		v.OneOf("priority", *in.Priority, model.Priorities...)
		if v["priority"] == "" && *in.Priority != t.Priority {
			set["priority"] = *in.Priority
			act("task.priority_changed", map[string]any{"from": t.Priority, "to": *in.Priority})
		}
	}
	if in.AssigneeID.Set {
		var next *bson.ObjectID
		if in.AssigneeID.Value != nil {
			next = checkAssignee(v, p, *in.AssigneeID.Value)
		}
		if v["assignee_id"] == "" && !sameID(next, t.AssigneeID) {
			set["assignee_id"] = next
			meta := map[string]any{"assignee": nil}
			if next != nil {
				if u, err := s.r.Users.FindByID(ctx, *next); err == nil {
					meta["assignee"] = u.Name
				}
			}
			act("task.assigned", meta)
		}
	}
	if in.DueDate.Set {
		var next *time.Time
		if in.DueDate.Value != nil {
			next = v.Date("due_date", *in.DueDate.Value)
		}
		if v["due_date"] == "" && !sameTime(next, t.DueDate) {
			set["due_date"] = next
			changed = append(changed, "due date")
		}
	}
	if in.Labels != nil {
		if labels := normalizeLabels(v, *in.Labels); v["labels"] == "" && strings.Join(labels, ",") != strings.Join(t.Labels, ",") {
			set["labels"] = labels
			changed = append(changed, "labels")
		}
	}
	if err := v.Err(); err != nil {
		return nil, err
	}
	if len(set) == 0 {
		return s.Get(ctx, a, id)
	}
	if len(changed) > 0 {
		act("task.updated", map[string]any{"fields": changed})
	}
	updated, err := s.r.Tasks.Update(ctx, a.OrgID, id, set)
	if err != nil {
		return nil, err
	}
	s.log.record(ctx, a, acts...)
	return &TaskView{*updated, p.Name}, nil
}

// Delete: project members may delete tasks they reported; admins and the owner may delete any.
func (s *TaskService) Delete(ctx context.Context, a Actor, id bson.ObjectID) error {
	t, err := s.r.Tasks.Get(ctx, a.OrgID, id)
	if err != nil {
		return err
	}
	if _, err := s.projectForWrite(ctx, a, t.ProjectID); err != nil {
		return err
	}
	if t.ReporterID != a.UserID && !a.Can(permissions.TaskDeleteAny) {
		return apperr.Forbidden("only the reporter or an admin can delete this task")
	}
	if err := s.r.Tasks.Delete(ctx, a.OrgID, id); err != nil {
		return err
	}
	_ = s.r.Comments.DeleteByTask(ctx, a.OrgID, id)
	s.log.record(ctx, a, model.Activity{ProjectID: &t.ProjectID, Action: "task.deleted", EntityTitle: t.Title})
	return nil
}

// ---- comments & history ----

func (s *TaskService) ListComments(ctx context.Context, a Actor, taskID bson.ObjectID, page model.PageQuery) ([]model.Comment, int64, error) {
	if _, err := s.r.Tasks.Get(ctx, a.OrgID, taskID); err != nil {
		return nil, 0, err
	}
	return s.r.Comments.List(ctx, a.OrgID, taskID, page)
}

func (s *TaskService) AddComment(ctx context.Context, a Actor, taskID bson.ObjectID, body string) (*model.Comment, error) {
	t, err := s.r.Tasks.Get(ctx, a.OrgID, taskID)
	if err != nil {
		return nil, err
	}
	if _, err := s.projectForWrite(ctx, a, t.ProjectID); err != nil {
		return nil, err
	}
	v := validate.Errors{}
	body = v.Text("body", body, 1, 5000)
	if err := v.Err(); err != nil {
		return nil, err
	}
	u, err := s.r.Users.FindByID(ctx, a.UserID)
	if err != nil {
		return nil, err
	}
	c := &model.Comment{OrgID: a.OrgID, TaskID: taskID, AuthorID: a.UserID, AuthorName: u.Name, Body: body}
	if err := s.r.Comments.Create(ctx, c); err != nil {
		return nil, err
	}
	s.log.record(ctx, a, model.Activity{ProjectID: &t.ProjectID, TaskID: &t.ID, Action: "comment.added", EntityTitle: t.Title})
	return c, nil
}

func (s *TaskService) DeleteComment(ctx context.Context, a Actor, taskID, commentID bson.ObjectID) error {
	c, err := s.r.Comments.Get(ctx, a.OrgID, taskID, commentID)
	if err != nil {
		return err
	}
	if c.AuthorID != a.UserID && !a.Can(permissions.CommentDeleteAny) {
		return apperr.Forbidden("you can only delete your own comments")
	}
	return s.r.Comments.Delete(ctx, a.OrgID, commentID)
}

func (s *TaskService) History(ctx context.Context, a Actor, taskID bson.ObjectID, page model.PageQuery) ([]model.Activity, int64, error) {
	if _, err := s.r.Tasks.Get(ctx, a.OrgID, taskID); err != nil {
		return nil, 0, err
	}
	return s.r.Activity.List(ctx, a.OrgID, &taskID, page)
}

// ---- helpers ----

func (s *TaskService) hydrate(ctx context.Context, orgID bson.ObjectID, tasks []model.Task) ([]TaskView, error) {
	seen := map[bson.ObjectID]bool{}
	var pids []bson.ObjectID
	for _, t := range tasks {
		if !seen[t.ProjectID] {
			seen[t.ProjectID] = true
			pids = append(pids, t.ProjectID)
		}
	}
	names := map[bson.ObjectID]string{}
	if len(pids) > 0 {
		projects, err := s.r.Projects.FindByIDs(ctx, orgID, pids)
		if err != nil {
			return nil, err
		}
		for _, p := range projects {
			names[p.ID] = p.Name
		}
	}
	out := make([]TaskView, len(tasks))
	for i, t := range tasks {
		out[i] = TaskView{t, names[t.ProjectID]}
	}
	return out, nil
}

// checkAssignee only accepts members of the task's project, which are by construction members of the org.
func checkAssignee(v validate.Errors, p *model.Project, hex string) *bson.ObjectID {
	id, ok := parseID(v, "assignee_id", hex)
	if !ok {
		return nil
	}
	if !p.HasMember(id) {
		v["assignee_id"] = "must be a member of this project"
		return nil
	}
	return &id
}

func normalizeLabels(v validate.Errors, in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, l := range in {
		l = strings.ToLower(strings.TrimSpace(l))
		if l == "" || seen[l] {
			continue
		}
		if utf8.RuneCountInString(l) > 30 {
			v["labels"] = "each label must be at most 30 characters"
			return out
		}
		seen[l] = true
		out = append(out, l)
	}
	if len(out) > 10 {
		v["labels"] = "at most 10 labels are allowed"
	}
	return out
}

func sameID(a, b *bson.ObjectID) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

func sameTime(a, b *time.Time) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && a.Equal(*b))
}
