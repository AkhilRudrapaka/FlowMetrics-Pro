import { useState, type FormEvent } from "react";
import { Link, useParams, useSearchParams } from "react-router";
import { ApiError } from "../api/client";
import { useMoveTask, useProject, useProjectMember, useTasks, useUpdateProject } from "../api/hooks";
import type { Project, TaskStatus } from "../api/types";
import Kanban from "../components/Kanban";
import { useOrgCtx } from "../components/OrgLayout";
import TaskModal, { NewTaskModal } from "../components/TaskModal";
import { Avatar, Badge, Button, ConfirmDialog, ErrorState, Field, Input, Modal, PageSpinner, Select, Skeleton, Textarea, cx } from "../components/ui";
import { PRIORITIES, projectStatusMeta } from "../lib/constants";
import { useDebounced } from "../lib/hooks";
import { toast } from "../store/ui";

function EditProjectModal({ orgId, project, onClose }: { orgId: string; project: Project; onClose: () => void }) {
  const update = useUpdateProject(orgId, project.id);
  const [name, setName] = useState(project.name);
  const [description, setDescription] = useState(project.description);
  const [status, setStatus] = useState(project.status);
  const errs = update.error instanceof ApiError ? (update.error.details ?? {}) : {};
  const submit = (e: FormEvent) => {
    e.preventDefault();
    update.mutate({ name, description, status }, { onSuccess: () => { toast.success("Project updated"); onClose(); } });
  };
  return (
    <Modal title="Edit project" onClose={onClose}>
      <form onSubmit={submit} className="space-y-4">
        <Field label="Name" error={errs.name}>{(id) => <Input id={id} value={name} onChange={(e) => setName(e.target.value)} required maxLength={100} autoFocus />}</Field>
        <Field label="Description" error={errs.description}>{(id) => <Textarea id={id} rows={4} value={description} onChange={(e) => setDescription(e.target.value)} />}</Field>
        <Field label="Status" error={errs.status} hint="Use Archive on the project page to freeze a project.">
          {(id) => (
            <Select id={id} value={status} onChange={(e) => setStatus(e.target.value as Project["status"])}>
              {(["active", "on_hold", "completed"] as const).map((s) => <option key={s} value={s}>{projectStatusMeta(s).label}</option>)}
            </Select>
          )}
        </Field>
        <div className="flex justify-end gap-2">
          <Button type="button" variant="secondary" onClick={onClose}>Cancel</Button>
          <Button type="submit" loading={update.isPending}>Save</Button>
        </div>
      </form>
    </Modal>
  );
}

function Team({ orgId, project }: { orgId: string; project: Project }) {
  const { members, isAdmin } = useOrgCtx();
  const mutate = useProjectMember(orgId, project.id);
  const [pick, setPick] = useState("");
  const inProject = members.filter((m) => project.member_ids.includes(m.user_id));
  const available = members.filter((m) => !project.member_ids.includes(m.user_id));
  const frozen = project.status === "archived";

  return (
    <div className="max-w-2xl">
      {isAdmin && !frozen && (
        <form className="mb-5 flex items-end gap-2" onSubmit={(e) => { e.preventDefault(); if (pick) mutate.mutate({ userId: pick, add: true }, { onSuccess: () => setPick("") }); }}>
          <div className="flex-1">
            <Field label="Add a team member" hint="Only people already in this organization can be added.">
              {(id) => (
                <Select id={id} value={pick} onChange={(e) => setPick(e.target.value)} disabled={available.length === 0}>
                  <option value="">{available.length ? "Choose a person…" : "Everyone is already on this project"}</option>
                  {available.map((m) => <option key={m.user_id} value={m.user_id}>{m.name} ({m.email})</option>)}
                </Select>
              )}
            </Field>
          </div>
          <Button type="submit" disabled={!pick} loading={mutate.isPending}>Add</Button>
        </form>
      )}
      <ul className="divide-y divide-slate-100 rounded-xl bg-white ring-1 ring-slate-200">
        {inProject.map((m) => (
          <li key={m.user_id} className="flex items-center gap-3 px-4 py-3">
            <Avatar name={m.name} className="size-9" />
            <div className="min-w-0 flex-1"><p className="truncate text-sm font-medium text-slate-900">{m.name}</p><p className="truncate text-xs text-slate-500">{m.email}</p></div>
            {isAdmin && !frozen && <Button variant="ghost" size="sm" onClick={() => mutate.mutate({ userId: m.user_id, add: false })}>Remove</Button>}
          </li>
        ))}
        {inProject.length === 0 && <li className="px-4 py-6 text-center text-sm text-slate-500">No members yet.</li>}
      </ul>
      <p className="mt-3 text-xs text-slate-500">Removing someone unassigns their tasks in this project. Owners and admins can always work in every project.</p>
    </div>
  );
}

