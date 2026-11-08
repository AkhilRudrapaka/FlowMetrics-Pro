package config

import (
	"strings"
	"testing"
)

func setEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	for _, k := range []string{"APP_ENV", "MONGO_URI", "JWT_SECRET", "BCRYPT_COST", "ACCESS_TOKEN_TTL", "COOKIE_SECURE"} {
		t.Setenv(k, "")
	}
	for k, v := range kv {
		t.Setenv(k, v)
	}
}

func TestLoadRequiresSecrets(t *testing.T) {
	setEnv(t, nil)
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "MONGO_URI") || !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Fatalf("missing config must be reported, got %v", err)
	}
	setEnv(t, map[string]string{"MONGO_URI": "mongodb://x", "JWT_SECRET": "too-short"})
	if _, err := Load(); err == nil {
		t.Fatal("short JWT secret accepted")
	}
}

func TestLoadDefaultsAndProductionGuards(t *testing.T) {
	secret := strings.Repeat("s", 40)
	setEnv(t, map[string]string{"MONGO_URI": "mongodb://x", "JWT_SECRET": secret})
	c, err := Load()
	if err != nil || c.CookieSecure || c.AccessTTL.Minutes() != 15 || c.BcryptCost != 12 {
		t.Fatalf("dev defaults: %+v, %v", c, err)
	}

	setEnv(t, map[string]string{"MONGO_URI": "mongodb://x", "JWT_SECRET": secret, "APP_ENV": "production"})
	if c, err := Load(); err != nil || !c.CookieSecure {
		t.Fatalf("production must default to secure cookies: %+v, %v", c, err)
	}

	setEnv(t, map[string]string{"MONGO_URI": "mongodb://x", "JWT_SECRET": "dev-only-change-me-dev-only-change-me-00", "APP_ENV": "production"})
	if _, err := Load(); err == nil {
		t.Fatal("placeholder secret accepted in production")
	}

	setEnv(t, map[string]string{"MONGO_URI": "mongodb://x", "JWT_SECRET": secret, "ACCESS_TOKEN_TTL": "soon"})
	if _, err := Load(); err == nil {
		t.Fatal("bad duration accepted")
	}
}
