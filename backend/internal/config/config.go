// Package config loads runtime configuration from environment variables.
// There are deliberately no defaults for secrets: the server refuses to start without them.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env            string // "development" | "production"
	Port           string
	MongoURI       string
	MongoDB        string
	JWTSecret      string
	AccessTTL      time.Duration
	RefreshTTL     time.Duration
	BcryptCost     int
	CookieSecure   bool
	AllowedOrigins []string
	TrustedProxies []string
	ClientIPHeader string // header carrying the real client IP behind a CDN/proxy (e.g. CF-Connecting-IP); empty = ignore
	StaticDir      string // when set, the API also serves the built web UI from this directory (single-origin deploys)
	AuthRatePerMin int    // per-IP limit on login/register; 0 disables
	SeedOnStart    bool
}

func Load() (*Config, error) {
	var errs []error
	c := &Config{
		Env:            get("APP_ENV", "development"),
		Port:           get("PORT", "8080"),
		MongoURI:       os.Getenv("MONGO_URI"),
		MongoDB:        get("MONGO_DB", "flowmetrics"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		AccessTTL:      duration("ACCESS_TOKEN_TTL", 15*time.Minute, &errs),
		RefreshTTL:     duration("REFRESH_TOKEN_TTL", 7*24*time.Hour, &errs),
		BcryptCost:     integer("BCRYPT_COST", 12, &errs),
		AllowedOrigins: list(os.Getenv("CORS_ALLOWED_ORIGINS")),
		TrustedProxies: list(os.Getenv("TRUSTED_PROXIES")),
		ClientIPHeader: get("CLIENT_IP_HEADER", ""),
		StaticDir:      get("STATIC_DIR", ""),
		AuthRatePerMin: integer("AUTH_RATE_LIMIT_PER_MIN", 20, &errs),
		SeedOnStart:    get("SEED_ON_START", "false") == "true",
	}
	c.CookieSecure = get("COOKIE_SECURE", strconv.FormatBool(c.Env == "production")) == "true"

	if c.MongoURI == "" {
		errs = append(errs, errors.New("MONGO_URI is required"))
	}
	if len(c.JWTSecret) < 32 {
		errs = append(errs, errors.New("JWT_SECRET is required and must be at least 32 characters"))
	}
	if c.Env == "production" && strings.Contains(strings.ToLower(c.JWTSecret), "change-me") {
		errs = append(errs, errors.New("JWT_SECRET still holds the example placeholder; generate a real one (openssl rand -hex 32)"))
	}
	if c.BcryptCost < 4 || c.BcryptCost > 31 {
		errs = append(errs, errors.New("BCRYPT_COST must be between 4 and 31"))
	}
	return c, errors.Join(errs...)
}

func get(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func integer(key string, def int, errs *[]error) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("%s: %w", key, err))
		return def
	}
	return n
}

func duration(key string, def time.Duration, errs *[]error) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("%s: %w", key, err))
		return def
	}
	return d
}

func list(v string) []string {
	var out []string
	for _, s := range strings.Split(v, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