export default function ProjectPage() {
  const { projectId = "" } = useParams();
  const { org, isAdmin, canWrite, userId, members } = useOrgCtx();
  const [sp, setSp] = useSearchParams();
  const openTask = sp.get("task");
  const [tab, setTab] = useState<"board" | "team">("board");
  const [search, setSearch] = useState("");
  const [assignee, setAssignee] = useState("");
  const [priority, setPriority] = useState("");
  const [creating, setCreating] = useState<TaskStatus | null>(null);
  const [editing, setEditing] = useState(false);
  const [archiving, setArchiving] = useState(false);

  const q = useDebounced(search.trim());
  const projectQ = useProject(org.id, projectId);
  const tasksQ = useTasks(org.id, { project_id: projectId, q, assignee, priority, sort: "-updated_at", page_size: 100 });
  const move = useMoveTask(org.id);
  const update = useUpdateProject(org.id, projectId);

  if (projectQ.isPending) return <PageSpinner label="Loading project…" />;
  if (projectQ.isError) return <ErrorState error={projectQ.error} onRetry={() => projectQ.refetch()} />;
  const project = projectQ.data;
  const writable = canWrite(project);
  const meta = projectStatusMeta(project.status);
  const closeTask = () => setSp((p) => { p.delete("task"); return p; }, { replace: true });
  const filtered = !!(q || assignee || priority);

  return (
    <>
      <nav aria-label="Breadcrumb" className="mb-3 text-sm"><Link to={`/o/${org.id}/projects`} className="text-slate-500 hover:text-indigo-600">← Projects</Link></nav>
      <div className="mb-5 flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 max-w-3xl">
          <div className="flex flex-wrap items-center gap-3">
            <h1 className="truncate text-2xl font-semibold tracking-tight">{project.name}</h1>
            <Badge className={meta.badge}>{meta.label}</Badge>
          </div>
          {project.description && <p className="mt-1.5 text-sm text-slate-600">{project.description}</p>}
          <p className="mt-1.5 text-xs text-slate-500">{project.done_count} of {project.task_count} tasks done · {project.member_ids.length} member{project.member_ids.length === 1 ? "" : "s"}</p>
        </div>
        <div className="flex flex-wrap gap-2">
          {writable && <Button onClick={() => setCreating("todo")}>New task</Button>}
          {isAdmin && project.status !== "archived" && <Button variant="secondary" onClick={() => setEditing(true)}>Edit</Button>}
          {isAdmin && project.status !== "archived" && <Button variant="secondary" onClick={() => setArchiving(true)}>Archive</Button>}
          {isAdmin && project.status === "archived" && <Button variant="secondary" loading={update.isPending} onClick={() => update.mutate({ status: "active" }, { onSuccess: () => toast.success("Project restored") })}>Restore</Button>}
        </div>
      </div>

      {project.status === "archived" && <p className="mb-4 rounded-lg bg-slate-100 px-4 py-2.5 text-sm text-slate-600">This project is archived and read-only. Its tasks are excluded from dashboard totals.</p>}
      {!writable && project.status !== "archived" && <p className="mb-4 rounded-lg bg-amber-50 px-4 py-2.5 text-sm text-amber-800 ring-1 ring-amber-200">You can view this project, but only its members (and org admins) can change tasks.</p>}

      <div className="mb-4 flex gap-1 border-b border-slate-200" role="tablist">
        {(["board", "team"] as const).map((t) => (
          <button key={t} role="tab" aria-selected={tab === t} onClick={() => setTab(t)} className={cx("-mb-px border-b-2 px-3 pb-2 text-sm font-medium", tab === t ? "border-indigo-600 text-indigo-700" : "border-transparent text-slate-500 hover:text-slate-700")}>
            {t === "board" ? "Board" : `Team (${project.member_ids.length})`}
          </button>
        ))}
      </div>

      {tab === "team" ? <Team orgId={org.id} project={project} /> : (
        <>
          <div className="mb-4 flex flex-wrap gap-3">
            <div className="min-w-48 flex-1"><label htmlFor="tsearch" className="sr-only">Search tasks</label><Input id="tsearch" type="search" placeholder="Search tasks…" value={search} onChange={(e) => setSearch(e.target.value)} /></div>
            <label htmlFor="tassignee" className="sr-only">Assignee</label>
            <Select id="tassignee" className="w-44" value={assignee} onChange={(e) => setAssignee(e.target.value)}>
              <option value="">Anyone</option><option value="me">Assigned to me</option><option value="unassigned">Unassigned</option>
              {members.filter((m) => project.member_ids.includes(m.user_id) && m.user_id !== userId).map((m) => <option key={m.user_id} value={m.user_id}>{m.name}</option>)}
            </Select>
            <label htmlFor="tprio" className="sr-only">Priority</label>
            <Select id="tprio" className="w-40" value={priority} onChange={(e) => setPriority(e.target.value)}>
              <option value="">Any priority</option>{PRIORITIES.map((p) => <option key={p.value} value={p.value}>{p.label}</option>)}
            </Select>
            {filtered && <Button variant="ghost" onClick={() => { setSearch(""); setAssignee(""); setPriority(""); }}>Clear filters</Button>}
          </div>

          {tasksQ.isPending && <div className="grid grid-cols-3 gap-3 lg:grid-cols-6">{[0, 1, 2, 3, 4, 5].map((i) => <Skeleton key={i} className="h-64" />)}</div>}
          {tasksQ.isError && <ErrorState error={tasksQ.error} onRetry={() => tasksQ.refetch()} />}
          {tasksQ.data && (
            <>
              {tasksQ.data.items.length === 0 && !filtered && <p className="mb-4 rounded-lg bg-white px-4 py-3 text-sm text-slate-600 ring-1 ring-slate-200">This project has no tasks yet. {writable ? "Use “New task” or the + on any column to add the first one." : ""}</p>}
              {tasksQ.data.items.length === 0 && filtered && <p className="mb-4 text-sm text-slate-500">No tasks match your filters.</p>}
              {tasksQ.data.meta.total > tasksQ.data.items.length && <p className="mb-3 text-xs text-amber-700">Showing the {tasksQ.data.items.length} most recently updated of {tasksQ.data.meta.total} tasks. Narrow with filters to find the rest.</p>}
              <Kanban
                tasks={tasksQ.data.items}
                canDrag={writable}
                onOpen={(id) => setSp((p) => { p.set("task", id); return p; })}
                onMove={(id, status) => move.mutate({ id, status })}
                onAdd={writable ? setCreating : undefined}
              />
              {writable && <p className="mt-1 text-xs text-slate-400">Tip: drag cards between columns, or open a card to change its status.</p>}
            </>
          )}
        </>
      )}

      {openTask && <TaskModal taskId={openTask} onClose={closeTask} />}
      {creating && <NewTaskModal orgId={org.id} project={project} defaultStatus={creating} onClose={() => setCreating(null)} />}
      {editing && <EditProjectModal orgId={org.id} project={project} onClose={() => setEditing(false)} />}
      {archiving && (
        <ConfirmDialog title="Archive this project?" confirmLabel="Archive" loading={update.isPending} onClose={() => setArchiving(false)}
          body="Archived projects become read-only and their tasks stop counting toward dashboard totals. You can restore it later."
          onConfirm={() => update.mutate({ status: "archived" }, { onSuccess: () => { toast.success("Project archived"); setArchiving(false); } })} />
      )}
    </>
  );
}
