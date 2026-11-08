// Package validate collects field errors so a request reports every problem at once.
package validate

import (
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"flowmetrics/internal/apperr"
)

type Errors map[string]string

// Err returns nil when no field failed.
func (e Errors) Err() error {
	if len(e) == 0 {
		return nil
	}
	return apperr.Validation(e)
}

// Text trims v and checks its rune length. min=0 makes the field optional.
func (e Errors) Text(field, v string, min, max int) string {
	v = strings.TrimSpace(v)
	n := utf8.RuneCountInString(v)
	switch {
	case n < min && n == 0:
		e[field] = "is required"
	case n < min:
		e[field] = "must be at least " + strconv.Itoa(min) + " characters"
	case n > max:
		e[field] = "must be at most " + strconv.Itoa(max) + " characters"
	}
	return v
}

// Email returns the normalised (lower-case) address.
func (e Errors) Email(field, v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" {
		e[field] = "is required"
	} else if a, err := mail.ParseAddress(v); err != nil || a.Address != v || len(v) > 254 {
		e[field] = "must be a valid email address"
	}
	return v
}

// OneOf checks v against an allowed set.
func (e Errors) OneOf(field, v string, allowed ...string) {
	for _, a := range allowed {
		if v == a {
			return
		}
	}
	e[field] = "must be one of: " + strings.Join(allowed, ", ")
}

// Date parses "YYYY-MM-DD" or RFC3339 and normalises to midnight UTC (due dates are calendar dates).
func (e Errors) Date(field, v string) *time.Time {
	t, err := time.Parse("2006-01-02", v)
	if err != nil {
		t, err = time.Parse(time.RFC3339, v)
		if err != nil {
			e[field] = "must be a date in YYYY-MM-DD format"
			return nil
		}
		t = t.UTC()
	}
	d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return &d
}
