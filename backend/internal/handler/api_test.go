package handler_test

// Integration tests: real router + real MongoDB. Skipped unless MONGO_TEST_URI is set, e.g.
//   MONGO_TEST_URI=mongodb://localhost:27017 go test ./...
// Each test run uses a throwaway database that is dropped afterwards.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"flowmetrics/internal/auth"
	"flowmetrics/internal/config"
	"flowmetrics/internal/database"
	"flowmetrics/internal/handler"
	"flowmetrics/internal/repository"
	"flowmetrics/internal/service"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type env struct {
	t  *testing.T
	r  http.Handler
	db *mongo.Database
}

type res struct {
	code    int
	body    map[string]any
	cookies []*http.Cookie
}

func newEnv(t *testing.T) *env { return newEnvWith(t, nil) }

// newEnvWith lets a test adjust the config (e.g. enable the rate limiter) before the router is built.
func newEnvWith(t *testing.T, tweak func(*config.Config)) *env {
	t.Helper()
	uri := os.Getenv("MONGO_TEST_URI")
	if uri == "" {
		t.Skip("MONGO_TEST_URI not set; skipping integration test")
	}
	ctx := context.Background()
	client, err := database.Connect(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	db := client.Database(fmt.Sprintf("fm_test_%d", time.Now().UnixNano()))
	t.Cleanup(func() { _ = db.Drop(ctx); _ = client.Disconnect(ctx) })
	if err := database.EnsureIndexes(ctx, db); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{BcryptCost: 4, AccessTTL: time.Minute, RefreshTTL: time.Hour}
	if tweak != nil {
		tweak(cfg)
	}
	svc := service.New(repository.New(db), auth.NewTokenManager("test-secret-test-secret-test-secret-1234", time.Minute), 4, time.Hour)
	return &env{t, handler.NewRouter(cfg, svc, func(ctx context.Context) error { return client.Ping(ctx, nil) }), db}
}

func (e *env) call(method, path, token string, body any, cookies ...*http.Cookie) res {
	e.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if s, ok := body.(string); ok {
			buf.WriteString(s)
		} else if err := json.NewEncoder(&buf).Encode(body); err != nil {
			e.t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, "/api/v1"+path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	out := res{code: w.Code, cookies: w.Result().Cookies()}
	_ = json.Unmarshal(w.Body.Bytes(), &out.body)
	return out
}

func (r res) data() map[string]any { d, _ := r.body["data"].(map[string]any); return d }
func (r res) list() []any          { d, _ := r.body["data"].([]any); return d }
func (r res) id() string           { s, _ := r.data()["id"].(string); return s }
func (r res) meta() map[string]any { m, _ := r.body["meta"].(map[string]any); return m }
func (r res) errCode() string {
	m, _ := r.body["error"].(map[string]any)
	s, _ := m["code"].(string)
	return s
}

func (e *env) expect(r res, code int) res {
	e.t.Helper()
	if r.code != code {
		e.t.Fatalf("status = %d, want %d; body = %v", r.code, code, r.body)
	}
	return r
}

type user struct {
	id, token string
	cookie    *http.Cookie
}

func (e *env) register(name, email string) user {
	e.t.Helper()
	r := e.expect(e.call("POST", "/auth/register", "", map[string]string{"name": name, "email": email, "password": "Passw0rd!x"}), 201)
	u := user{id: r.data()["user"].(map[string]any)["id"].(string), token: r.data()["access_token"].(string)}
	for _, c := range r.cookies {
		if c.Name == "fm_refresh" {
			u.cookie = c
		}
	}
	return u
}

func (e *env) org(u user, name string) string {
	e.t.Helper()
	return e.expect(e.call("POST", "/orgs", u.token, map[string]string{"name": name}), 201).id()
}

func (e *env) addMember(owner user, org, email, role string) {
	e.t.Helper()
	e.expect(e.call("POST", "/orgs/"+org+"/members", owner.token, map[string]string{"email": email, "role": role}), 201)
}

func (e *env) project(u user, org, name string, members ...string) string {
	e.t.Helper()
	return e.expect(e.call("POST", "/orgs/"+org+"/projects", u.token, map[string]any{"name": name, "member_ids": members}), 201).id()
}

func (e *env) task(u user, org, project string, body map[string]any) string {
	e.t.Helper()
	return e.expect(e.call("POST", "/orgs/"+org+"/projects/"+project+"/tasks", u.token, body), 201).id()
}

func TestAuthFlow(t *testing.T) {
	e := newEnv(t)
	u := e.register("Ann", "ann@test.dev")

	e.expect(e.call("POST", "/auth/register", "", map[string]string{"name": "Ann", "email": "ANN@test.dev", "password": "Passw0rd!x"}), 409) // case-insensitive duplicate
	r := e.expect(e.call("POST", "/auth/register", "", map[string]string{"name": "", "email": "nope", "password": "short"}), 422)
	if len(r.body["error"].(map[string]any)["details"].(map[string]any)) != 3 {
		t.Errorf("expected 3 field errors, got %v", r.body)
	}

	// Wrong password and unknown email must be indistinguishable.
	a := e.expect(e.call("POST", "/auth/login", "", map[string]string{"email": "ann@test.dev", "password": "wrong-password"}), 401)
	b := e.expect(e.call("POST", "/auth/login", "", map[string]string{"email": "ghost@test.dev", "password": "wrong-password"}), 401)
	if !reflect.DeepEqual(a.body, b.body) {
		t.Errorf("login errors differ: %v vs %v", a.body, b.body)
	}
	e.expect(e.call("POST", "/auth/login", "", map[string]string{"email": "ann@test.dev", "password": "Passw0rd!x"}), 200)

	e.expect(e.call("GET", "/auth/me", u.token, nil), 200)
	e.expect(e.call("GET", "/auth/me", "", nil), 401)
	e.expect(e.call("GET", "/auth/me", "garbage.token.here", nil), 401)

	// The password hash must never be serialised.
	if _, leaked := e.call("GET", "/auth/me", u.token, nil).data()["password_hash"]; leaked {
		t.Error("password_hash leaked in response")
	}
}

func TestRefreshRotationAndReuseDetection(t *testing.T) {
	e := newEnv(t)
	u := e.register("Ann", "ann@test.dev")
	if u.cookie == nil || !u.cookie.HttpOnly || u.cookie.Path != "/api/v1/auth" {
		t.Fatalf("refresh cookie must be httpOnly and path-scoped: %+v", u.cookie)
	}

	r1 := e.expect(e.call("POST", "/auth/refresh", "", nil, u.cookie), 200)
	var c2 *http.Cookie
	for _, c := range r1.cookies {
		if c.Name == "fm_refresh" {
			c2 = c
		}
	}
	if c2 == nil || c2.Value == u.cookie.Value {
		t.Fatal("refresh token was not rotated")
	}
	e.expect(e.call("POST", "/auth/refresh", "", nil), 401) // no cookie

	// Replaying the spent token after the grace window is theft: the whole family dies.
	_, err := e.db.Collection("refresh_tokens").UpdateMany(context.Background(), bson.M{"revoked_at": bson.M{"$ne": nil}},
		bson.M{"$set": bson.M{"revoked_at": time.Now().Add(-time.Minute)}})
	if err != nil {
		t.Fatal(err)
	}
	e.expect(e.call("POST", "/auth/refresh", "", nil, u.cookie), 401)
	e.expect(e.call("POST", "/auth/refresh", "", nil, c2), 401) // the legitimate new token is revoked too

	// Logout revokes the session.
	v := e.register("Ben", "ben@test.dev")
	e.expect(e.call("POST", "/auth/logout", "", nil, v.cookie), 204)
	e.expect(e.call("POST", "/auth/refresh", "", nil, v.cookie), 401)
	e.expect(e.call("POST", "/auth/logout", "", nil), 204) // idempotent
}

func TestTenantIsolation(t *testing.T) {
	e := newEnv(t)
	alice, mallory := e.register("Alice", "alice@a.dev"), e.register("Mallory", "mallory@b.dev")
	orgA, orgB := e.org(alice, "Alpha"), e.org(mallory, "Beta")
	projA := e.project(alice, orgA, "Secret project", alice.id)
	taskA := e.task(alice, orgA, projA, map[string]any{"title": "Secret task", "assignee_id": alice.id})
	commentA := e.expect(e.call("POST", "/orgs/"+orgA+"/tasks/"+taskA+"/comments", alice.token, map[string]string{"body": "hush"}), 201).id()

	missing := bson.NewObjectID().Hex()
	pa, ta := "/orgs/"+orgA, "/orgs/"+orgA+"/tasks/"+taskA

	// 1. Mallory addressing Alice's org directly: every route is 404 (not 403: existence is not confirmed).
	for _, c := range []struct{ method, path string }{
		{"GET", pa}, {"PATCH", pa}, {"DELETE", pa}, {"GET", pa + "/dashboard"}, {"GET", pa + "/activity"},
		{"GET", pa + "/members"}, {"POST", pa + "/members"}, {"DELETE", pa + "/members/" + alice.id},
		{"GET", pa + "/projects"}, {"POST", pa + "/projects"}, {"GET", pa + "/projects/" + projA},
		{"POST", pa + "/projects/" + projA + "/tasks"}, {"GET", pa + "/tasks"}, {"GET", ta}, {"PATCH", ta}, {"DELETE", ta},
		{"GET", ta + "/comments"}, {"POST", ta + "/comments"}, {"DELETE", ta + "/comments/" + commentA}, {"GET", ta + "/activity"},
	} {
		r := e.call(c.method, c.path, mallory.token, map[string]string{})
		if r.code != 404 {
			t.Errorf("%s %s as outsider = %d, want 404", c.method, c.path, r.code)
		}
	}

	// 2. Mallory inside her OWN org, but guessing Alice's object ids: still 404, byte-identical to a missing id.
	pb := "/orgs/" + orgB
	for _, c := range []struct{ method, path, missing string }{
		{"GET", pb + "/projects/" + projA, pb + "/projects/" + missing},
		{"PATCH", pb + "/projects/" + projA, pb + "/projects/" + missing},
		{"GET", pb + "/tasks/" + taskA, pb + "/tasks/" + missing},
		{"PATCH", pb + "/tasks/" + taskA, pb + "/tasks/" + missing},
		{"DELETE", pb + "/tasks/" + taskA, pb + "/tasks/" + missing},
		{"GET", pb + "/tasks/" + taskA + "/comments", pb + "/tasks/" + missing + "/comments"},
		{"POST", pb + "/projects/" + projA + "/tasks", pb + "/projects/" + missing + "/tasks"},
		{"GET", pb + "/tasks?project_id=" + projA, pb + "/tasks?project_id=" + missing},
	} {
		body := map[string]string{"status": "done"} // a field every PATCH/POST above accepts, so only the id decides the outcome
		got := e.call(c.method, c.path, mallory.token, body)
		want := e.call(c.method, c.missing, mallory.token, body)
		if got.code != 404 || !reflect.DeepEqual(got.body, want.body) {
			t.Errorf("%s %s: got %d %v, want the same 404 as a nonexistent id (%d %v)", c.method, c.path, got.code, got.body, want.code, want.body)
		}
	}

	// 3. Org-wide lists in Mallory's org never contain Alice's data.
	if n := len(e.expect(e.call("GET", pb+"/tasks", mallory.token, nil), 200).list()); n != 0 {
		t.Errorf("Mallory sees %d foreign tasks", n)
	}
	if n := len(e.expect(e.call("GET", pb+"/projects", mallory.token, nil), 200).list()); n != 0 {
		t.Errorf("Mallory sees %d foreign projects", n)
	}
	d := e.expect(e.call("GET", pb+"/dashboard", mallory.token, nil), 200).data()
	if d["total_tasks"].(float64) != 0 || d["total_projects"].(float64) != 0 || len(d["recent_activity"].([]any)) != 0 {
		t.Errorf("dashboard leaks foreign data: %v", d)
	}

	// 4. A foreign user cannot be attached to a project, nor a foreign id used as an assignee.
	projB := e.project(mallory, orgB, "Mine", mallory.id)
	e.expect(e.call("PUT", pb+"/projects/"+projB+"/members/"+alice.id, mallory.token, nil), 422)
	e.expect(e.call("POST", pb+"/projects", mallory.token, map[string]any{"name": "x", "member_ids": []string{alice.id}}), 422)
	e.expect(e.call("POST", pb+"/projects/"+projB+"/tasks", mallory.token, map[string]any{"title": "x", "assignee_id": alice.id}), 422)

	// 5. Alice's data is intact and unauthenticated access is refused.
	e.expect(e.call("GET", ta, alice.token, nil), 200)
	e.expect(e.call("GET", ta, "", nil), 401)

	// 6. Membership is re-checked on every request: once removed, a still-valid token is useless.
	e.addMember(alice, orgA, "mallory@b.dev", "member")
	e.expect(e.call("GET", ta, mallory.token, nil), 200)
	e.expect(e.call("DELETE", pa+"/members/"+mallory.id, alice.token, nil), 204)
	e.expect(e.call("GET", ta, mallory.token, nil), 404)
}

func TestRBAC(t *testing.T) {
	e := newEnv(t)
	owner, admin, member := e.register("Olive", "olive@t.dev"), e.register("Adam", "adam@t.dev"), e.register("Mia", "mia@t.dev")
	org := e.org(owner, "Org")
	e.addMember(owner, org, "adam@t.dev", "admin")
	e.addMember(owner, org, "mia@t.dev", "member")
	o := "/orgs/" + org

	// Org-level
	e.expect(e.call("POST", o+"/members", admin.token, map[string]string{"email": "adam@t.dev", "role": "admin"}), 403)   // admin can't mint admins
	e.expect(e.call("POST", o+"/members", member.token, map[string]string{"email": "adam@t.dev", "role": "member"}), 403) // member can't add anyone
	e.expect(e.call("POST", o+"/members", owner.token, map[string]string{"email": "nobody@t.dev", "role": "member"}), 422)
	e.expect(e.call("POST", o+"/members", owner.token, map[string]string{"email": "mia@t.dev", "role": "owner"}), 422) // there is one owner
	e.expect(e.call("PATCH", o+"/members/"+member.id, admin.token, map[string]string{"role": "admin"}), 403)           // only the owner changes roles
	e.expect(e.call("PATCH", o+"/members/"+owner.id, owner.token, map[string]string{"role": "member"}), 403)           // the owner is immutable
	e.expect(e.call("PATCH", o, member.token, map[string]string{"name": "Hacked"}), 403)
	e.expect(e.call("PATCH", o, admin.token, map[string]string{"name": "Renamed Org"}), 200)
	e.expect(e.call("DELETE", o+"/members/"+admin.id, member.token, nil), 403)
	e.expect(e.call("DELETE", o+"/members/"+owner.id, admin.token, nil), 403)
	e.expect(e.call("DELETE", o+"/members/"+owner.id, owner.token, nil), 409) // owner can't abandon the org
	e.expect(e.call("DELETE", o, admin.token, nil), 403)

	// Project-level
	e.expect(e.call("POST", o+"/projects", member.token, map[string]string{"name": "P"}), 403)
	proj := e.project(admin, org, "Admin project", member.id)
	e.expect(e.call("PATCH", o+"/projects/"+proj, member.token, map[string]string{"name": "x"}), 403)
	e.expect(e.call("POST", o+"/projects/"+proj+"/archive", member.token, nil), 403)
	e.expect(e.call("PUT", o+"/projects/"+proj+"/members/"+owner.id, member.token, nil), 403)

	// Task-level: a plain member who is NOT in the project can read but not write.
	other := e.project(admin, org, "Other project") // members: admin only
	e.expect(e.call("GET", o+"/projects/"+other, member.token, nil), 200)
	e.expect(e.call("POST", o+"/projects/"+other+"/tasks", member.token, map[string]string{"title": "sneaky"}), 403)
	t1 := e.task(admin, org, other, map[string]any{"title": "Admin's task"})
	e.expect(e.call("GET", o+"/tasks/"+t1, member.token, nil), 200)
	e.expect(e.call("PATCH", o+"/tasks/"+t1, member.token, map[string]string{"status": "done"}), 403)
	e.expect(e.call("POST", o+"/tasks/"+t1+"/comments", member.token, map[string]string{"body": "hi"}), 403)

	// Inside her project the member can work, but only delete her own tasks.
	mine := e.task(member, org, proj, map[string]any{"title": "Member's task"})
	adminsTask := e.task(admin, org, proj, map[string]any{"title": "Admin's task 2"})
	e.expect(e.call("PATCH", o+"/tasks/"+adminsTask, member.token, map[string]string{"status": "in_progress"}), 200)
	e.expect(e.call("DELETE", o+"/tasks/"+adminsTask, member.token, nil), 403)
	e.expect(e.call("DELETE", o+"/tasks/"+mine, member.token, nil), 204)
	e.expect(e.call("DELETE", o+"/tasks/"+adminsTask, admin.token, nil), 204)

	// Owner promotes, then the new admin can create projects; demotion takes it away again.
	e.expect(e.call("PATCH", o+"/members/"+member.id, owner.token, map[string]string{"role": "admin"}), 204)
	e.expect(e.call("POST", o+"/projects", member.token, map[string]string{"name": "Now allowed"}), 201)
	e.expect(e.call("PATCH", o+"/members/"+member.id, owner.token, map[string]string{"role": "member"}), 204)
	e.expect(e.call("POST", o+"/projects", member.token, map[string]string{"name": "No longer"}), 403)

	// Leaving, then deletion cascade.
	e.expect(e.call("DELETE", o+"/members/"+member.id, member.token, nil), 204)
	e.expect(e.call("GET", o, member.token, nil), 404)
	e.expect(e.call("DELETE", o, owner.token, nil), 204)
	e.expect(e.call("GET", o, owner.token, nil), 404)
	for _, coll := range []string{"projects", "tasks", "comments", "activities", "memberships"} {
		oid, _ := bson.ObjectIDFromHex(org)
		if n, _ := e.db.Collection(coll).CountDocuments(context.Background(), bson.M{"org_id": oid}); n != 0 {
			t.Errorf("%s still has %d documents after org deletion", coll, n)
		}
	}
}

func TestTaskLifecycle(t *testing.T) {
	e := newEnv(t)
	owner, member, outsider := e.register("Olive", "olive@t.dev"), e.register("Mia", "mia@t.dev"), e.register("Ola", "ola@t.dev")
	org := e.org(owner, "Org")
	e.addMember(owner, org, "mia@t.dev", "member")
	e.addMember(owner, org, "ola@t.dev", "member")
	o := "/orgs/" + org
	proj := e.project(owner, org, "P", member.id)

	// Validation and unknown fields
	e.expect(e.call("POST", o+"/projects/"+proj+"/tasks", owner.token, map[string]any{"title": " ", "status": "nope", "priority": "urgent", "due_date": "soon"}), 422)
	e.expect(e.call("POST", o+"/projects/"+proj+"/tasks", owner.token, `{"title":"x","bogus":1}`), 400)
	e.expect(e.call("POST", o+"/projects/"+proj+"/tasks", owner.token, `{not json`), 400)
	e.expect(e.call("POST", o+"/projects/"+proj+"/tasks", owner.token, map[string]any{"title": "x", "assignee_id": outsider.id}), 422) // org member but not in the project

	id := e.task(owner, org, proj, map[string]any{
		"title": "Ship it", "status": "todo", "priority": "high", "assignee_id": member.id,
		"due_date": "2030-01-15", "labels": []string{" Bug ", "bug", "UI"},
	})
	task := e.expect(e.call("GET", o+"/tasks/"+id, owner.token, nil), 200).data()
	if !reflect.DeepEqual(task["labels"], []any{"bug", "ui"}) || task["project_name"] != "P" || task["assignee_id"] != member.id {
		t.Errorf("unexpected task: %v", task)
	}

	// Status change is persisted and stamps completed_at; leaving "done" clears it.
	done := e.expect(e.call("PATCH", o+"/tasks/"+id, member.token, map[string]string{"status": "done"}), 200).data()
	if done["status"] != "done" || done["completed_at"] == nil {
		t.Errorf("done task: %v", done)
	}
	reopened := e.expect(e.call("PATCH", o+"/tasks/"+id, member.token, map[string]string{"status": "in_progress"}), 200).data()
	if reopened["completed_at"] != nil {
		t.Errorf("completed_at not cleared: %v", reopened)
	}
	e.expect(e.call("PATCH", o+"/tasks/"+id, member.token, map[string]string{"status": "finished"}), 422)

	// Explicit null clears; absence leaves untouched.
	cleared := e.expect(e.call("PATCH", o+"/tasks/"+id, owner.token, `{"assignee_id":null,"due_date":null}`), 200).data()
	if cleared["assignee_id"] != nil || cleared["due_date"] != nil || cleared["priority"] != "high" {
		t.Errorf("null handling: %v", cleared)
	}
	e.expect(e.call("PATCH", o+"/tasks/"+id, owner.token, map[string]any{"assignee_id": outsider.id}), 422)

	// Comments: anyone with write access may post; authors (and admins) may delete.
	c1 := e.expect(e.call("POST", o+"/tasks/"+id+"/comments", member.token, map[string]string{"body": "on it"}), 201)
	if c1.data()["author_name"] != "Mia" {
		t.Errorf("comment: %v", c1.data())
	}
	e.expect(e.call("POST", o+"/tasks/"+id+"/comments", member.token, map[string]string{"body": "  "}), 422)
	e.expect(e.call("DELETE", o+"/tasks/"+id+"/comments/"+c1.id(), outsider.token, nil), 403)
	e.expect(e.call("DELETE", o+"/tasks/"+id+"/comments/"+c1.id(), owner.token, nil), 204)
	e.expect(e.call("DELETE", o+"/tasks/"+id+"/comments/"+c1.id(), owner.token, nil), 404)

	// History records what changed.
	actions := map[string]bool{}
	for _, a := range e.expect(e.call("GET", o+"/tasks/"+id+"/activity", owner.token, nil), 200).list() {
		actions[a.(map[string]any)["action"].(string)] = true
	}
	for _, want := range []string{"task.created", "task.status_changed", "task.assigned", "comment.added"} {
		if !actions[want] {
			t.Errorf("history missing %q: %v", want, actions)
		}
	}

	// Removing a member from the project unassigns their tasks.
	id2 := e.task(owner, org, proj, map[string]any{"title": "Assigned", "assignee_id": member.id})
	e.expect(e.call("DELETE", o+"/projects/"+proj+"/members/"+member.id, owner.token, nil), 200)
	if e.expect(e.call("GET", o+"/tasks/"+id2, owner.token, nil), 200).data()["assignee_id"] != nil {
		t.Error("task still assigned to removed project member")
	}

	// Archived projects are read-only; restoring reopens them.
	e.expect(e.call("POST", o+"/projects/"+proj+"/archive", owner.token, nil), 200)
	e.expect(e.call("POST", o+"/projects/"+proj+"/tasks", owner.token, map[string]string{"title": "late"}), 409)
	e.expect(e.call("PATCH", o+"/tasks/"+id, owner.token, map[string]string{"status": "done"}), 409)
	e.expect(e.call("PATCH", o+"/projects/"+proj, owner.token, map[string]string{"name": "renamed"}), 409)
	e.expect(e.call("PATCH", o+"/projects/"+proj, owner.token, map[string]string{"status": "active"}), 200)
	e.expect(e.call("PATCH", o+"/tasks/"+id, owner.token, map[string]string{"status": "done"}), 200)
}

func TestSearchFilterPagination(t *testing.T) {
	e := newEnv(t)
	u := e.register("Olive", "olive@t.dev")
	org := e.org(u, "Org")
	o := "/orgs/" + org
	proj := e.project(u, org, "P", u.id)
	other := e.project(u, org, "Other", u.id)

	statuses := []string{"todo", "in_progress", "done"}
	for i := 0; i < 25; i++ {
		body := map[string]any{"title": fmt.Sprintf("Task %02d", i), "status": statuses[i%3], "priority": []string{"low", "high"}[i%2]}
		switch {
		case i == 3:
			body["title"] = "Fix the login (v2.0) bug"
			body["assignee_id"] = u.id
		case i == 4:
			body["due_date"] = "2000-01-01" // overdue and not done (4%3==1 → in_progress)
		case i == 5:
			body["due_date"] = "2000-01-01" // overdue but done → must not count
		}
		e.task(u, org, proj, body)
	}
	e.task(u, org, other, map[string]any{"title": "Elsewhere"})

	page := func(q string) res { return e.expect(e.call("GET", o+"/tasks?"+q, u.token, nil), 200) }
	total := func(q string) float64 { return page(q).meta()["total"].(float64) }

	r := page("project_id=" + proj + "&page_size=10")
	if len(r.list()) != 10 || r.meta()["total"].(float64) != 25 || r.meta()["total_pages"].(float64) != 3 {
		t.Errorf("page 1: %d items, meta %v", len(r.list()), r.meta())
	}
	if n := len(page("project_id=" + proj + "&page_size=10&page=3").list()); n != 5 {
		t.Errorf("last page has %d items, want 5", n)
	}
	if n := len(page("project_id=" + proj + "&page=99").list()); n != 0 {
		t.Errorf("out-of-range page has %d items", n)
	}

	// Stable, non-overlapping pages.
	seen := map[string]bool{}
	for p := 1; p <= 3; p++ {
		for _, it := range page(fmt.Sprintf("project_id=%s&page_size=10&page=%d&sort=title", proj, p)).list() {
			id := it.(map[string]any)["id"].(string)
			if seen[id] {
				t.Fatalf("task %s appears on two pages", id)
			}
			seen[id] = true
		}
	}
	if len(seen) != 25 {
		t.Errorf("paged through %d tasks, want 25", len(seen))
	}

	if got := total("status=done&project_id=" + proj); got != 8 {
		t.Errorf("done = %v, want 8", got)
	}
	if got := total("status=todo,in_progress&project_id=" + proj); got != 17 {
		t.Errorf("todo+in_progress = %v, want 17", got)
	}
	if got := total("priority=high&project_id=" + proj); got != 12 {
		t.Errorf("high = %v, want 12", got)
	}
	if got := total("q=LOGIN"); got != 1 { // case-insensitive
		t.Errorf("search login = %v, want 1", got)
	}
	if got := total("q=v2.0"); got != 1 { // '.' is literal
		t.Errorf("search v2.0 = %v, want 1", got)
	}
	if got := total("q=.*"); got != 0 { // regex metacharacters are escaped, not interpreted
		t.Errorf("search '.*' = %v, want 0", got)
	}
	if got := total("overdue=true"); got != 1 {
		t.Errorf("overdue = %v, want 1", got)
	}
	if got := total("assignee=me"); got != 1 {
		t.Errorf("assignee=me = %v, want 1", got)
	}
	if got := total("assignee=unassigned&project_id=" + proj); got != 24 {
		t.Errorf("unassigned = %v, want 24", got)
	}
	if got := total("label=nothing"); got != 0 {
		t.Errorf("label = %v", got)
	}
	if got := total(""); got != 26 {
		t.Errorf("org-wide = %v, want 26", got)
	}

	for _, bad := range []string{"sort=password", "page_size=1000", "page=0", "page=abc", "status=bogus", "assignee=zzz", "project_id=zzz"} {
		e.expect(e.call("GET", o+"/tasks?"+bad, u.token, nil), 422)
	}

	// Archived projects drop out of org-wide lists and dashboard counters.
	e.expect(e.call("POST", o+"/projects/"+other+"/archive", u.token, nil), 200)
	if got := total(""); got != 25 {
		t.Errorf("org-wide after archive = %v, want 25", got)
	}
	e.expect(e.call("GET", o+"/projects?q=Other", u.token, nil), 200)
	if n := len(e.call("GET", o+"/projects", u.token, nil).list()); n != 1 {
		t.Errorf("archived project listed by default (%d projects)", n)
	}
	if n := len(e.call("GET", o+"/projects?status=archived", u.token, nil).list()); n != 1 {
		t.Errorf("status=archived returned %d projects", n)
	}

	d := e.expect(e.call("GET", o+"/dashboard", u.token, nil), 200).data()
	want := map[string]float64{"total_projects": 1, "active_projects": 1, "total_tasks": 25, "completed_tasks": 8, "overdue_tasks": 1, "my_open_tasks": 1}
	for k, v := range want {
		if d[k].(float64) != v {
			t.Errorf("dashboard %s = %v, want %v", k, d[k], v)
		}
	}
	// active = todo + in_progress (backlog/done/in_review/blocked excluded): 9 todo + 8 in_progress = 17
	if d["active_tasks"].(float64) != 17 {
		t.Errorf("dashboard active_tasks = %v, want 17", d["active_tasks"])
	}
	if len(d["recent_activity"].([]any)) != 10 {
		t.Errorf("recent_activity has %d entries, want 10", len(d["recent_activity"].([]any)))
	}
}

// Behind a CDN (Render sits behind Cloudflare) every request arrives from the proxy's address, so the
// login rate limiter must key on the real client IP header or one visitor would throttle everyone.
func TestClientIPHeader(t *testing.T) {
	failedLogin := func(e *env, clientIP string) int {
		req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(`{"email":"x@y.dev","password":"wrong-password"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("CF-Connecting-IP", clientIP)
		w := httptest.NewRecorder()
		e.r.ServeHTTP(w, req)
		return w.Code
	}

	t.Run("configured: every visitor gets their own bucket", func(t *testing.T) {
		e := newEnvWith(t, func(c *config.Config) { c.AuthRatePerMin = 2; c.ClientIPHeader = "CF-Connecting-IP" }) // burst = 5
		for i := 0; i < 5; i++ {
			if got := failedLogin(e, "203.0.113.1"); got != 401 {
				t.Fatalf("attempt %d from visitor A = %d, want 401", i+1, got)
			}
		}
		if got := failedLogin(e, "203.0.113.1"); got != 429 {
			t.Errorf("visitor A's 6th attempt = %d, want 429", got)
		}
		if got := failedLogin(e, "203.0.113.2"); got != 401 {
			t.Errorf("visitor B = %d, want 401: one visitor must not throttle another", got)
		}
	})

	t.Run("not configured: the header is ignored, so it cannot be used to dodge the limiter", func(t *testing.T) {
		e := newEnvWith(t, func(c *config.Config) { c.AuthRatePerMin = 2 })
		for i := 0; i < 5; i++ {
			failedLogin(e, fmt.Sprintf("198.51.100.%d", i)) // a different claimed IP each time
		}
		if got := failedLogin(e, "198.51.100.200"); got != 429 {
			t.Errorf("spoofed header bypassed the limiter: got %d, want 429", got)
		}
	})
}

func TestHealthAndNotFound(t *testing.T) {
	e := newEnv(t)
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, httptest.NewRequest("GET", "/healthz", nil))
	if w.Code != 200 {
		t.Errorf("healthz = %d", w.Code)
	}
	e.expect(e.call("GET", "/nope", "", nil), 404)
}
