import { keepPreviousData, useMutation, useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query";
import { api } from "./client";
import type {
  Activity, Comment, Dashboard, Member, Org, Paged, Project, ProjectParams, Role, Task, TaskInput, TaskParams, TaskStatus,
} from "./types";

// Query keys for tenant data start with the org id, so switching organizations can never
// show another org's cached data, and one predicate can refresh everything for an org.
const k = {
  orgs: ["orgs"] as const,
  org: (o: string) => [o, "org"] as const,
  members: (o: string) => [o, "members"] as const,
  dashboard: (o: string) => [o, "dashboard"] as const,
  activity: (o: string, p: number) => [o, "activity", p] as const,
  projects: (o: string, p: ProjectParams) => [o, "projects", p] as const,
  project: (o: string, id: string) => [o, "project", id] as const,
  tasks: (o: string, p: TaskParams) => [o, "tasks", p] as const,
  task: (o: string, id: string) => [o, "task", id] as const,
  comments: (o: string, id: string) => [o, "comments", id] as const,
  history: (o: string, id: string) => [o, "history", id] as const,
};

/** Refresh every cached query of an org except the slow-changing org/members. */
function refreshWork(qc: QueryClient, orgId: string) {
  return qc.invalidateQueries({ predicate: (q) => q.queryKey[0] === orgId && q.queryKey[1] !== "org" && q.queryKey[1] !== "members" });
}

const base = (o: string) => `/orgs/${o}`;

// ---- organizations & members ----
export const useOrgs = () => useQuery({ queryKey: k.orgs, queryFn: () => api.get<Org[]>("/orgs") });
export const useOrg = (o: string) => useQuery({ queryKey: k.org(o), queryFn: () => api.get<Org>(base(o)), retry: false });
export const useMembers = (o: string) => useQuery({ queryKey: k.members(o), queryFn: () => api.get<Member[]>(`${base(o)}/members`) });

export function useCreateOrg() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => api.post<Org>("/orgs", { name }),
    onSuccess: () => qc.invalidateQueries({ queryKey: k.orgs }),
  });
}

export function useRenameOrg(o: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => api.patch<Org>(base(o), { name }),
    onSuccess: () => Promise.all([qc.invalidateQueries({ queryKey: k.orgs }), qc.invalidateQueries({ queryKey: k.org(o) })]),
  });
}

export function useDeleteOrg(o: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.del(base(o)),
    onSuccess: () => {
      qc.removeQueries({ predicate: (q) => q.queryKey[0] === o });
      return qc.invalidateQueries({ queryKey: k.orgs });
    },
  });
}

export function useAddMember(o: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (v: { email: string; role: Role }) => api.post<Member>(`${base(o)}/members`, v),
    onSuccess: () => Promise.all([qc.invalidateQueries({ queryKey: k.members(o) }), refreshWork(qc, o)]),
  });
}

export function useUpdateMemberRole(o: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (v: { userId: string; role: Role }) => api.patch<void>(`${base(o)}/members/${v.userId}`, { role: v.role }),
    onSuccess: () => Promise.all([qc.invalidateQueries({ queryKey: k.members(o) }), refreshWork(qc, o)]),
  });
}

export function useRemoveMember(o: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (userId: string) => api.del(`${base(o)}/members/${userId}`),
    onSuccess: () => Promise.all([qc.invalidateQueries({ queryKey: k.members(o) }), qc.invalidateQueries({ queryKey: k.orgs }), refreshWork(qc, o)]),
  });
}

// ---- dashboard ----
export const useDashboard = (o: string) => useQuery({ queryKey: k.dashboard(o), queryFn: () => api.get<Dashboard>(`${base(o)}/dashboard`) });

export const useOrgActivity = (o: string, page: number) =>
  useQuery({
    queryKey: k.activity(o, page),
    queryFn: () => api.page<Activity>(`${base(o)}/activity`, { page, page_size: 20 }),
    placeholderData: keepPreviousData,
  });

