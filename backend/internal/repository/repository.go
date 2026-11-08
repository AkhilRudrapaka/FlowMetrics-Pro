// Package repository is the only code that talks to MongoDB.
//
// TENANT ISOLATION RULE: every method on an org-owned collection (projects, tasks, comments,
// activities, memberships) takes an orgID and puts it in the filter via scoped(). There is no
// "find by id alone" method for those collections, so a handler cannot forget the tenant check.
package repository

import (
	"context"
	"errors"
	"time"

	"flowmetrics/internal/apperr"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Repos struct {
	Users    *UserRepo
	Orgs     *OrgRepo
	Members  *MembershipRepo
	Projects *ProjectRepo
	Tasks    *TaskRepo
	Comments *CommentRepo
	Activity *ActivityRepo
	Tokens   *RefreshTokenRepo
}

func New(db *mongo.Database) *Repos {
	return &Repos{
		Users:    &UserRepo{db.Collection("users")},
		Orgs:     &OrgRepo{db.Collection("organizations")},
		Members:  &MembershipRepo{db.Collection("memberships")},
		Projects: &ProjectRepo{db.Collection("projects")},
		Tasks:    &TaskRepo{db.Collection("tasks")},
		Comments: &CommentRepo{db.Collection("comments")},
		Activity: &ActivityRepo{db.Collection("activities")},
		Tokens:   &RefreshTokenRepo{db.Collection("refresh_tokens")},
	}
}

// scoped builds a filter that is always constrained to one tenant.
func scoped(orgID bson.ObjectID, extra bson.M) bson.M {
	f := bson.M{"org_id": orgID}
	for k, v := range extra {
		f[k] = v
	}
	return f
}

// now is truncated to milliseconds, the precision MongoDB stores, so values round-trip unchanged.
func now() time.Time { return time.Now().UTC().Truncate(time.Millisecond) }

// stamp fills in a new document's id and timestamps, keeping any the caller already set (the seeder
// back-dates demo data this way). updated may be nil for documents that are never modified.
func stamp(id *bson.ObjectID, created, updated *time.Time) {
	if id.IsZero() {
		*id = bson.NewObjectID()
	}
	if created.IsZero() {
		*created = now()
	}
	if updated != nil && updated.IsZero() {
		*updated = *created
	}
}

func notFound(err error, what string) error {
	if errors.Is(err, mongo.ErrNoDocuments) {
		return apperr.NotFound(what)
	}
	return err
}

func findAll[T any](ctx context.Context, c *mongo.Collection, filter any, opts ...options.Lister[options.FindOptions]) ([]T, error) {
	cur, err := c.Find(ctx, filter, opts...)
	if err != nil {
		return nil, err
	}
	out := []T{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// sortBy builds a Mongo sort from an already-allowlisted field and direction.
func sortBy(field string, desc bool) bson.D {
	dir := 1
	if desc {
		dir = -1
	}
	// _id as a tiebreaker keeps pagination stable when many rows share the sort value.
	return bson.D{{Key: field, Value: dir}, {Key: "_id", Value: dir}}
}
