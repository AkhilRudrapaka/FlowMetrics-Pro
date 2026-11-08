package handler

import (
	"net/http"

	"flowmetrics/internal/middleware"
	"flowmetrics/internal/model"
	"flowmetrics/internal/respond"
	"flowmetrics/internal/service"

	"github.com/gin-gonic/gin"
)

func (h *Handler) CreateTask(c *gin.Context) {
	pid, ok := pathID(c, "projectId", "project")
	if !ok {
		return
	}
	var in struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Status      string   `json:"status"`
		Priority    string   `json:"priority"`
		AssigneeID  *string  `json:"assignee_id"`
		DueDate     *string  `json:"due_date"`
		Labels      []string `json:"labels"`
	}
	if !bind(c, &in) {
		return
	}
	t, err := h.svc.Tasks.Create(c.Request.Context(), middleware.Actor(c), pid, service.CreateTaskInput(in))
	if err != nil {
		respond.Error(c, err)
		return
	}
	respond.JSON(c, http.StatusCreated, t)
}

func (h *Handler) ListTasks(c *gin.Context) {
	page, ok := pageQuery(c, 20)
	if !ok {
		return
	}
	items, total, err := h.svc.Tasks.List(c.Request.Context(), middleware.Actor(c), service.TaskListQuery{
		ProjectID: c.Query("project_id"), Statuses: csv(c, "status"), Priorities: csv(c, "priority"),
		Assignee: c.Query("assignee"), Label: c.Query("label"), Q: c.Query("q"), Sort: c.Query("sort"),
		Overdue: c.Query("overdue") == "true", Page: page,
	})
	if err != nil {
		respond.Error(c, err)
		return
	}
	respond.List(c, items, model.NewPageMeta(page, total))
}

func (h *Handler) GetTask(c *gin.Context) {
	id, ok := pathID(c, "taskId", "task")
	if !ok {
		return
	}
	t, err := h.svc.Tasks.Get(c.Request.Context(), middleware.Actor(c), id)
	if err != nil {
		respond.Error(c, err)
		return
	}
	respond.JSON(c, http.StatusOK, t)
}

func (h *Handler) UpdateTask(c *gin.Context) {
	id, ok := pathID(c, "taskId", "task")
	if !ok {
		return
	}
	var in struct {
		Title       *string                `json:"title"`
		Description *string                `json:"description"`
		Status      *string                `json:"status"`
		Priority    *string                `json:"priority"`
		AssigneeID  model.Nullable[string] `json:"assignee_id"`
		DueDate     model.Nullable[string] `json:"due_date"`
		Labels      *[]string              `json:"labels"`
	}
	if !bind(c, &in) {
		return
	}
	t, err := h.svc.Tasks.Update(c.Request.Context(), middleware.Actor(c), id, service.UpdateTaskInput(in))
	if err != nil {
		respond.Error(c, err)
		return
	}
	respond.JSON(c, http.StatusOK, t)
}

func (h *Handler) DeleteTask(c *gin.Context) {
	id, ok := pathID(c, "taskId", "task")
	if !ok {
		return
	}
	if err := h.svc.Tasks.Delete(c.Request.Context(), middleware.Actor(c), id); err != nil {
		respond.Error(c, err)
		return
	}
	respond.NoContent(c)
}

func (h *Handler) ListComments(c *gin.Context) {
	id, ok := pathID(c, "taskId", "task")
	if !ok {
		return
	}
	page, ok := pageQuery(c, 50)
	if !ok {
		return
	}
	items, total, err := h.svc.Tasks.ListComments(c.Request.Context(), middleware.Actor(c), id, page)
	if err != nil {
		respond.Error(c, err)
		return
	}
	respond.List(c, items, model.NewPageMeta(page, total))
}

func (h *Handler) AddComment(c *gin.Context) {
	id, ok := pathID(c, "taskId", "task")
	if !ok {
		return
	}
	var in struct {
		Body string `json:"body"`
	}
	if !bind(c, &in) {
		return
	}
	cm, err := h.svc.Tasks.AddComment(c.Request.Context(), middleware.Actor(c), id, in.Body)
	if err != nil {
		respond.Error(c, err)
		return
	}
	respond.JSON(c, http.StatusCreated, cm)
}

func (h *Handler) DeleteComment(c *gin.Context) {
	tid, ok := pathID(c, "taskId", "task")
	if !ok {
		return
	}
	cid, ok := pathID(c, "commentId", "comment")
	if !ok {
		return
	}
	if err := h.svc.Tasks.DeleteComment(c.Request.Context(), middleware.Actor(c), tid, cid); err != nil {
		respond.Error(c, err)
		return
	}
	respond.NoContent(c)
}

func (h *Handler) TaskHistory(c *gin.Context) {
	id, ok := pathID(c, "taskId", "task")
	if !ok {
		return
	}
	page, ok := pageQuery(c, 50)
	if !ok {
		return
	}
	items, total, err := h.svc.Tasks.History(c.Request.Context(), middleware.Actor(c), id, page)
	if err != nil {
		respond.Error(c, err)
		return
	}
	respond.List(c, items, model.NewPageMeta(page, total))
}
