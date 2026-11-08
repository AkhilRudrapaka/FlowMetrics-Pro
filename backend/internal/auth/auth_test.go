package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const secret = "0123456789abcdef0123456789abcdef"

func TestAccessTokenRoundTrip(t *testing.T) {
	m := NewTokenManager(secret, time.Minute)
	tok, _, err := m.NewAccessToken("user-1")
	if err != nil {
		t.Fatal(err)
	}
	got, err := m.ParseAccessToken(tok)
	if err != nil || got != "user-1" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestAccessTokenRejections(t *testing.T) {
	m := NewTokenManager(secret, time.Minute)

	expired, _, _ := NewTokenManager(secret, -time.Minute).NewAccessToken("u")
	wrongKey, _, _ := NewTokenManager("another-secret-another-secret-123456", time.Minute).NewAccessToken("u")
	// alg=none is the classic bypass: a valid-looking unsigned token must be refused.
	none, _ := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.RegisteredClaims{
		Issuer: issuer, Subject: "u", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	// HS512 with the right key: wrong algorithm for this service.
	hs512, _ := jwt.NewWithClaims(jwt.SigningMethodHS512, jwt.RegisteredClaims{
		Issuer: issuer, Subject: "u", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}).SignedString([]byte(secret))
	noExp, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{Issuer: issuer, Subject: "u"}).SignedString([]byte(secret))

	for name, tok := range map[string]string{
		"expired": expired, "wrong key": wrongKey, "alg none": none, "wrong alg": hs512, "no expiry": noExp, "garbage": "x.y.z",
	} {
		if _, err := m.ParseAccessToken(tok); err == nil {
			t.Errorf("%s: token was accepted", name)
		}
	}
}

func TestPassword(t *testing.T) {
	h, err := HashPassword("correct horse", 4)
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(h, "correct horse") || CheckPassword(h, "wrong") {
		t.Fatal("password check misbehaves")
	}
}

func TestRefreshToken(t *testing.T) {
	a, ah, _ := NewRefreshToken()
	b, _, _ := NewRefreshToken()
	if a == b || HashToken(a) != ah || ah == a {
		t.Fatal("refresh tokens must be unique and stored hashed")
	}
}
