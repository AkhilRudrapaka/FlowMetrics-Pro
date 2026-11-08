// Package middleware holds the Gin middleware: the auth gate, the tenancy gate, and hardening.
package middleware

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"flowmetrics/internal/apperr"
	"flowmetrics/internal/respond"
	"flowmetrics/internal/service"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/time/rate"
)

const (
	userKey  = "fm.user_id"
	actorKey = "fm.actor"
)

// Auth requires a valid access token and records the user id. It deliberately knows nothing about orgs.
func Auth(a *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		tok, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
		if !ok || tok == "" {
			respond.Error(c, apperr.Unauthorized("missing bearer token"))
			return
		}
		uid, err := a.ParseAccessToken(tok)
		if err != nil {
			respond.Error(c, apperr.Unauthorized("invalid or expired access token"))
			return
		}
		c.Set(userKey, uid)
		c.Next()
	}
}

// OrgScope is the tenancy gate for every /orgs/:orgId/... route. It loads the caller's membership
// from the database; no handler below it can run for a non-member. Everything downstream receives
// the resulting Actor and nothing else about the org, so there is no way to act on another tenant.
func OrgScope(o *service.OrgService) gin.HandlerFunc {
	return func(c *gin.Context) {
		orgID, err := bson.ObjectIDFromHex(c.Param("orgId"))
		if err != nil {
			respond.Error(c, apperr.NotFound("organization"))
			return
		}
		actor, err := o.Resolve(c.Request.Context(), UserID(c), orgID)
		if err != nil {
			respond.Error(c, err)
			return
		}
		c.Set(actorKey, *actor)
		c.Next()
	}
}

func UserID(c *gin.Context) bson.ObjectID { return c.MustGet(userKey).(bson.ObjectID) }
func Actor(c *gin.Context) service.Actor  { return c.MustGet(actorKey).(service.Actor) }

// RequestLog emits one structured line per request.
func RequestLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		slog.Info("request", "method", c.Request.Method, "route", c.FullPath(), "status", c.Writer.Status(),
			"ms", time.Since(start).Milliseconds(), "ip", c.ClientIP())
	}
}

func Recover() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		slog.Error("panic recovered", "panic", recovered, "route", c.FullPath())
		respond.Error(c, apperr.New(http.StatusInternalServerError, "internal_error", "something went wrong"))
	})
}

func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-store") // API responses are per-user; never cache them in shared caches
		c.Next()
	}
}

// BodyLimit caps request bodies so a single request cannot exhaust memory.
func BodyLimit(n int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, n)
		c.Next()
	}
}

// RateLimit is a per-client-IP token bucket kept in memory.
// ponytail: single-process only; use a shared store (e.g. Redis) when running several replicas.
func RateLimit(perMinute int) gin.HandlerFunc {
	if perMinute <= 0 {
		return func(c *gin.Context) { c.Next() }
	}
	type entry struct {
		lim  *rate.Limiter
		seen time.Time
	}
	var (
		mu        sync.Mutex
		clients   = map[string]*entry{}
		lastPurge = time.Now()
	)
	return func(c *gin.Context) {
		ip := c.ClientIP()
		mu.Lock()
		if time.Since(lastPurge) > 10*time.Minute {
			for k, e := range clients {
				if time.Since(e.seen) > 10*time.Minute {
					delete(clients, k)
				}
			}
			lastPurge = time.Now()
		}
		e, ok := clients[ip]
		if !ok {
			e = &entry{lim: rate.NewLimiter(rate.Limit(float64(perMinute)/60), max(5, perMinute/2))}
			clients[ip] = e
		}
		e.seen = time.Now()
		allowed := e.lim.Allow()
		mu.Unlock()
		if !allowed {
			c.Header("Retry-After", strconv.Itoa(60/perMinute+1))
			respond.Error(c, apperr.New(http.StatusTooManyRequests, "rate_limited", "too many attempts, please try again shortly"))
			return
		}
		c.Next()
	}
}
