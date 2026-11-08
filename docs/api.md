# API reference

Base URL: `/api/v1`. All bodies are JSON. Ids are 24-character hex strings. Timestamps are RFC 3339 (UTC). Due dates are calendar dates (`YYYY-MM-DD`; a timestamp is truncated to its UTC date).

## Conventions

**Authentication.** Send `Authorization: Bearer <access_token>` on every route except `/auth/*` and `/healthz`. The refresh token travels only in the `fm_refresh` cookie.

**Success envelope**

```json
{ "data": { ... } }
{ "data": [ ... ], "meta": { "page": 1, "page_size": 20, "total": 57, "total_pages": 3 } }
```

Deletes and role changes return `204 No Content`.

**Error envelope**

```json
{ "error": { "code": "validation_error", "message": "one or more fields are invalid",
             "details": { "title": "is required", "due_date": "must be a date in YYYY-MM-DD format" } } }
```

| HTTP | `code` | Meaning |
| --- | --- | --- |
| 400 | `bad_request` | Malformed JSON, or an unknown field in the body |
| 401 | `unauthorized` | Missing/expired token, wrong credentials, or revoked session |
| 403 | `forbidden` | You're a member, but your role or project membership doesn't allow this |
| 404 | `not_found` | Doesn't exist **or isn't in an organization you belong to** (deliberately indistinguishable) |
| 409 | `conflict` | Duplicate email/membership, archived (read-only) project, owner can't leave |
| 422 | `validation_error` | Field-level problems, listed together in `details` |
| 429 | `rate_limited` | Too many login/register attempts (`Retry-After` header set) |
| 500 | `internal_error` | A bug or outage; details are in the server log, never the response |

**Pagination.** `page` (default 1) and `page_size` (default 20, max 100) on every list. Out-of-range pages return an empty `data` array.

## Auth

| Method & path | Body | Result |
| --- | --- | --- |
| `POST /auth/register` | `{name, email, password}` (password 8–72 bytes) | `201` `{user, access_token, expires_at}` + sets `fm_refresh` cookie |
| `POST /auth/login` | `{email, password}` | `200` same shape. Wrong email and wrong password give the same `401` |
| `POST /auth/refresh` | none (cookie) | `200` new access token + **rotated** cookie. A spent token returns `401` and may revoke the session |
| `POST /auth/logout` | none (cookie) | `204`; revokes the session, clears the cookie. Idempotent |
| `GET /auth/me` | | `200` the current user |

## Organizations

| Method & path | Who | Body / notes |
| --- | --- | --- |
| `GET /orgs` | any user | Your organizations, each with your `role` |
| `POST /orgs` | any user | `{name}` (2–80 chars). You become the owner |
| `GET /orgs/:orgId` | member | |
| `PATCH /orgs/:orgId` | owner, admin | `{name}` |
| `DELETE /orgs/:orgId` | owner | Permanently deletes every project, task, comment, and membership |

Everything below is under `/orgs/:orgId` and requires membership (otherwise `404`).

## Members

| Method & path | Who | Body / notes |
| --- | --- | --- |
| `GET /members` | member | `[{user_id, name, email, role, joined_at}]` |
| `POST /members` | owner (admin or member), admin (member only) | `{email, role}`. The user must already be registered |
| `PATCH /members/:userId` | owner | `{role: "admin" \| "member"}`. The owner's role can't change |
| `DELETE /members/:userId` | owner (anyone but self), admin (members), anyone (themselves, except the owner) | Also removes them from all projects and unassigns their tasks |

## Projects

