package repository

import (
	"context"
	"regexp"
	"time"

	"flowmetrics/internal/model"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type TaskRepo struct{ c *mongo.Collection }

type TaskFilter struct {
	ProjectID       *bson.ObjectID
	ExcludeProjects []bson.ObjectID // e.g. archived projects when listing org-wide
	Statuses        []string
	Priorities      []string
	AssigneeID      *bson.ObjectID
	Unassigned      bool
	Label           string
	Q               string
	OverdueBefore   *time.Time // due_date < t and not done
}

func (f TaskFilter) bson() bson.M {
	q := bson.M{}
	switch {
	case f.ProjectID != nil:
		q["project_id"] = *f.ProjectID
	case len(f.ExcludeProjects) > 0:
		q["project_id"] = bson.M{"$nin": f.ExcludeProjects}
	}
	if len(f.Statuses) > 0 {
		q["status"] = bson.M{"$in": f.Statuses}
	}
	if len(f.Priorities) > 0 {
		q["priority"] = bson.M{"$in": f.Priorities}
	}
	if f.AssigneeID != nil {
		q["assignee_id"] = *f.AssigneeID
	} else if f.Unassigned {
		q["assignee_id"] = nil
	}
	if f.Label != "" {
		q["labels"] = f.Label
	}
	if f.OverdueBefore != nil {
		q["due_date"] = bson.M{"$lt": *f.OverdueBefore}
		if _, ok := q["status"]; !ok {
			q["status"] = bson.M{"$ne": model.StatusDone}
		}
	}
	if f.Q != "" {
		re := bson.Regex{Pattern: regexp.QuoteMeta(f.Q), Options: "i"}
		q["$or"] = bson.A{bson.M{"title": re}, bson.M{"description": re}}
	}
	return q
}

func (r *TaskRepo) Create(ctx context.Context, t *model.Task) error {
	stamp(&t.ID, &t.CreatedAt, &t.UpdatedAt)
	_, err := r.c.InsertOne(ctx, t)
	return err
}

func (r *TaskRepo) Get(ctx context.Context, orgID, id bson.ObjectID) (*model.Task, error) {
	var t model.Task
	return &t, notFound(r.c.FindOne(ctx, scoped(orgID, bson.M{"_id": id})).Decode(&t), "task")
}

func (r *TaskRepo) List(ctx context.Context, orgID bson.ObjectID, f TaskFilter, sortField string, desc bool, page model.PageQuery) ([]model.Task, int64, error) {
	filter := scoped(orgID, f.bson())
	total, err := r.c.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	items, err := findAll[model.Task](ctx, r.c, filter,
		options.Find().SetSort(sortBy(sortField, desc)).SetSkip(page.Skip()).SetLimit(int64(page.Size)))
	return items, total, err
}

func (r *TaskRepo) Update(ctx context.Context, orgID, id bson.ObjectID, set bson.M) (*model.Task, error) {
	set["updated_at"] = now()
	var t model.Task
	err := r.c.FindOneAndUpdate(ctx, scoped(orgID, bson.M{"_id": id}), bson.M{"$set": set},
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&t)
	return &t, notFound(err, "task")
}

func (r *TaskRepo) Delete(ctx context.Context, orgID, id bson.ObjectID) error {
	res, err := r.c.DeleteOne(ctx, scoped(orgID, bson.M{"_id": id}))
	if err == nil && res.DeletedCount == 0 {
		return notFound(mongo.ErrNoDocuments, "task")
	}
	return err
}

// Unassign clears a user from tasks. projectID == nil means every project in the org.
func (r *TaskRepo) Unassign(ctx context.Context, orgID, userID bson.ObjectID, projectID *bson.ObjectID) error {
	q := bson.M{"assignee_id": userID}
	if projectID != nil {
		q["project_id"] = *projectID
	}
	_, err := r.c.UpdateMany(ctx, scoped(orgID, q), bson.M{"$set": bson.M{"assignee_id": nil, "updated_at": now()}})
	return err
}

type ProjectCounts struct{ Total, Done int64 }

func (r *TaskRepo) CountsByProject(ctx context.Context, orgID bson.ObjectID, projectIDs []bson.ObjectID) (map[bson.ObjectID]ProjectCounts, error) {
	cur, err := r.c.Aggregate(ctx, []bson.M{
		{"$match": scoped(orgID, bson.M{"project_id": bson.M{"$in": projectIDs}})},
		{"$group": bson.M{
			"_id":   "$project_id",
			"total": bson.M{"$sum": 1},
			"done":  bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$status", model.StatusDone}}, 1, 0}}},
		}},
	})
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ID    bson.ObjectID `bson:"_id"`
		Total int64         `bson:"total"`
		Done  int64         `bson:"done"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	out := make(map[bson.ObjectID]ProjectCounts, len(rows))
	for _, row := range rows {
		out[row.ID] = ProjectCounts{row.Total, row.Done}
	}
	return out, nil
}

type TaskStats struct {
	ByStatus   map[string]int64
	ByPriority map[string]int64
	Overdue    int64
	Mine       int64 // open tasks assigned to the asking user
}

// Stats computes every dashboard counter in a single pass over the org's tasks.
func (r *TaskRepo) Stats(ctx context.Context, orgID, userID bson.ObjectID, excludeProjects []bson.ObjectID, today time.Time) (*TaskStats, error) {
	match := bson.M{}
	if len(excludeProjects) > 0 {
		match["project_id"] = bson.M{"$nin": excludeProjects}
	}
	notDone := bson.M{"$ne": model.StatusDone}
	group := func(field string) bson.A {
		return bson.A{bson.M{"$group": bson.M{"_id": "$" + field, "n": bson.M{"$sum": 1}}}}
	}
	cur, err := r.c.Aggregate(ctx, []bson.M{
		{"$match": scoped(orgID, match)},
		{"$facet": bson.M{
			"by_status":   group("status"),
			"by_priority": group("priority"),
			"overdue":     bson.A{bson.M{"$match": bson.M{"status": notDone, "due_date": bson.M{"$lt": today}}}, bson.M{"$count": "n"}},
			"mine":        bson.A{bson.M{"$match": bson.M{"status": notDone, "assignee_id": userID}}, bson.M{"$count": "n"}},
		}},
	})
	if err != nil {
		return nil, err
	}
	type kv struct {
		ID string `bson:"_id"`
		N  int64  `bson:"n"`
	}
	type count struct {
		N int64 `bson:"n"`
	}
	var rows []struct {
		ByStatus   []kv    `bson:"by_status"`
		ByPriority []kv    `bson:"by_priority"`
		Overdue    []count `bson:"overdue"`
		Mine       []count `bson:"mine"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	out := &TaskStats{ByStatus: map[string]int64{}, ByPriority: map[string]int64{}}
	if len(rows) == 0 {
		return out, nil
	}
	for _, x := range rows[0].ByStatus {
		out.ByStatus[x.ID] = x.N
	}
	for _, x := range rows[0].ByPriority {
		out.ByPriority[x.ID] = x.N
	}
	if len(rows[0].Overdue) > 0 {
		out.Overdue = rows[0].Overdue[0].N
	}
	if len(rows[0].Mine) > 0 {
		out.Mine = rows[0].Mine[0].N
	}
	return out, nil
}

func (r *TaskRepo) DeleteByOrg(ctx context.Context, orgID bson.ObjectID) error {
	_, err := r.c.DeleteMany(ctx, scoped(orgID, nil))
	return err
}
