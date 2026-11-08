package handler

import (
	"net/http"

	"flowmetrics/internal/middleware"
	"flowmetrics/internal/model"
	"flowmetrics/internal/respond"
	"flowmetrics/internal/service"

	"github.com/gin-gonic/gin"
)

func (h *Handler) ListProjects(c *gin.Context) {
	page, ok := pageQuery(c, 20)
	if !ok {
		return
	}
	items, total, err := h.svc.Projects.List(c.Request.Context(), middleware.Actor(c), service.ProjectListQuery{
		Q: c.Query("q"), Status: c.Query("status"), Sort: c.Query("sort"), Page: page,
	})
	if err != nil {
		respond.Error(c, err)
		return
	}
	respond.List(c, items, model.NewPageMeta(page, total))
}

func (h *Handler) CreateProject(c *gin.Context) {
	var in struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		MemberIDs   []string `json:"member_ids"`
	}
	if !bind(c, &in) {
		return
	}
	p, err := h.svc.Projects.Create(c.Request.Context(), middleware.Actor(c), service.CreateProjectInput{
		Name: in.Name, Description: in.Description, MemberIDs: in.MemberIDs,
	})
	if err != nil {
		respond.Error(c, err)
		return
	}
	respond.JSON(c, http.StatusCreated, p)
}

func (h *Handler) GetProject(c *gin.Context) {
	id, ok := pathID(c, "projectId", "project")
	if !ok {
		return
	}
	p, err := h.svc.Projects.Get(c.Request.Context(), middleware.Actor(c), id)
	if err != nil {
		respond.Error(c, err)
		return
	}
	respond.JSON(c, http.StatusOK, p)
}

func (h *Handler) UpdateProject(c *gin.Context) {
	id, ok := pathID(c, "projectId", "project")
	if !ok {
		return
	}
	var in struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Status      *string `json:"status"`
	}
	if !bind(c, &in) {
		return
	}
	p, err := h.svc.Projects.Update(c.Request.Context(), middleware.Actor(c), id, service.UpdateProjectInput(in))
	if err != nil {
		respond.Error(c, err)
		return
	}
	respond.JSON(c, http.StatusOK, p)
}

func (h *Handler) ArchiveProject(c *gin.Context) {
	id, ok := pathID(c, "projectId", "project")
	if !ok {
		return
	}
	p, err := h.svc.Projects.Archive(c.Request.Context(), middleware.Actor(c), id)
	if err != nil {
		respond.Error(c, err)
		return
	}
	respond.JSON(c, http.StatusOK, p)
}

func (h *Handler) AddProjectMember(c *gin.Context) {
	id, ok := pathID(c, "projectId", "project")
	if !ok {
		return
	}
	p, err := h.svc.Projects.AddMember(c.Request.Context(), middleware.Actor(c), id, c.Param("userId"))
	if err != nil {
		respond.Error(c, err)
		return
	}
	respond.JSON(c, http.StatusOK, p)
}

func (h *Handler) RemoveProjectMember(c *gin.Context) {
	id, ok := pathID(c, "projectId", "project")
	if !ok {
		return
	}
	p, err := h.svc.Projects.RemoveMember(c.Request.Context(), middleware.Actor(c), id, c.Param("userId"))
	if err != nil {
		respond.Error(c, err)
		return
	}
	respond.JSON(c, http.StatusOK, p)
}
