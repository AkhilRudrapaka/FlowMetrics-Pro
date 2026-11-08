// Package service holds business rules and authorization. Handlers translate HTTP to these calls;
// repositories translate these calls to MongoDB. Permission checks live here, not in handlers,
// so they apply no matter which transport or test calls the service.
package service

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"flowmetrics/internal/apperr"
	"flowmetrics/internal/auth"
	"flowmetrics/internal/model"
	"flowmetrics/internal/permissions"
	"flowmetrics/internal/repository"
	"flowmetrics/internal/validate"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Actor is a verified identity inside one organization. The role is read from the
// membership document on every request, never from the JWT.
type Actor struct {
	UserID bson.ObjectID
	OrgID  bson.ObjectID
	Role   model.Role
}

func (a Actor) Can(p permissions.Permission) bool { return permissions.Can(a.Role, p) }

func (a Actor) require(p permissions.Permission) error {
	if !a.Can(p) {
		return apperr.Forbidden("your role does not allow this action")
	}
	return nil
}

type Services struct {
	Auth      *AuthService
	Orgs      *OrgService
	Projects  *ProjectService
	Tasks     *TaskService
	Dashboard *DashboardService
}

func New(r *repository.Repos, tm *auth.TokenManager, bcryptCost int, refreshTTL time.Duration) *Services {
	log := &activityLogger{r}
	return &Services{
		Auth:      newAuthService(r, tm, bcryptCost, refreshTTL),
		Orgs:      &OrgService{r, log},
		Projects:  &ProjectService{r, log},
		Tasks:     &TaskService{r, log},
		Dashboard: &DashboardService{r},
	}
}

// activityLogger writes audit entries. It is best-effort: a failed audit write is logged
// but never fails the user's action (MongoDB standalone has no multi-document transactions).
type activityLogger struct{ r *repository.Repos }

func (l *activityLogger) record(ctx context.Context, a Actor, entries ...model.Activity) {
	ctx = context.WithoutCancel(ctx) // a client disconnect must not drop the audit trail
	name := "Unknown user"
	if u, err := l.r.Users.FindByID(ctx, a.UserID); err == nil {
		name = u.Name
	}
	docs := make([]*model.Activity, len(entries))
	for i := range entries {
		e := entries[i]
		e.OrgID, e.ActorID, e.ActorName = a.OrgID, a.UserID, name
		docs[i] = &e
	}
	if err := l.r.Activity.Insert(ctx, docs...); err != nil {
		slog.Error("activity log write failed", "err", err, "org", a.OrgID.Hex())
	}
}

// ---- shared parsing helpers ----

// parseID records a validation error and returns false when s is not a valid object id.
func parseID(v validate.Errors, field, s string) (bson.ObjectID, bool) {
	id, err := bson.ObjectIDFromHex(s)
	if err != nil {
		v[field] = "must be a valid id"
		return bson.NilObjectID, false
	}
	return id, true
}

// parseSort turns "-created_at" into (field, desc), rejecting anything not in the allowlist.
func parseSort(v validate.Errors, raw, def string, allowed ...string) (string, bool) {
	if raw == "" {
		raw = def
	}
	desc := strings.HasPrefix(raw, "-")
	field := strings.TrimPrefix(raw, "-")
	for _, a := range allowed {
		if a == field {
			return field, desc
		}
	}
	v["sort"] = "must be one of: " + strings.Join(allowed, ", ") + " (prefix with - for descending)"
	return def, false
}

func ptr[T any](v T) *T { return &v }