| Method & path | Who | Body / notes |
| --- | --- | --- |
| `GET /projects` | member | Query: `q`, `status` (`active\|on_hold\|completed\|archived`; omitted = all except archived), `sort` (`name`, `created_at`, `updated_at`; prefix `-` for descending), `page`, `page_size`. Each item includes `task_count` and `done_count` |
| `POST /projects` | owner, admin | `{name (1–100), description?, member_ids?}`. You're added to the team automatically. All `member_ids` must belong to the org |
| `GET /projects/:projectId` | member | |
| `PATCH /projects/:projectId` | owner, admin | `{name?, description?, status?}`. An archived project only accepts a status change (restore) |
| `POST /projects/:projectId/archive` | owner, admin | Makes the project read-only and removes its tasks from org-wide lists and dashboard totals |
| `PUT /projects/:projectId/members/:userId` | owner, admin | Add a project member (must be an org member) |
| `DELETE /projects/:projectId/members/:userId` | owner, admin | Also unassigns that user's tasks in the project |

## Tasks

| Method & path | Who | Body / notes |
| --- | --- | --- |
| `POST /projects/:projectId/tasks` | project members, owner, admin | `{title (1–200), description?, status? (default todo), priority? (default medium), assignee_id?, due_date?, labels?}` |
| `GET /tasks` | member | See filters below |
| `GET /tasks/:taskId` | member | Includes `project_name` |
| `PATCH /tasks/:taskId` | project members, owner, admin | Any subset of the create fields. `assignee_id` and `due_date` accept **`null` to clear**; omitting a field leaves it unchanged |
| `DELETE /tasks/:taskId` | the reporter, owner, admin | Also deletes its comments |

Task enums: `status` ∈ `backlog, todo, in_progress, in_review, done, blocked`; `priority` ∈ `low, medium, high, critical`. The assignee must be a member of the task's project. Labels are lower-cased, de-duplicated, max 10 of 30 characters each. Moving a task to `done` sets `completed_at`; moving it out clears it.

**`GET /tasks` query parameters**

| Param | Example | Meaning |
| --- | --- | --- |
| `project_id` | `...` | One project (including an archived one). Without it, archived projects are excluded |
| `status`, `priority` | `todo,in_progress` | Comma-separated (or repeated) |
| `assignee` | `me`, `unassigned`, or a user id | |
| `label` | `bug` | Exact label |
| `q` | `login` | Case-insensitive substring in title or description (max 100 chars; matched literally) |
| `overdue` | `true` | Due before today (UTC) and not done |
| `sort` | `-updated_at` | `created_at`, `updated_at`, `due_date`, `title`; prefix `-` for descending. Default `-updated_at` |
| `page`, `page_size` | `2`, `15` | |

## Comments and history

| Method & path | Who | Body / notes |
| --- | --- | --- |
| `GET /tasks/:taskId/comments` | member | Oldest first. Default `page_size` 50 |
| `POST /tasks/:taskId/comments` | project members, owner, admin | `{body (1–5000)}` |
| `DELETE /tasks/:taskId/comments/:commentId` | author, owner, admin | |
| `GET /tasks/:taskId/activity` | member | Newest first: `task.created`, `task.status_changed`, `task.assigned`, `task.priority_changed`, `task.updated`, `comment.added` |

## Dashboard and activity

| Method & path | Notes |
| --- | --- |
| `GET /dashboard` | `{total_projects, active_projects, total_tasks, active_tasks, completed_tasks, overdue_tasks, my_open_tasks, tasks_by_status, tasks_by_priority, recent_activity[10]}` |
| `GET /activity` | The organization-wide audit feed, newest first, paginated |

## Health

`GET /healthz` (no auth): `200 {"status":"ok"}` when MongoDB answers a ping, otherwise `503`.

## Example

```bash
# log in (stores the refresh cookie in a jar) and capture the access token
TOKEN=$(curl -s -c jar.txt localhost:3000/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@acme.test","password":"Password123!"}' | jq -r .data.access_token)

# list my organizations, then overdue tasks in the first one
ORG=$(curl -s localhost:3000/api/v1/orgs -H "Authorization: Bearer $TOKEN" | jq -r '.data[0].id')
curl -s "localhost:3000/api/v1/orgs/$ORG/tasks?overdue=true&sort=due_date" -H "Authorization: Bearer $TOKEN" | jq '.meta, .data[].title'
```
