package repository

import (
	"context"

	"flowmetrics/internal/model"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ---- comments ----

type CommentRepo struct{ c *mongo.Collection }

func (r *CommentRepo) Create(ctx context.Context, cm *model.Comment) error {
	stamp(&cm.ID, &cm.CreatedAt, nil)
	_, err := r.c.InsertOne(ctx, cm)
	return err
}

func (r *CommentRepo) Get(ctx context.Context, orgID, taskID, id bson.ObjectID) (*model.Comment, error) {
	var cm model.Comment
	err := r.c.FindOne(ctx, scoped(orgID, bson.M{"_id": id, "task_id": taskID})).Decode(&cm)
	return &cm, notFound(err, "comment")
}

// List returns the oldest comments first, the natural reading order of a thread.
func (r *CommentRepo) List(ctx context.Context, orgID, taskID bson.ObjectID, page model.PageQuery) ([]model.Comment, int64, error) {
	filter := scoped(orgID, bson.M{"task_id": taskID})
	total, err := r.c.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	items, err := findAll[model.Comment](ctx, r.c, filter,
		options.Find().SetSort(sortBy("created_at", false)).SetSkip(page.Skip()).SetLimit(int64(page.Size)))
	return items, total, err
}

func (r *CommentRepo) Delete(ctx context.Context, orgID, id bson.ObjectID) error {
	_, err := r.c.DeleteOne(ctx, scoped(orgID, bson.M{"_id": id}))
	return err
}

func (r *CommentRepo) DeleteByTask(ctx context.Context, orgID, taskID bson.ObjectID) error {
	_, err := r.c.DeleteMany(ctx, scoped(orgID, bson.M{"task_id": taskID}))
	return err
}

func (r *CommentRepo) DeleteByOrg(ctx context.Context, orgID bson.ObjectID) error {
	_, err := r.c.DeleteMany(ctx, scoped(orgID, nil))
	return err
}

// ---- activity ----

type ActivityRepo struct{ c *mongo.Collection }

func (r *ActivityRepo) Insert(ctx context.Context, entries ...*model.Activity) error {
	if len(entries) == 0 {
		return nil
	}
	docs := make([]any, len(entries))
	for i, a := range entries {
		stamp(&a.ID, &a.CreatedAt, nil)
		docs[i] = a
	}
	_, err := r.c.InsertMany(ctx, docs)
	return err
}

// List returns newest first; taskID == nil returns the whole org feed.
func (r *ActivityRepo) List(ctx context.Context, orgID bson.ObjectID, taskID *bson.ObjectID, page model.PageQuery) ([]model.Activity, int64, error) {
	q := bson.M{}
	if taskID != nil {
		q["task_id"] = *taskID
	}
	filter := scoped(orgID, q)
	total, err := r.c.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	items, err := findAll[model.Activity](ctx, r.c, filter,
		options.Find().SetSort(sortBy("created_at", true)).SetSkip(page.Skip()).SetLimit(int64(page.Size)))
	return items, total, err
}

func (r *ActivityRepo) DeleteByOrg(ctx context.Context, orgID bson.ObjectID) error {
	_, err := r.c.DeleteMany(ctx, scoped(orgID, nil))
	return err
}
