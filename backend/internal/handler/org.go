package handler

import (
	"net/http"

	"flowmetrics/internal/middleware"
	"flowmetrics/internal/respond"

	"github.com/gin-gonic/gin"
)

type nameBody struct {
	Name string `json:"name"`
}

func (h *Handler) ListOrgs(c *gin.Context) {
	orgs, err := h.svc.Orgs.ListForUser(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		respond.Error(c, err)
		return
	}
	respond.JSON(c, http.StatusOK, orgs)
}

func (h *Handler) CreateOrg(c *gin.Context) {
	var in nameBody
	if !bind(c, &in) {
		return
	}
	org, err := h.svc.Orgs.Create(c.Request.Context(), middleware.UserID(c), in.Name)
	if err != nil {
		respond.Error(c, err)
		return
	}
	respond.JSON(c, http.StatusCreated, org)
}

func (h *Handler) GetOrg(c *gin.Context) {
	org, err := h.svc.Orgs.Get(c.Request.Context(), middleware.Actor(c))
	if err != nil {
		respond.Error(c, err)
		return
	}
	respond.JSON(c, http.StatusOK, org)
}

func (h *Handler) RenameOrg(c *gin.Context) {
	var in nameBody
	if !bind(c, &in) {
		return
	}
	org, err := h.svc.Orgs.Rename(c.Request.Context(), middleware.Actor(c), in.Name)
	if err != nil {
		respond.Error(c, err)
		return
	}
	respond.JSON(c, http.StatusOK, org)
}

func (h *Handler) DeleteOrg(c *gin.Context) {
	if err := h.svc.Orgs.Delete(c.Request.Context(), middleware.Actor(c)); err != nil {
		respond.Error(c, err)
		return
	}
	respond.NoContent(c)
}

func (h *Handler) ListMembers(c *gin.Context) {
	members, err := h.svc.Orgs.ListMembers(c.Request.Context(), middleware.Actor(c))
	if err != nil {
		respond.Error(c, err)
		return
	}
	respond.JSON(c, http.StatusOK, members)
}

func (h *Handler) AddMember(c *gin.Context) {
	var in struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if !bind(c, &in) {
		return
	}
	m, err := h.svc.Orgs.AddMember(c.Request.Context(), middleware.Actor(c), in.Email, in.Role)
	if err != nil {
		respond.Error(c, err)
		return
	}
	respond.JSON(c, http.StatusCreated, m)
}

func (h *Handler) UpdateMemberRole(c *gin.Context) {
	var in struct {
		Role string `json:"role"`
	}
	if !bind(c, &in) {
		return
	}
	if err := h.svc.Orgs.UpdateMemberRole(c.Request.Context(), middleware.Actor(c), c.Param("userId"), in.Role); err != nil {
		respond.Error(c, err)
		return
	}
	respond.NoContent(c)
}

func (h *Handler) RemoveMember(c *gin.Context) {
	if err := h.svc.Orgs.RemoveMember(c.Request.Context(), middleware.Actor(c), c.Param("userId")); err != nil {
		respond.Error(c, err)
		return
	}
	respond.NoContent(c)
}
