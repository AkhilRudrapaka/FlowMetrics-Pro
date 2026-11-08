package model

import (
	"encoding/json"
	"testing"
)

func TestNullable(t *testing.T) {
	var body struct {
		A Nullable[string] `json:"a"`
		B Nullable[string] `json:"b"`
		C Nullable[string] `json:"c"`
	}
	if err := json.Unmarshal([]byte(`{"a":null,"b":"x"}`), &body); err != nil {
		t.Fatal(err)
	}
	if !body.A.Set || body.A.Value != nil {
		t.Errorf("explicit null must be Set with nil Value: %+v", body.A)
	}
	if !body.B.Set || body.B.Value == nil || *body.B.Value != "x" {
		t.Errorf("value must be Set: %+v", body.B)
	}
	if body.C.Set {
		t.Errorf("absent field must not be Set: %+v", body.C)
	}
	if err := json.Unmarshal([]byte(`{"a":5}`), &body); err == nil {
		t.Error("wrong type must be rejected")
	}
}

func TestPageMeta(t *testing.T) {
	for _, c := range []struct {
		total int64
		pages int
	}{{0, 0}, {1, 1}, {20, 1}, {21, 2}, {100, 5}} {
		if got := NewPageMeta(PageQuery{1, 20}, c.total).TotalPages; got != c.pages {
			t.Errorf("total %d: pages = %d, want %d", c.total, got, c.pages)
		}
	}
	if (PageQuery{Page: 3, Size: 20}).Skip() != 40 {
		t.Error("skip is wrong")
	}
}
