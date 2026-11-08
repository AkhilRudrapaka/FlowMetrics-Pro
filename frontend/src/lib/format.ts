import type { Activity, TaskStatus } from "../api/types";
import { statusLabel } from "./constants";

/** Due dates are calendar dates; the API compares them in UTC, so the UI does too. */
export const todayUTC = () => new Date().toISOString().slice(0, 10);

export function isOverdue(due: string | null, status: TaskStatus, today = todayUTC()) {
  return !!due && status !== "done" && due.slice(0, 10) < today;
}

export function formatDate(iso: string | null | undefined) {
  if (!iso) return "";
  return new Date(iso).toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric", timeZone: "UTC" });
}

export const toDateInput = (iso: string | null) => (iso ? iso.slice(0, 10) : "");

export function timeAgo(iso: string, now = Date.now()) {
  const s = Math.max(0, Math.round((now - new Date(iso).getTime()) / 1000));
  if (s < 45) return "just now";
  if (s < 3600) return `${Math.round(s / 60)}m ago`;
  if (s < 86400) return `${Math.round(s / 3600)}h ago`;
  if (s < 7 * 86400) return `${Math.round(s / 86400)}d ago`;
  return new Date(iso).toLocaleDateString(undefined, { month: "short", day: "numeric" });
}

export const initials = (name: string) =>
  name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((p) => p[0]!.toUpperCase())
    .join("");

/** One readable sentence per audit entry. */
export function describeActivity(a: Activity): string {
  const m = a.meta ?? {};
  const t = `"${a.entity_title}"`;
  switch (a.action) {
    case "project.created": return `created project ${t}`;
    case "project.updated": return `updated project ${t}`;
    case "project.archived": return `archived project ${t}`;
    case "project.member_added": return `added ${m.member} to project ${t}`;
    case "project.member_removed": return `removed ${m.member} from project ${t}`;
    case "task.created": return `created task ${t}`;
    case "task.deleted": return `deleted task ${t}`;
    case "task.status_changed": return `moved ${t} from ${statusLabel(String(m.from))} to ${statusLabel(String(m.to))}`;
    case "task.priority_changed": return `changed priority of ${t} from ${m.from} to ${m.to}`;
    case "task.assigned": return m.assignee ? `assigned ${t} to ${m.assignee}` : `unassigned ${t}`;
    case "task.updated": {
      const fields = (m.fields as string[] | undefined)?.join(", ");
      return fields ? `updated ${fields} on ${t}` : `updated ${t}`;
    }
    case "comment.added": return `commented on ${t}`;
    case "member.added": return `added ${a.entity_title} to the organization as ${m.role}`;
    case "member.removed": return `removed ${a.entity_title} from the organization`;
    case "member.role_changed": return `changed ${a.entity_title}'s role from ${m.from} to ${m.to}`;
    default: return a.action;
  }
}
