package handler

import (
	"net/http"

	"flowmetrics/internal/middleware"
	"flowmetrics/internal/model"
	"flowmetrics/internal/respond"

	"github.com/gin-gonic/gin"
)

func (h *Handler) Dashboard(c *gin.Context) {
	d, err := h.svc.Dashboard.Get(c.Request.Context(), middleware.Actor(c))
	if err != nil {
		respond.Error(c, err)
		return
	}
	respond.JSON(c, http.StatusOK, d)
}

func (h *Handler) OrgActivity(c *gin.Context) {
	page, ok := pageQuery(c, 20)
	if !ok {
		return
	}
	items, total, err := h.svc.Dashboard.Activity(c.Request.Context(), middleware.Actor(c), page)
	if err != nil {
		respond.Error(c, err)
		return
	}
	respond.List(c, items, model.NewPageMeta(page, total))
}
