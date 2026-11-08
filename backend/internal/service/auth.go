package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"flowmetrics/internal/apperr"
	"flowmetrics/internal/auth"
	"flowmetrics/internal/model"
	"flowmetrics/internal/repository"
	"flowmetrics/internal/validate"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// A refresh token that was rotated moments ago may legitimately be presented again
// (two browser tabs refreshing together). Beyond this window, reuse means theft.
const reuseLeeway = 10 * time.Second

type AuthService struct {
	users      *repository.UserRepo
	tokens     *repository.RefreshTokenRepo
	tm         *auth.TokenManager
	cost       int
	refreshTTL time.Duration
	dummyHash  string // compared against when the email is unknown, so timing does not reveal which emails exist
}

func newAuthService(r *repository.Repos, tm *auth.TokenManager, cost int, refreshTTL time.Duration) *AuthService {
	dummy, _ := auth.HashPassword("not-a-real-password", cost)
	return &AuthService{r.Users, r.Tokens, tm, cost, refreshTTL, dummy}
}

type Session struct {
	User           *model.User
	AccessToken    string
	AccessExpires  time.Time
	RefreshToken   string
	RefreshExpires time.Time
}

type RegisterInput struct{ Name, Email, Password string }

func (s *AuthService) Register(ctx context.Context, in RegisterInput) (*Session, error) {
	v := validate.Errors{}
	name := v.Text("name", in.Name, 1, 80)
	email := v.Email("email", in.Email)
	switch n := len(in.Password); {
	case n < 8:
		v["password"] = "must be at least 8 characters"
	case n > auth.MaxPasswordBytes:
		v["password"] = "must be at most 72 bytes"
	}
	if err := v.Err(); err != nil {
		return nil, err
	}
	hash, err := auth.HashPassword(in.Password, s.cost)
	if err != nil {
		return nil, err
	}
	u := &model.User{Name: name, Email: email, PasswordHash: hash}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, err
	}
	return s.newSession(ctx, u, bson.NewObjectID())
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*Session, error) {
	u, err := s.users.FindByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	hash := s.dummyHash
	switch {
	case err == nil:
		hash = u.PasswordHash
	case !isNotFound(err):
		return nil, err
	}
	// Always run bcrypt, and always return the same error, whether the email or the password was wrong.
	if !auth.CheckPassword(hash, password) || err != nil {
		return nil, apperr.Unauthorized("invalid email or password")
	}
	return s.newSession(ctx, u, bson.NewObjectID())
}

// Refresh rotates the refresh token: the presented one is spent, a new one is issued in the same family.
// Presenting a spent token again revokes the whole family (the session is assumed stolen).
func (s *AuthService) Refresh(ctx context.Context, raw string) (*Session, error) {
	invalid := apperr.Unauthorized("session is invalid or has expired")
	if raw == "" {
		return nil, invalid
	}
	rt, err := s.tokens.FindByHash(ctx, auth.HashToken(raw))
	if err != nil {
		if isNotFound(err) {
			return nil, invalid
		}
		return nil, err
	}
	if rt.RevokedAt != nil {
		if time.Since(*rt.RevokedAt) > reuseLeeway {
			_ = s.tokens.RevokeFamily(ctx, rt.FamilyID)
		}
		return nil, invalid
	}
	if time.Now().After(rt.ExpiresAt) {
		return nil, invalid
	}
	won, err := s.tokens.Revoke(ctx, rt.ID) // atomic: only one concurrent caller can spend the token
	if err != nil {
		return nil, err
	}
	if !won {
		return nil, invalid
	}
	u, err := s.users.FindByID(ctx, rt.UserID)
	if err != nil {
		if isNotFound(err) {
			return nil, invalid
		}
		return nil, err
	}
	return s.newSession(ctx, u, rt.FamilyID)
}

// Logout is idempotent: an unknown or already-revoked token is not an error.
func (s *AuthService) Logout(ctx context.Context, raw string) error {
	if raw == "" {
		return nil
	}
	rt, err := s.tokens.FindByHash(ctx, auth.HashToken(raw))
	if err != nil {
		if isNotFound(err) {
			return nil
		}
		return err
	}
	return s.tokens.RevokeFamily(ctx, rt.FamilyID)
}

func (s *AuthService) Me(ctx context.Context, userID bson.ObjectID) (*model.User, error) {
	u, err := s.users.FindByID(ctx, userID)
	if isNotFound(err) {
		return nil, apperr.Unauthorized("account no longer exists")
	}
	return u, err
}

func (s *AuthService) ParseAccessToken(raw string) (bson.ObjectID, error) {
	sub, err := s.tm.ParseAccessToken(raw)
	if err != nil {
		return bson.NilObjectID, err
	}
	return bson.ObjectIDFromHex(sub)
}

func (s *AuthService) newSession(ctx context.Context, u *model.User, family bson.ObjectID) (*Session, error) {
	access, exp, err := s.tm.NewAccessToken(u.ID.Hex())
	if err != nil {
		return nil, err
	}
	raw, hash, err := auth.NewRefreshToken()
	if err != nil {
		return nil, err
	}
	rt := &model.RefreshToken{UserID: u.ID, FamilyID: family, TokenHash: hash, ExpiresAt: time.Now().Add(s.refreshTTL).UTC()}
	if err := s.tokens.Create(ctx, rt); err != nil {
		return nil, err
	}
	return &Session{User: u, AccessToken: access, AccessExpires: exp, RefreshToken: raw, RefreshExpires: rt.ExpiresAt}, nil
}

func isNotFound(err error) bool {
	var ae *apperr.Error
	return errors.As(err, &ae) && ae.Code == "not_found"
}
