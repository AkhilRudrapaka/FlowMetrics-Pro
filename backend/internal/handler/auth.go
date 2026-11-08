package handler

import (
	"net/http"
	"time"

	"flowmetrics/internal/middleware"
	"flowmetrics/internal/model"
	"flowmetrics/internal/respond"
	"flowmetrics/internal/service"

	"github.com/gin-gonic/gin"
)

const refreshCookie = "fm_refresh"

type sessionResponse struct {
	User        *model.User `json:"user"`
	AccessToken string      `json:"access_token"`
	ExpiresAt   time.Time   `json:"expires_at"`
}

// The access token travels in the JSON body and lives only in browser memory.
// The refresh token travels only in an httpOnly, SameSite=Strict cookie scoped to /api/v1/auth,
// so page scripts (and therefore XSS) can never read it and other routes never receive it.
func (h *Handler) writeSession(c *gin.Context, status int, s *service.Session) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: refreshCookie, Value: s.RefreshToken, Path: "/api/v1/auth",
		Expires: s.RefreshExpires, MaxAge: int(time.Until(s.RefreshExpires).Seconds()),
		HttpOnly: true, Secure: h.cfg.CookieSecure, SameSite: http.SameSiteStrictMode,
	})
	respond.JSON(c, status, sessionResponse{s.User, s.AccessToken, s.AccessExpires})
}

func (h *Handler) clearCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: refreshCookie, Path: "/api/v1/auth", MaxAge: -1,
		HttpOnly: true, Secure: h.cfg.CookieSecure, SameSite: http.SameSiteStrictMode,
	})
}

func (h *Handler) Register(c *gin.Context) {
	var in struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !bind(c, &in) {
		return
	}
	s, err := h.svc.Auth.Register(c.Request.Context(), service.RegisterInput(in))
	if err != nil {
		respond.Error(c, err)
		return
	}
	h.writeSession(c, http.StatusCreated, s)
}

func (h *Handler) Login(c *gin.Context) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !bind(c, &in) {
		return
	}
	s, err := h.svc.Auth.Login(c.Request.Context(), in.Email, in.Password)
	if err != nil {
		respond.Error(c, err)
		return
	}
	h.writeSession(c, http.StatusOK, s)
}

func (h *Handler) Refresh(c *gin.Context) {
	raw, _ := c.Cookie(refreshCookie)
	s, err := h.svc.Auth.Refresh(c.Request.Context(), raw)
	if err != nil {
		h.clearCookie(c)
		respond.Error(c, err)
		return
	}
	h.writeSession(c, http.StatusOK, s)
}

func (h *Handler) Logout(c *gin.Context) {
	raw, _ := c.Cookie(refreshCookie)
	if err := h.svc.Auth.Logout(c.Request.Context(), raw); err != nil {
		respond.Error(c, err)
		return
	}
	h.clearCookie(c)
	respond.NoContent(c)
}

func (h *Handler) Me(c *gin.Context) {
	u, err := h.svc.Auth.Me(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		respond.Error(c, err)
		return
	}
	respond.JSON(c, http.StatusOK, u)
}
