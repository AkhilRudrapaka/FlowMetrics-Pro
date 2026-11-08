// Package handler translates HTTP into service calls. It contains no business rules and no
// authorization decisions beyond "who is calling" (see the middleware package).
package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"flowmetrics/internal/apperr"
	"flowmetrics/internal/config"
	"flowmetrics/internal/middleware"
	"flowmetrics/internal/model"
	"flowmetrics/internal/respond"
	"flowmetrics/internal/service"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type Handler struct {
	svc *service.Services
	cfg *config.Config
}

// NewRouter wires every route. ping backs the health check (database reachability).
func NewRouter(cfg *config.Config, svc *service.Services, ping func(context.Context) error) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	binding.EnableDecoderDisallowUnknownFields = true // typos in field names fail loudly instead of being ignored

	h := &Handler{svc, cfg}
	r := gin.New()
	_ = r.SetTrustedProxies(cfg.TrustedProxies) // nil disables X-Forwarded-For trust
	r.Use(middleware.RequestLog(), middleware.Recover(), middleware.SecurityHeaders(), middleware.BodyLimit(1<<20))
	if len(cfg.AllowedOrigins) > 0 {
		r.Use(cors.New(cors.Config{
			AllowOrigins:     cfg.AllowedOrigins,
			AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Authorization", "Content-Type"},
			AllowCredentials: true,
			MaxAge:           12 * time.Hour,
		}))
	}

	r.GET("/healthz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "database unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.NoRoute(func(c *gin.Context) { respond.Error(c, apperr.NotFound("route")) })

	api := r.Group("/api/v1")
	limited := middleware.RateLimit(cfg.AuthRatePerMin)
	api.POST("/auth/register", limited, h.Register)
	api.POST("/auth/login", limited, h.Login)
	api.POST("/auth/refresh", h.Refresh)
	api.POST("/auth/logout", h.Logout)

	authed := api.Group("", middleware.Auth(svc.Auth))
	authed.GET("/auth/me", h.Me)
	authed.GET("/orgs", h.ListOrgs)
	authed.POST("/orgs", h.CreateOrg)

	// Everything below is tenant-scoped: OrgScope rejects non-members before any handler runs.
	org := authed.Group("/orgs/:orgId", middleware.OrgScope(svc.Orgs))
	org.GET("", h.GetOrg)
	org.PATCH("", h.RenameOrg)
	org.DELETE("", h.DeleteOrg)
	org.GET("/dashboard", h.Dashboard)
	org.GET("/activity", h.OrgActivity)

	org.GET("/members", h.ListMembers)
	org.POST("/members", h.AddMember)
	org.PATCH("/members/:userId", h.UpdateMemberRole)
	org.DELETE("/members/:userId", h.RemoveMember)

	org.GET("/projects", h.ListProjects)
	org.POST("/projects", h.CreateProject)
	org.GET("/projects/:projectId", h.GetProject)
	org.PATCH("/projects/:projectId", h.UpdateProject)
	org.POST("/projects/:projectId/archive", h.ArchiveProject)
	org.PUT("/projects/:projectId/members/:userId", h.AddProjectMember)
	org.DELETE("/projects/:projectId/members/:userId", h.RemoveProjectMember)
	org.POST("/projects/:projectId/tasks", h.CreateTask)

	org.GET("/tasks", h.ListTasks)
	org.GET("/tasks/:taskId", h.GetTask)
	org.PATCH("/tasks/:taskId", h.UpdateTask)
	org.DELETE("/tasks/:taskId", h.DeleteTask)
	org.GET("/tasks/:taskId/comments", h.ListComments)
	org.POST("/tasks/:taskId/comments", h.AddComment)
	org.DELETE("/tasks/:taskId/comments/:commentId", h.DeleteComment)
	org.GET("/tasks/:taskId/activity", h.TaskHistory)
	return r
}

// ---- helpers shared by the handler files ----

// bind decodes a JSON body and rejects unknown fields and malformed JSON.
func bind(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		respond.Error(c, apperr.BadRequest("request body must be valid JSON with only known fields"))
		return false
	}
	return true
}

// pathID parses an object-id path parameter. A malformed id is reported as "not found",
// the same answer as a well-formed id that belongs to another tenant.
func pathID(c *gin.Context, param, what string) (bson.ObjectID, bool) {
	id, err := bson.ObjectIDFromHex(c.Param(param))
	if err != nil {
		respond.Error(c, apperr.NotFound(what))
		return bson.NilObjectID, false
	}
	return id, true
}

func pageQuery(c *gin.Context, defSize int) (model.PageQuery, bool) {
	p := model.PageQuery{Page: 1, Size: defSize}
	bad := map[string]string{}
	if v := c.Query("page"); v != "" {
		if n, err := strconv.Atoi(v); err != nil || n < 1 {
			bad["page"] = "must be a positive integer"
		} else {
			p.Page = n
		}
	}
	if v := c.Query("page_size"); v != "" {
		if n, err := strconv.Atoi(v); err != nil || n < 1 || n > 100 {
			bad["page_size"] = "must be between 1 and 100"
		} else {
			p.Size = n
		}
	}
	if len(bad) > 0 {
		respond.Error(c, apperr.Validation(bad))
		return p, false
	}
	return p, true
}

// csv reads a multi-value query parameter: ?status=a,b and ?status=a&status=b both work.
func csv(c *gin.Context, key string) []string {
	var out []string
	for _, raw := range c.QueryArray(key) {
		for _, s := range strings.Split(raw, ",") {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}
