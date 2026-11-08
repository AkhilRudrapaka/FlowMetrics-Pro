// Package seed fills an empty database with a small, realistic demo workspace.
// It is idempotent: it does nothing if any user already exists.
//
// The data deliberately contains TWO organizations so tenant isolation is visible in the UI:
// Alice belongs to both (as owner of Acme, plain member of Globex); Dave belongs only to Globex.
package seed

import (
	"context"
	"log/slog"
	"time"

	"flowmetrics/internal/auth"
	"flowmetrics/internal/model"
	"flowmetrics/internal/repository"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const noDue = 9999

type taskSpec struct {
	project            int
	title, status, pri string
	assignee           int // index into the project's member list, -1 for none
	due                int // days from today (negative = overdue)
	labels             []string
}

func Run(ctx context.Context, r *repository.Repos, password string, bcryptCost int) error {
	if _, err := r.Users.FindByEmail(ctx, "alice@acme.test"); err == nil {
		slog.Info("seed: demo data already present, skipping")
		return nil
	}
	hash, err := auth.HashPassword(password, bcryptCost)
	if err != nil {
		return err
	}
	mk := func(name, email string) *model.User {
		u := &model.User{Name: name, Email: email, PasswordHash: hash}
		must(r.Users.Create(ctx, u))
		return u
	}
	alice, bob, carol, dave := mk("Alice Johnson", "alice@acme.test"), mk("Bob Smith", "bob@acme.test"),
		mk("Carol Diaz", "carol@acme.test"), mk("Dave Lee", "dave@globex.test")

	org := func(name string, owner *model.User) *model.Organization {
		o := &model.Organization{Name: name, OwnerID: owner.ID}
		must(r.Orgs.Create(ctx, o))
		join(ctx, r, o, owner, model.RoleOwner)
		return o
	}
	acme, globex := org("Acme Corp", alice), org("Globex Inc", dave)
	join(ctx, r, acme, bob, model.RoleAdmin)
	join(ctx, r, acme, carol, model.RoleMember)
	join(ctx, r, globex, alice, model.RoleMember)

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	ago := func(h int) time.Time { return now.Add(-time.Duration(h) * time.Hour).Truncate(time.Millisecond) }

	type projSpec struct {
		org     *model.Organization
		name    string
		desc    string
		status  string
		members []*model.User
		owner   *model.User
	}
	specs := []projSpec{
		{acme, "Website Redesign", "Rebuild the marketing site with a new information architecture, CMS and performance budget.", model.ProjectActive, []*model.User{alice, bob, carol}, alice},
		{acme, "Mobile App v2", "Second major release of the customer mobile app: offline mode, push preferences, and stability.", model.ProjectActive, []*model.User{alice, carol}, alice},
		{acme, "Q3 Infrastructure Migration", "Move production data to the new MongoDB cluster. Completed.", model.ProjectCompleted, []*model.User{bob, alice}, bob},
		{acme, "Legacy CRM Cleanup", "Retired initiative, kept for reference.", model.ProjectArchived, []*model.User{bob}, bob},
		{globex, "Rocket Fuel Optimization", "Confidential Globex project. Must never be visible to Acme users.", model.ProjectActive, []*model.User{dave, alice}, dave},
	}
	projects := make([]*model.Project, len(specs))
	for i, s := range specs {
		p := &model.Project{OrgID: s.org.ID, Name: s.name, Description: s.desc, Status: s.status, MemberIDs: []bson.ObjectID{}, CreatedBy: s.owner.ID, CreatedAt: ago(24 * (20 - i))}
		for _, m := range s.members {
			p.MemberIDs = append(p.MemberIDs, m.ID)
		}
		if s.status == model.ProjectArchived {
			p.ArchivedAt = ptr(ago(24 * 3))
		}
		must(r.Projects.Create(ctx, p))
		projects[i] = p
		must(r.Activity.Insert(ctx, &model.Activity{OrgID: s.org.ID, ProjectID: &p.ID, ActorID: s.owner.ID, ActorName: s.owner.Name, Action: "project.created", EntityTitle: p.Name, CreatedAt: p.CreatedAt}))
	}

	tasks := []taskSpec{
		{0, "Audit current site analytics", "done", "medium", 1, -10, []string{"research"}},
		{0, "Define new information architecture", "done", "high", 0, -6, []string{"ux", "planning"}},
		{0, "Design homepage wireframes", "in_review", "high", 2, 2, []string{"design", "ux"}},
		{0, "Implement responsive navigation", "in_progress", "high", 1, 5, []string{"frontend"}},
		{0, "Set up CMS content model", "in_progress", "medium", 0, 9, []string{"backend", "cms"}},
		{0, "Write copy for About page", "todo", "low", 2, 12, []string{"content"}},
		{0, "Accessibility audit (WCAG 2.2 AA)", "todo", "medium", -1, 14, []string{"a11y"}},
		{0, "Migrate blog posts to new CMS", "backlog", "low", -1, noDue, []string{"content", "migration"}},
		{0, "Fix broken image CDN URLs", "blocked", "critical", 1, -2, []string{"bug", "infra"}},
		{0, "Performance budget: LCP under 2.5s", "todo", "high", 0, 7, []string{"performance"}},
		{1, "Offline mode: sync queue", "in_progress", "critical", 0, 3, []string{"mobile", "sync"}},
		{1, "Push notification preferences screen", "todo", "medium", 1, 10, []string{"mobile", "ux"}},
		{1, "Crash on login with expired token", "in_review", "critical", 1, -1, []string{"bug"}},
		{1, "Dark mode support", "backlog", "low", -1, noDue, []string{"mobile", "design"}},
		{1, "Upgrade React Native to latest", "todo", "medium", 0, 20, []string{"mobile", "tech-debt"}},
		{1, "Release notes for v2.0", "backlog", "low", -1, noDue, []string{"content"}},
		{1, "Biometric login", "done", "high", 0, -12, []string{"security", "mobile"}},
		{1, "App store screenshots", "blocked", "medium", 1, 4, []string{"marketing"}},
		{2, "Provision new MongoDB cluster", "done", "high", 0, -30, []string{"infra"}},
		{2, "Write cutover runbook", "done", "medium", 1, -26, []string{"docs"}},
		{2, "Decommission old servers", "done", "low", 0, -20, []string{"infra"}},
		{3, "Remove unused CRM fields", "todo", "low", 0, 30, nil},
		{3, "Export historical contacts", "in_progress", "medium", 0, 15, nil},
		{4, "Model combustion efficiency", "in_progress", "high", 0, 6, []string{"research"}},
		{4, "Source new oxidizer supplier", "todo", "critical", 1, 3, []string{"procurement"}},
		{4, "Draft safety review", "backlog", "medium", -1, noDue, nil},
	}
	created := map[int]*model.Task{}
	for i, t := range tasks {
		p, ps := projects[t.project], specs[t.project]
		task := &model.Task{
			OrgID: p.OrgID, ProjectID: p.ID, Title: t.title, Description: "", Status: t.status, Priority: t.pri,
			ReporterID: ps.owner.ID, Labels: append([]string{}, t.labels...), CreatedAt: ago(24*15 - i*9),
		}
		if t.labels == nil {
			task.Labels = []string{}
		}
		if t.assignee >= 0 {
			task.AssigneeID = ptr(ps.members[t.assignee].ID)
		}
		if t.due != noDue {
			task.DueDate = ptr(today.AddDate(0, 0, t.due))
		}
		task.UpdatedAt = ago(24*14 - i*8)
		if t.status == model.StatusDone {
			task.CompletedAt = ptr(task.UpdatedAt)
		}
		must(r.Tasks.Create(ctx, task))
		created[i] = task

		acts := []*model.Activity{{OrgID: p.OrgID, ProjectID: &p.ID, TaskID: &task.ID, ActorID: ps.owner.ID, ActorName: ps.owner.Name, Action: "task.created", EntityTitle: task.Title, CreatedAt: task.CreatedAt}}
		if t.status != "todo" && t.status != "backlog" {
			actor := ps.members[max(t.assignee, 0)]
			acts = append(acts, &model.Activity{OrgID: p.OrgID, ProjectID: &p.ID, TaskID: &task.ID, ActorID: actor.ID, ActorName: actor.Name,
				Action: "task.status_changed", EntityTitle: task.Title, Meta: map[string]any{"from": "todo", "to": t.status}, CreatedAt: task.UpdatedAt})
		}
		must(r.Activity.Insert(ctx, acts...))
	}

	comment := func(taskIdx int, by *model.User, body string, hoursAgo int) {
		t := created[taskIdx]
		must(r.Comments.Create(ctx, &model.Comment{OrgID: t.OrgID, TaskID: t.ID, AuthorID: by.ID, AuthorName: by.Name, Body: body, CreatedAt: ago(hoursAgo)}))
		must(r.Activity.Insert(ctx, &model.Activity{OrgID: t.OrgID, ProjectID: &t.ProjectID, TaskID: &t.ID, ActorID: by.ID, ActorName: by.Name, Action: "comment.added", EntityTitle: t.Title, CreatedAt: ago(hoursAgo)}))
	}
	comment(2, alice, "Love the hero section. Can we try a variant with the product screenshot on the right?", 30)
	comment(2, carol, "Sure, I'll add a second frame to the Figma file by tomorrow.", 26)
	comment(8, bob, "Blocked on the CDN vendor: they need our account manager to re-enable the old bucket.", 20)
	comment(8, alice, "I've pinged them again. Escalating if there's no reply by Friday.", 6)
	comment(12, carol, "Reproduced on Android 14. The refresh token is read before the keychain unlocks.", 12)
	comment(10, alice, "Conflict resolution strategy: last-write-wins per field. Writing it up in the ADR.", 3)

	slog.Info("seed: demo data created", "users", 4, "orgs", 2, "projects", len(projects), "tasks", len(tasks))
	return nil
}

func join(ctx context.Context, r *repository.Repos, o *model.Organization, u *model.User, role model.Role) {
	must(r.Members.Create(ctx, &model.Membership{OrgID: o.ID, UserID: u.ID, Role: role}))
}

// must keeps the seeder linear; a failure aborts startup with a clear message.
func must(err error) {
	if err != nil {
		panic("seed: " + err.Error())
	}
}

func ptr[T any](v T) *T { return &v }
