// Package database owns the MongoDB connection and index definitions.
package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Open connects and prepares the database (indexes), retrying while MongoDB is still starting up.
// Under docker compose the container can report "started" several seconds before it accepts
// authenticated connections, so failing on the first attempt would make the API crash-loop.
func Open(ctx context.Context, uri, dbName string) (*mongo.Client, *mongo.Database, error) {
	const attempts, wait = 12, 2 * time.Second
	var lastErr error
	for i := 1; i <= attempts; i++ {
		client, err := Connect(ctx, uri)
		if err == nil {
			db := client.Database(dbName)
			if err = EnsureIndexes(ctx, db); err == nil {
				return client, db, nil
			}
			_ = client.Disconnect(context.Background())
		}
		lastErr = err
		slog.Warn("database not ready yet", "attempt", i, "of", attempts, "err", err)
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		}
	}
	return nil, nil, fmt.Errorf("database unavailable after %d attempts: %w", attempts, lastErr)
}

func Connect(ctx context.Context, uri string) (*mongo.Client, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri).SetServerSelectionTimeout(5 * time.Second))
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("ping mongodb: %w", err)
	}
	return client, nil
}

// EnsureIndexes is idempotent and runs on every start. Every tenant-scoped index leads with org_id,
// so isolation is also what makes the queries fast.
func EnsureIndexes(ctx context.Context, db *mongo.Database) error {
	idx := func(opts *options.IndexOptionsBuilder, keys ...any) mongo.IndexModel {
		k := bson.D{}
		for i := 0; i < len(keys); i += 2 {
			k = append(k, bson.E{Key: keys[i].(string), Value: keys[i+1]})
		}
		return mongo.IndexModel{Keys: k, Options: opts}
	}
	unique := func() *options.IndexOptionsBuilder { return options.Index().SetUnique(true) }

	plan := map[string][]mongo.IndexModel{
		"users":       {idx(unique(), "email", 1)},
		"memberships": {idx(unique(), "org_id", 1, "user_id", 1), idx(nil, "user_id", 1)},
		"projects":    {idx(nil, "org_id", 1, "status", 1, "updated_at", -1)},
		"tasks": {
			idx(nil, "org_id", 1, "project_id", 1, "status", 1),
			idx(nil, "org_id", 1, "assignee_id", 1, "status", 1),
			idx(nil, "org_id", 1, "due_date", 1),
			idx(nil, "org_id", 1, "updated_at", -1),
		},
		"comments":   {idx(nil, "org_id", 1, "task_id", 1, "created_at", 1)},
		"activities": {idx(nil, "org_id", 1, "created_at", -1), idx(nil, "org_id", 1, "task_id", 1, "created_at", -1)},
		"refresh_tokens": {
			idx(unique(), "token_hash", 1),
			idx(nil, "family_id", 1),
			idx(options.Index().SetExpireAfterSeconds(0), "expires_at", 1), // TTL: Mongo purges expired sessions
		},
	}
	for coll, models := range plan {
		if _, err := db.Collection(coll).Indexes().CreateMany(ctx, models); err != nil {
			return fmt.Errorf("indexes for %s: %w", coll, err)
		}
	}
	return nil
}