// ---- projects ----
export const useProjects = (o: string, p: ProjectParams) =>
  useQuery({ queryKey: k.projects(o, p), queryFn: () => api.page<Project>(`${base(o)}/projects`, { ...p }), placeholderData: keepPreviousData });

export const useProject = (o: string, id: string) =>
  useQuery({ queryKey: k.project(o, id), queryFn: () => api.get<Project>(`${base(o)}/projects/${id}`), enabled: !!id, retry: false });

export function useCreateProject(o: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (v: { name: string; description: string; member_ids: string[] }) => api.post<Project>(`${base(o)}/projects`, v),
    onSuccess: () => refreshWork(qc, o),
  });
}

export function useUpdateProject(o: string, id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (v: { name?: string; description?: string; status?: string }) => api.patch<Project>(`${base(o)}/projects/${id}`, v),
    onSuccess: () => refreshWork(qc, o),
  });
}

export function useProjectMember(o: string, id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (v: { userId: string; add: boolean }) =>
      v.add ? api.put<Project>(`${base(o)}/projects/${id}/members/${v.userId}`) : api.del(`${base(o)}/projects/${id}/members/${v.userId}`),
    onSuccess: () => refreshWork(qc, o),
  });
}

// ---- tasks ----
export const useTasks = (o: string, p: TaskParams) =>
  useQuery({ queryKey: k.tasks(o, p), queryFn: () => api.page<Task>(`${base(o)}/tasks`, { ...p }), placeholderData: keepPreviousData });

export const useTask = (o: string, id: string | null) =>
  useQuery({ queryKey: k.task(o, id ?? ""), queryFn: () => api.get<Task>(`${base(o)}/tasks/${id}`), enabled: !!id, retry: false });

export function useCreateTask(o: string, projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (v: TaskInput) => api.post<Task>(`${base(o)}/projects/${projectId}/tasks`, v),
    onSuccess: () => refreshWork(qc, o),
  });
}

export function useUpdateTask(o: string, id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (v: Partial<TaskInput>) => api.patch<Task>(`${base(o)}/tasks/${id}`, v),
    onSuccess: () => refreshWork(qc, o),
  });
}

export function useDeleteTask(o: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.del(`${base(o)}/tasks/${id}`),
    onSuccess: () => refreshWork(qc, o),
  });
}

/** Kanban move: the card jumps columns instantly; the cache rolls back if the server refuses. */
export function useMoveTask(o: string) {
  const qc = useQueryClient();
  const key = [o, "tasks"];
  return useMutation({
    mutationFn: (v: { id: string; status: TaskStatus }) => api.patch<Task>(`${base(o)}/tasks/${v.id}`, { status: v.status }),
    onMutate: async (v) => {
      await qc.cancelQueries({ queryKey: key });
      const snapshots = qc.getQueriesData<Paged<Task>>({ queryKey: key });
      qc.setQueriesData<Paged<Task>>({ queryKey: key }, (old) =>
        old && { ...old, items: old.items.map((t) => (t.id === v.id ? { ...t, status: v.status } : t)) },
      );
      return { snapshots };
    },
    onError: (_e, _v, ctx) => ctx?.snapshots.forEach(([k, data]) => qc.setQueryData(k, data)),
    onSettled: () => refreshWork(qc, o),
  });
}

// ---- comments & history ----
export const useComments = (o: string, taskId: string) =>
  useQuery({ queryKey: k.comments(o, taskId), queryFn: () => api.page<Comment>(`${base(o)}/tasks/${taskId}/comments`, { page_size: 100 }) });

export const useHistory = (o: string, taskId: string, enabled: boolean) =>
  useQuery({ queryKey: k.history(o, taskId), queryFn: () => api.page<Activity>(`${base(o)}/tasks/${taskId}/activity`, { page_size: 100 }), enabled });

export function useAddComment(o: string, taskId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: string) => api.post<Comment>(`${base(o)}/tasks/${taskId}/comments`, { body }),
    onSuccess: () => refreshWork(qc, o),
  });
}

export function useDeleteComment(o: string, taskId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.del(`${base(o)}/tasks/${taskId}/comments/${id}`),
    onSuccess: () => refreshWork(qc, o),
  });
}
