package service

import (
	"context"

	"flowmetrics/internal/model"
	"flowmetrics/internal/repository"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type DashboardService struct{ r *repository.Repos }

// Dashboard counters exclude archived projects (and their tasks), which are read-only history.
// "Active" tasks are those in todo, in_progress, in_review or blocked; backlog is not yet active work.
type Dashboard struct {
	TotalProjects  int64            `json:"total_projects"`
	ActiveProjects int64            `json:"active_projects"`
	TotalTasks     int64            `json:"total_tasks"`
	ActiveTasks    int64            `json:"active_tasks"`
	CompletedTasks int64            `json:"completed_tasks"`
	OverdueTasks   int64            `json:"overdue_tasks"`
	MyOpenTasks    int64            `json:"my_open_tasks"`
	TasksByStatus  map[string]int64 `json:"tasks_by_status"`
	TasksByPrio    map[string]int64 `json:"tasks_by_priority"`
	RecentActivity []model.Activity `json:"recent_activity"`
}

func (s *DashboardService) Get(ctx context.Context, a Actor) (*Dashboard, error) {
	archived, err := s.r.Projects.ArchivedIDs(ctx, a.OrgID)
	if err != nil {
		return nil, err
	}
	d := &Dashboard{}
	if d.TotalProjects, err = s.r.Projects.Count(ctx, a.OrgID, bson.M{"status": bson.M{"$ne": model.ProjectArchived}}); err != nil {
		return nil, err
	}
	if d.ActiveProjects, err = s.r.Projects.Count(ctx, a.OrgID, bson.M{"status": model.ProjectActive}); err != nil {
		return nil, err
	}
	stats, err := s.r.Tasks.Stats(ctx, a.OrgID, a.UserID, archived, today())
	if err != nil {
		return nil, err
	}
	d.TasksByStatus, d.TasksByPrio = map[string]int64{}, stats.ByPriority
	for _, st := range model.TaskStatuses { // zero-fill so the UI always gets every column
		d.TasksByStatus[st] = stats.ByStatus[st]
		d.TotalTasks += stats.ByStatus[st]
	}
	for _, st := range model.ActiveTaskStatuses {
		d.ActiveTasks += stats.ByStatus[st]
	}
	d.CompletedTasks, d.OverdueTasks, d.MyOpenTasks = stats.ByStatus[model.StatusDone], stats.Overdue, stats.Mine
	d.RecentActivity, _, err = s.r.Activity.List(ctx, a.OrgID, nil, model.PageQuery{Page: 1, Size: 10})
	return d, err
}

func (s *DashboardService) Activity(ctx context.Context, a Actor, page model.PageQuery) ([]model.Activity, int64, error) {
	return s.r.Activity.List(ctx, a.OrgID, nil, page)
}
