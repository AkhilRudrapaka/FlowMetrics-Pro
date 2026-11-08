import type { Priority, ProjectStatus, Role, TaskStatus } from "../api/types";

// Full class strings (not built dynamically) so Tailwind can see and generate them.
export const STATUSES: { value: TaskStatus; label: string; dot: string; column: string }[] = [
  { value: "backlog", label: "Backlog", dot: "bg-slate-400", column: "border-t-slate-400" },
  { value: "todo", label: "To do", dot: "bg-sky-500", column: "border-t-sky-500" },
  { value: "in_progress", label: "In progress", dot: "bg-indigo-500", column: "border-t-indigo-500" },
  { value: "in_review", label: "In review", dot: "bg-violet-500", column: "border-t-violet-500" },
  { value: "done", label: "Done", dot: "bg-emerald-500", column: "border-t-emerald-500" },
  { value: "blocked", label: "Blocked", dot: "bg-rose-500", column: "border-t-rose-500" },
];

export const statusLabel = (s: string) => STATUSES.find((x) => x.value === s)?.label ?? s;

export const PRIORITIES: { value: Priority; label: string; rank: number; badge: string }[] = [
  { value: "critical", label: "Critical", rank: 4, badge: "bg-rose-100 text-rose-700 ring-rose-200" },
  { value: "high", label: "High", rank: 3, badge: "bg-orange-100 text-orange-700 ring-orange-200" },
  { value: "medium", label: "Medium", rank: 2, badge: "bg-amber-100 text-amber-700 ring-amber-200" },
  { value: "low", label: "Low", rank: 1, badge: "bg-slate-100 text-slate-600 ring-slate-200" },
];

export const priorityMeta = (p: string) => PRIORITIES.find((x) => x.value === p) ?? PRIORITIES[3];

export const PROJECT_STATUSES: { value: ProjectStatus; label: string; badge: string }[] = [
  { value: "active", label: "Active", badge: "bg-emerald-100 text-emerald-700 ring-emerald-200" },
  { value: "on_hold", label: "On hold", badge: "bg-amber-100 text-amber-700 ring-amber-200" },
  { value: "completed", label: "Completed", badge: "bg-sky-100 text-sky-700 ring-sky-200" },
  { value: "archived", label: "Archived", badge: "bg-slate-100 text-slate-600 ring-slate-200" },
];

export const projectStatusMeta = (s: string) => PROJECT_STATUSES.find((x) => x.value === s) ?? PROJECT_STATUSES[0];

export const ROLE_BADGE: Record<Role, string> = {
  owner: "bg-indigo-100 text-indigo-700 ring-indigo-200",
  admin: "bg-sky-100 text-sky-700 ring-sky-200",
  member: "bg-slate-100 text-slate-600 ring-slate-200",
};
