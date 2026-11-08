package validate

import "testing"

func TestText(t *testing.T) {
	e := Errors{}
	if got := e.Text("a", "  hello  ", 1, 10); got != "hello" || len(e) != 0 {
		t.Errorf("trim failed: %q %v", got, e)
	}
	e.Text("b", "   ", 1, 10)
	e.Text("c", "12345678901", 1, 10)
	e.Text("d", "", 0, 10)            // optional
	e.Text("e", "日本語日本語日本語日", 1, 10)  // 10 runes = 30 bytes: the limit counts characters, not bytes
	e.Text("f", "日本語日本語日本語日本", 1, 10) // 11 runes: one too many
	if e["b"] != "is required" || e["c"] == "" || e["d"] != "" || e["e"] != "" || e["f"] == "" {
		t.Errorf("unexpected errors: %v", e)
	}
}

func TestEmail(t *testing.T) {
	good := Errors{}
	if got := good.Email("e", "  Ann@Example.COM "); got != "ann@example.com" || len(good) != 0 {
		t.Errorf("normalise failed: %q %v", got, good)
	}
	for _, bad := range []string{"", "nope", "a@", "Ann <ann@x.com>", "a b@x.com"} {
		e := Errors{}
		e.Email("e", bad)
		if e["e"] == "" {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

func TestDate(t *testing.T) {
	e := Errors{}
	if d := e.Date("d", "2030-01-15"); d == nil || d.Format("2006-01-02T15:04:05Z") != "2030-01-15T00:00:00Z" {
		t.Errorf("date-only parse: %v %v", d, e)
	}
	// A timestamp is truncated to its UTC calendar date: due dates carry no time of day.
	if d := e.Date("d", "2030-01-15T23:30:00Z"); d == nil || d.Hour() != 0 || d.Day() != 15 {
		t.Errorf("rfc3339 truncation: %v", d)
	}
	if e.Date("bad", "soon"); e["bad"] == "" {
		t.Error("garbage date accepted")
	}
}

func TestOneOf(t *testing.T) {
	e := Errors{}
	e.OneOf("s", "todo", "todo", "done")
	e.OneOf("t", "nope", "todo", "done")
	if e["s"] != "" || e["t"] == "" {
		t.Errorf("OneOf: %v", e)
	}
}
