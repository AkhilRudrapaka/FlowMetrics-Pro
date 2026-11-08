// Package model holds the persisted domain types. Every type that belongs to a tenant
// carries an OrgID; repositories refuse to query those collections without one.
package model

import (
	"encoding/json"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

func (r Role) Valid() bool { return r == RoleOwner || r == RoleAdmin || r == RoleMember }

// Task statuses, in Kanban column order.
const (
	StatusBacklog    = "backlog"
	StatusTodo       = "todo"
	StatusInProgress = "in_progress"
	StatusInReview   = "in_review"
	StatusDone       = "done"
	StatusBlocked    = "blocked"
)

var TaskStatuses = []string{StatusBacklog, StatusTodo, StatusInProgress, StatusInReview, StatusDone, StatusBlocked}

// Statuses that count as "active work" on the dashboard.
var ActiveTaskStatuses = []string{StatusTodo, StatusInProgress, StatusInReview, StatusBlocked}

var Priorities = []string{"low", "medium", "high", "critical"}

const (
	ProjectActive    = "active"
	ProjectOnHold    = "on_hold"
	ProjectCompleted = "completed"
	ProjectArchived  = "archived"
)

var ProjectStatuses = []string{ProjectActive, ProjectOnHold, ProjectCompleted, ProjectArchived}

type User struct {
	ID           bson.ObjectID `bson:"_id" json:"id"`
	Name         string        `bson:"name" json:"name"`
	Email        string        `bson:"email" json:"email"`
	PasswordHash string        `bson:"password_hash" json:"-"`
	CreatedAt    time.Time     `bson:"created_at" json:"created_at"`
}

type Organization struct {
	ID        bson.ObjectID `bson:"_id" json:"id"`
	Name      string        `bson:"name" json:"name"`
	OwnerID   bson.ObjectID `bson:"owner_id" json:"owner_id"`
	CreatedAt time.Time     `bson:"created_at" json:"created_at"`
}

type Membership struct {
	ID        bson.ObjectID `bson:"_id" json:"id"`
	OrgID     bson.ObjectID `bson:"org_id" json:"org_id"`
	UserID    bson.ObjectID `bson:"user_id" json:"user_id"`
	Role      Role          `bson:"role" json:"role"`
	CreatedAt time.Time     `bson:"created_at" json:"created_at"`
}

type Project struct {
	ID          bson.ObjectID   `bson:"_id" json:"id"`
	OrgID       bson.ObjectID   `bson:"org_id" json:"org_id"`
	Name        string          `bson:"name" json:"name"`
	Description string          `bson:"description" json:"description"`
	Status      string          `bson:"status" json:"status"`
	MemberIDs   []bson.ObjectID `bson:"member_ids" json:"member_ids"`
	CreatedBy   bson.ObjectID   `bson:"created_by" json:"created_by"`
	CreatedAt   time.Time       `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time       `bson:"updated_at" json:"updated_at"`
	ArchivedAt  *time.Time      `bson:"archived_at" json:"archived_at"`
}

func (p *Project) HasMember(id bson.ObjectID) bool {
	for _, m := range p.MemberIDs {
		if m == id {
			return true
		}
	}
	return false
}

type Task struct {
	ID          bson.ObjectID  `bson:"_id" json:"id"`
	OrgID       bson.ObjectID  `bson:"org_id" json:"org_id"`
	ProjectID   bson.ObjectID  `bson:"project_id" json:"project_id"`
	Title       string         `bson:"title" json:"title"`
	Description string         `bson:"description" json:"description"`
	Status      string         `bson:"status" json:"status"`
	Priority    string         `bson:"priority" json:"priority"`
	AssigneeID  *bson.ObjectID `bson:"assignee_id" json:"assignee_id"`
	ReporterID  bson.ObjectID  `bson:"reporter_id" json:"reporter_id"`
	DueDate     *time.Time     `bson:"due_date" json:"due_date"`
	Labels      []string       `bson:"labels" json:"labels"`
	CompletedAt *time.Time     `bson:"completed_at" json:"completed_at"`
	CreatedAt   time.Time      `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time      `bson:"updated_at" json:"updated_at"`
}

type Comment struct {
	ID         bson.ObjectID `bson:"_id" json:"id"`
	OrgID      bson.ObjectID `bson:"org_id" json:"org_id"`
	TaskID     bson.ObjectID `bson:"task_id" json:"task_id"`
	AuthorID   bson.ObjectID `bson:"author_id" json:"author_id"`
	AuthorName string        `bson:"author_name" json:"author_name"` // snapshot: survives member removal
	Body       string        `bson:"body" json:"body"`
	CreatedAt  time.Time     `bson:"created_at" json:"created_at"`
}

// Activity is an append-only audit entry. Names and titles are snapshots so the
// feed still reads correctly after the underlying entity is renamed or removed.
type Activity struct {
	ID          bson.ObjectID  `bson:"_id" json:"id"`
	OrgID       bson.ObjectID  `bson:"org_id" json:"org_id"`
	ProjectID   *bson.ObjectID `bson:"project_id" json:"project_id"`
	TaskID      *bson.ObjectID `bson:"task_id" json:"task_id"`
	ActorID     bson.ObjectID  `bson:"actor_id" json:"actor_id"`
	ActorName   string         `bson:"actor_name" json:"actor_name"`
	Action      string         `bson:"action" json:"action"`
	EntityTitle string         `bson:"entity_title" json:"entity_title"`
	Meta        map[string]any `bson:"meta,omitempty" json:"meta,omitempty"`
	CreatedAt   time.Time      `bson:"created_at" json:"created_at"`
}

type RefreshToken struct {
	ID        bson.ObjectID `bson:"_id"`
	UserID    bson.ObjectID `bson:"user_id"`
	FamilyID  bson.ObjectID `bson:"family_id"`
	TokenHash string        `bson:"token_hash"`
	ExpiresAt time.Time     `bson:"expires_at"`
	RevokedAt *time.Time    `bson:"revoked_at"`
	CreatedAt time.Time     `bson:"created_at"`
}

// PageQuery is a validated page request.
type PageQuery struct{ Page, Size int }

func (p PageQuery) Skip() int64 { return int64((p.Page - 1) * p.Size) }

type PageMeta struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

func NewPageMeta(p PageQuery, total int64) PageMeta {
	return PageMeta{Page: p.Page, PageSize: p.Size, Total: total, TotalPages: int((total + int64(p.Size) - 1) / int64(p.Size))}
}

// Nullable distinguishes "field absent" (leave unchanged) from "field is null" (clear it) in PATCH bodies.
type Nullable[T any] struct {
	Set   bool
	Value *T
}

func (n *Nullable[T]) UnmarshalJSON(b []byte) error {
	n.Set = true
	if string(b) == "null" {
		n.Value = nil
		return nil
	}
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	n.Value = &v
	return nil
}
