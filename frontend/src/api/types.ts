export type Role = "owner" | "admin" | "member";
export type TaskStatus = "backlog" | "todo" | "in_progress" | "in_review" | "done" | "blocked";
export type Priority = "low" | "medium" | "high" | "critical";
export type ProjectStatus = "active" | "on_hold" | "completed" | "archived";

export interface User {
  id: string;
  name: string;
  email: string;
  created_at: string;
}

export interface Org {
  id: string;
  name: string;
  owner_id: string;
  created_at: string;
  role: Role; // the caller's role
}

export interface Member {
  user_id: string;
  name: string;
  email: string;
  role: Role;
  joined_at: string;
}

export interface Project {
  id: string;
  org_id: string;
  name: string;
  description: string;
  status: ProjectStatus;
  member_ids: string[];
  created_by: string;
  created_at: string;
  updated_at: string;
  archived_at: string | null;
  task_count: number;
  done_count: number;
}

export interface Task {
  id: string;
  org_id: string;
  project_id: string;
  project_name: string;
  title: string;
  description: string;
  status: TaskStatus;
  priority: Priority;
  assignee_id: string | null;
  reporter_id: string;
  due_date: string | null;
  labels: string[];
  completed_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface Comment {
  id: string;
  task_id: string;
  author_id: string;
  author_name: string;
  body: string;
  created_at: string;
}

export interface Activity {
  id: string;
  project_id: string | null;
  task_id: string | null;
  actor_id: string;
  actor_name: string;
  action: string;
  entity_title: string;
  meta?: Record<string, unknown>;
  created_at: string;
}

export interface Dashboard {
  total_projects: number;
  active_projects: number;
  total_tasks: number;
  active_tasks: number;
  completed_tasks: number;
  overdue_tasks: number;
  my_open_tasks: number;
  tasks_by_status: Record<TaskStatus, number>;
  tasks_by_priority: Partial<Record<Priority, number>>;
  recent_activity: Activity[];
}

export interface PageMeta {
  page: number;
  page_size: number;
  total: number;
  total_pages: number;
}

export interface Paged<T> {
  items: T[];
  meta: PageMeta;
}

export interface TaskParams {
  project_id?: string;
  status?: string;
  priority?: string;
  assignee?: string;
  label?: string;
  q?: string;
  overdue?: boolean;
  sort?: string;
  page?: number;
  page_size?: number;
}

export interface ProjectParams {
  q?: string;
  status?: string;
  sort?: string;
  page?: number;
  page_size?: number;
}

export interface TaskInput {
  title: string;
  description?: string;
  status?: TaskStatus;
  priority?: Priority;
  assignee_id?: string | null;
  due_date?: string | null;
  labels?: string[];
}
