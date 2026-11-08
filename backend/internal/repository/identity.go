package repository

import (
	"context"

	"flowmetrics/internal/apperr"
	"flowmetrics/internal/model"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ---- users (global, not tenant-owned) ----

type UserRepo struct{ c *mongo.Collection }

func (r *UserRepo) Create(ctx context.Context, u *model.User) error {
	u.ID, u.CreatedAt = bson.NewObjectID(), now()
	if _, err := r.c.InsertOne(ctx, u); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return apperr.Conflict("an account with this email already exists")
		}
		return err
	}
	return nil
}

func (r *UserRepo) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	var u model.User
	return &u, notFound(r.c.FindOne(ctx, bson.M{"email": email}).Decode(&u), "user")
}

func (r *UserRepo) FindByID(ctx context.Context, id bson.ObjectID) (*model.User, error) {
	var u model.User
	return &u, notFound(r.c.FindOne(ctx, bson.M{"_id": id}).Decode(&u), "user")
}

func (r *UserRepo) FindByIDs(ctx context.Context, ids []bson.ObjectID) ([]model.User, error) {
	return findAll[model.User](ctx, r.c, bson.M{"_id": bson.M{"$in": ids}})
}

// ---- organizations ----

type OrgRepo struct{ c *mongo.Collection }

func (r *OrgRepo) Create(ctx context.Context, o *model.Organization) error {
	o.ID, o.CreatedAt = bson.NewObjectID(), now()
	_, err := r.c.InsertOne(ctx, o)
	return err
}

// FindByIDs is used only with ids taken from the caller's own memberships.
func (r *OrgRepo) FindByIDs(ctx context.Context, ids []bson.ObjectID) ([]model.Organization, error) {
	return findAll[model.Organization](ctx, r.c, bson.M{"_id": bson.M{"$in": ids}}, options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
}

func (r *OrgRepo) FindByID(ctx context.Context, id bson.ObjectID) (*model.Organization, error) {
	var o model.Organization
	return &o, notFound(r.c.FindOne(ctx, bson.M{"_id": id}).Decode(&o), "organization")
}

func (r *OrgRepo) Rename(ctx context.Context, id bson.ObjectID, name string) (*model.Organization, error) {
	var o model.Organization
	err := r.c.FindOneAndUpdate(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"name": name}},
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&o)
	return &o, notFound(err, "organization")
}

func (r *OrgRepo) Delete(ctx context.Context, id bson.ObjectID) error {
	_, err := r.c.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

// ---- memberships ----

type MembershipRepo struct{ c *mongo.Collection }

func (r *MembershipRepo) Create(ctx context.Context, m *model.Membership) error {
	m.ID, m.CreatedAt = bson.NewObjectID(), now()
	if _, err := r.c.InsertOne(ctx, m); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return apperr.Conflict("user is already a member of this organization")
		}
		return err
	}
	return nil
}

// Find is the tenancy gate: it answers "is this user in this org, and with what role?".
func (r *MembershipRepo) Find(ctx context.Context, orgID, userID bson.ObjectID) (*model.Membership, error) {
	var m model.Membership
	err := r.c.FindOne(ctx, scoped(orgID, bson.M{"user_id": userID})).Decode(&m)
	return &m, notFound(err, "membership")
}

func (r *MembershipRepo) ListByOrg(ctx context.Context, orgID bson.ObjectID) ([]model.Membership, error) {
	return findAll[model.Membership](ctx, r.c, scoped(orgID, nil), options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}))
}

func (r *MembershipRepo) ListByUser(ctx context.Context, userID bson.ObjectID) ([]model.Membership, error) {
	return findAll[model.Membership](ctx, r.c, bson.M{"user_id": userID})
}

func (r *MembershipRepo) UpdateRole(ctx context.Context, orgID, userID bson.ObjectID, role model.Role) error {
	res, err := r.c.UpdateOne(ctx, scoped(orgID, bson.M{"user_id": userID}), bson.M{"$set": bson.M{"role": role}})
	if err == nil && res.MatchedCount == 0 {
		return apperr.NotFound("member")
	}
	return err
}

func (r *MembershipRepo) Delete(ctx context.Context, orgID, userID bson.ObjectID) error {
	res, err := r.c.DeleteOne(ctx, scoped(orgID, bson.M{"user_id": userID}))
	if err == nil && res.DeletedCount == 0 {
		return apperr.NotFound("member")
	}
	return err
}

func (r *MembershipRepo) DeleteByOrg(ctx context.Context, orgID bson.ObjectID) error {
	_, err := r.c.DeleteMany(ctx, scoped(orgID, nil))
	return err
}

// ---- refresh tokens ----

type RefreshTokenRepo struct{ c *mongo.Collection }

func (r *RefreshTokenRepo) Create(ctx context.Context, t *model.RefreshToken) error {
	t.ID, t.CreatedAt = bson.NewObjectID(), now()
	_, err := r.c.InsertOne(ctx, t)
	return err
}

func (r *RefreshTokenRepo) FindByHash(ctx context.Context, hash string) (*model.RefreshToken, error) {
	var t model.RefreshToken
	return &t, notFound(r.c.FindOne(ctx, bson.M{"token_hash": hash}).Decode(&t), "session")
}

// Revoke atomically flips a live token to revoked. It reports false if someone else got there first,
// which is how concurrent use of the same refresh token is detected.
func (r *RefreshTokenRepo) Revoke(ctx context.Context, id bson.ObjectID) (bool, error) {
	res, err := r.c.UpdateOne(ctx, bson.M{"_id": id, "revoked_at": nil}, bson.M{"$set": bson.M{"revoked_at": now()}})
	if err != nil {
		return false, err
	}
	return res.ModifiedCount == 1, nil
}

func (r *RefreshTokenRepo) RevokeFamily(ctx context.Context, familyID bson.ObjectID) error {
	_, err := r.c.UpdateMany(ctx, bson.M{"family_id": familyID, "revoked_at": nil}, bson.M{"$set": bson.M{"revoked_at": now()}})
	return err
}
