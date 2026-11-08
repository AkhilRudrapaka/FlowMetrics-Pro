package repository

import (
	"context"
	"regexp"

	"flowmetrics/internal/model"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type ProjectRepo struct{ c *mongo.Collection }

type ProjectFilter struct {
	Q      string
	Status string // empty = every status except archived
}

func (r *ProjectRepo) Create(ctx context.Context, p *model.Project) error {
	stamp(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	_, err := r.c.InsertOne(ctx, p)
	return err
}

func (r *ProjectRepo) Get(ctx context.Context, orgID, id bson.ObjectID) (*model.Project, error) {
	var p model.Project
	return &p, notFound(r.c.FindOne(ctx, scoped(orgID, bson.M{"_id": id})).Decode(&p), "project")
}

func (r *ProjectRepo) FindByIDs(ctx context.Context, orgID bson.ObjectID, ids []bson.ObjectID) ([]model.Project, error) {
	return findAll[model.Project](ctx, r.c, scoped(orgID, bson.M{"_id": bson.M{"$in": ids}}))
}

func (r *ProjectRepo) List(ctx context.Context, orgID bson.ObjectID, f ProjectFilter, sortField string, desc bool, page model.PageQuery) ([]model.Project, int64, error) {
	q := bson.M{}
	if f.Status != "" {
		q["status"] = f.Status
	} else {
		q["status"] = bson.M{"$ne": model.ProjectArchived}
	}
	if f.Q != "" {
		re := bson.Regex{Pattern: regexp.QuoteMeta(f.Q), Options: "i"} // QuoteMeta: user input is a literal, never a pattern
		q["$or"] = bson.A{bson.M{"name": re}, bson.M{"description": re}}
	}
	filter := scoped(orgID, q)
	total, err := r.c.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	items, err := findAll[model.Project](ctx, r.c, filter,
		options.Find().SetSort(sortBy(sortField, desc)).SetSkip(page.Skip()).SetLimit(int64(page.Size)))
	return items, total, err
}

func (r *ProjectRepo) update(ctx context.Context, orgID, id bson.ObjectID, update bson.M) (*model.Project, error) {
	var p model.Project
	err := r.c.FindOneAndUpdate(ctx, scoped(orgID, bson.M{"_id": id}), update,
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&p)
	return &p, notFound(err, "project")
}

func (r *ProjectRepo) Update(ctx context.Context, orgID, id bson.ObjectID, set bson.M) (*model.Project, error) {
	set["updated_at"] = now()
	return r.update(ctx, orgID, id, bson.M{"$set": set})
}

func (r *ProjectRepo) AddMember(ctx context.Context, orgID, id, userID bson.ObjectID) (*model.Project, error) {
	return r.update(ctx, orgID, id, bson.M{"$addToSet": bson.M{"member_ids": userID}, "$set": bson.M{"updated_at": now()}})
}

func (r *ProjectRepo) RemoveMember(ctx context.Context, orgID, id, userID bson.ObjectID) (*model.Project, error) {
	return r.update(ctx, orgID, id, bson.M{"$pull": bson.M{"member_ids": userID}, "$set": bson.M{"updated_at": now()}})
}

// RemoveUserFromAll runs when someone leaves or is removed from the organization.
func (r *ProjectRepo) RemoveUserFromAll(ctx context.Context, orgID, userID bson.ObjectID) error {
	_, err := r.c.UpdateMany(ctx, scoped(orgID, bson.M{"member_ids": userID}), bson.M{"$pull": bson.M{"member_ids": userID}})
	return err
}

func (r *ProjectRepo) ArchivedIDs(ctx context.Context, orgID bson.ObjectID) ([]bson.ObjectID, error) {
	ids := []bson.ObjectID{}
	cur, err := r.c.Find(ctx, scoped(orgID, bson.M{"status": model.ProjectArchived}), options.Find().SetProjection(bson.M{"_id": 1}))
	if err != nil {
		return nil, err
	}
	var docs []struct {
		ID bson.ObjectID `bson:"_id"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	for _, d := range docs {
		ids = append(ids, d.ID)
	}
	return ids, nil
}

func (r *ProjectRepo) Count(ctx context.Context, orgID bson.ObjectID, extra bson.M) (int64, error) {
	return r.c.CountDocuments(ctx, scoped(orgID, extra))
}

func (r *ProjectRepo) DeleteByOrg(ctx context.Context, orgID bson.ObjectID) error {
	_, err := r.c.DeleteMany(ctx, scoped(orgID, nil))
	return err
}
