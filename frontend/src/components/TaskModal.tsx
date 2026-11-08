import { useEffect, useState, type FormEvent } from "react";
import { Link } from "react-router";
import { ApiError, errorMessage } from "../api/client";
import { useAddComment, useComments, useCreateTask, useDeleteComment, useDeleteTask, useHistory, useProject, useTask, useUpdateTask } from "../api/hooks";
import type { Priority, Project, Task, TaskStatus } from "../api/types";
import { PRIORITIES, STATUSES } from "../lib/constants";
import { describeActivity, formatDate, timeAgo, toDateInput } from "../lib/format";
import { useDebounced } from "../lib/hooks";
import { toast } from "../store/ui";
import { useOrgCtx } from "./OrgLayout";
import { Avatar, Button, ConfirmDialog, ErrorState, Field, Input, Modal, PageSpinner, Select, Textarea, cx } from "./ui";

// ---------------------------------------------------------------- shared bits

function LabelEditor({ labels, editable, onChange }: { labels: string[]; editable: boolean; onChange: (l: string[]) => void }) {
  const [draft, setDraft] = useState("");
  const add = () => {
    const v = draft.trim().toLowerCase();
    if (v && !labels.includes(v)) onChange([...labels, v]);
    setDraft("");
  };
  return (
    <div>
      <div className="flex flex-wrap gap-1.5">
        {labels.length === 0 && !editable && <span className="text-sm text-slate-400">None</span>}
        {labels.map((l) => (
          <span key={l} className="inline-flex items-center gap-1 rounded bg-slate-100 px-2 py-0.5 text-xs text-slate-700">
            {l}
            {editable && <button type="button" onClick={() => onChange(labels.filter((x) => x !== l))} aria-label={`Remove label ${l}`} className="text-slate-400 hover:text-rose-600">×</button>}
          </span>
        ))}
      </div>
      {editable && (
        <Input className="mt-2" value={draft} maxLength={30} placeholder="Add label, press Enter" aria-label="Add label"
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); add(); } }}
          onBlur={add} />
      )}
    </div>
  );
}

// ---------------------------------------------------------------- new task

export function NewTaskModal({ orgId, project, defaultStatus, onClose }: { orgId: string; project: Project; defaultStatus: TaskStatus; onClose: () => void }) {
  const { members } = useOrgCtx();
  const create = useCreateTask(orgId, project.id);
  const [f, setF] = useState({ title: "", description: "", status: defaultStatus, priority: "medium" as Priority, assignee: "", due: "" });
  const [labels, setLabels] = useState<string[]>([]);
  const errs = create.error instanceof ApiError ? (create.error.details ?? {}) : {};
  const team = members.filter((m) => project.member_ids.includes(m.user_id));

  function submit(e: FormEvent) {
    e.preventDefault();
    create.mutate(
      { title: f.title, description: f.description, status: f.status, priority: f.priority, assignee_id: f.assignee || null, due_date: f.due || null, labels },
      { onSuccess: () => { toast.success("Task created"); onClose(); } },
    );
  }

  return (
    <Modal title={`New task in ${project.name}`} onClose={onClose}>
      <form onSubmit={submit} className="space-y-4">
        <Field label="Title" error={errs.title}>{(id) => <Input id={id} value={f.title} maxLength={200} required autoFocus onChange={(e) => setF({ ...f, title: e.target.value })} />}</Field>
        <Field label="Description" error={errs.description}>{(id) => <Textarea id={id} rows={3} value={f.description} onChange={(e) => setF({ ...f, description: e.target.value })} />}</Field>
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label="Status">{(id) => <Select id={id} value={f.status} onChange={(e) => setF({ ...f, status: e.target.value as TaskStatus })}>{STATUSES.map((s) => <option key={s.value} value={s.value}>{s.label}</option>)}</Select>}</Field>
          <Field label="Priority">{(id) => <Select id={id} value={f.priority} onChange={(e) => setF({ ...f, priority: e.target.value as Priority })}>{[...PRIORITIES].reverse().map((p) => <option key={p.value} value={p.value}>{p.label}</option>)}</Select>}</Field>
          <Field label="Assignee" error={errs.assignee_id}>{(id) => <Select id={id} value={f.assignee} onChange={(e) => setF({ ...f, assignee: e.target.value })}><option value="">Unassigned</option>{team.map((m) => <option key={m.user_id} value={m.user_id}>{m.name}</option>)}</Select>}</Field>
          <Field label="Due date" error={errs.due_date}>{(id) => <Input id={id} type="date" value={f.due} onChange={(e) => setF({ ...f, due: e.target.value })} />}</Field>
        </div>
        <Field label="Labels" error={errs.labels}>{() => <LabelEditor labels={labels} editable onChange={setLabels} />}</Field>
        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="secondary" onClick={onClose}>Cancel</Button>
          <Button type="submit" loading={create.isPending}>Create task</Button>
        </div>
      </form>
    </Modal>
  );
}

// ---------------------------------------------------------------- comments & history

function Comments({ orgId, taskId, canWrite }: { orgId: string; taskId: string; canWrite: boolean }) {
  const { userId, isAdmin } = useOrgCtx();
  const list = useComments(orgId, taskId);
  const add = useAddComment(orgId, taskId);
  const del = useDeleteComment(orgId, taskId);
  const [body, setBody] = useState("");

  function submit(e: FormEvent) {
    e.preventDefault();
    add.mutate(body, { onSuccess: () => setBody("") });
  }
  return (
    <div>
      {list.isPending && <PageSpinner />}
      {list.isError && <ErrorState error={list.error} onRetry={() => list.refetch()} />}
      {list.data?.items.length === 0 && <p className="py-4 text-sm text-slate-500">No comments yet.</p>}
      <ul className="space-y-4">
        {list.data?.items.map((c) => (
          <li key={c.id} className="flex gap-3">
            <Avatar name={c.author_name} className="size-8" />
            <div className="min-w-0 flex-1">
              <p className="text-sm"><span className="font-medium text-slate-900">{c.author_name}</span> <span className="text-xs text-slate-400">{timeAgo(c.created_at)}</span></p>
              <p className="mt-0.5 whitespace-pre-wrap break-words text-sm text-slate-700">{c.body}</p>
              {(c.author_id === userId || isAdmin) && (
                <button onClick={() => del.mutate(c.id)} className="mt-1 text-xs text-slate-400 hover:text-rose-600">Delete</button>
              )}
            </div>
          </li>
        ))}
      </ul>
      {canWrite ? (
        <form onSubmit={submit} className="mt-5 space-y-2">
          <label htmlFor="new-comment" className="sr-only">Add a comment</label>
          <Textarea id="new-comment" rows={3} placeholder="Write a comment…" value={body} onChange={(e) => setBody(e.target.value)} maxLength={5000} />
          {add.error instanceof ApiError && add.error.status === 422 && <p role="alert" className="text-xs text-rose-600">{errorMessage(add.error)}</p>}
          <div className="flex justify-end"><Button type="submit" size="sm" loading={add.isPending} disabled={!body.trim()}>Comment</Button></div>
        </form>
      ) : (
        <p className="mt-5 text-xs text-slate-500">Only project members can comment.</p>
      )}
    </div>
  );
}

function History({ orgId, taskId }: { orgId: string; taskId: string }) {
  const q = useHistory(orgId, taskId, true);
  if (q.isPending) return <PageSpinner />;
  if (q.isError) return <ErrorState error={q.error} onRetry={() => q.refetch()} />;
  if (q.data.items.length === 0) return <p className="py-4 text-sm text-slate-500">No history yet.</p>;
  return (
    <ul className="space-y-3">
      {q.data.items.map((a) => (
        <li key={a.id} className="flex gap-3 text-sm">
          <Avatar name={a.actor_name} />
          <div>
            <p className="text-slate-700"><span className="font-medium text-slate-900">{a.actor_name}</span> {describeActivity(a)}</p>
            <p className="text-xs text-slate-400">{timeAgo(a.created_at)}</p>
          </div>
        </li>
      ))}
    </ul>
  );
}

// ---------------------------------------------------------------- task detail

function TaskDetail({ task, project, onClose }: { task: Task; project: Project; onClose: () => void }) {
  const ctx = useOrgCtx();
  const orgId = ctx.org.id;
  const update = useUpdateTask(orgId, task.id);
  const del = useDeleteTask(orgId);
  const [title, setTitle] = useState(task.title);
  const [description, setDescription] = useState(task.description);
  const [tab, setTab] = useState<"comments" | "history">("comments");
  const [confirmDelete, setConfirmDelete] = useState(false);

  const canWrite = ctx.canWrite(project);
  const canDelete = canWrite && (ctx.isAdmin || task.reporter_id === ctx.userId);
  const dirty = title.trim() !== task.title || description !== task.description;
  const team = ctx.members.filter((m) => project.member_ids.includes(m.user_id));
  const errs = update.error instanceof ApiError ? (update.error.details ?? {}) : {};
  const patch = (v: Parameters<typeof update.mutate>[0]) => update.mutate(v);

  // While a save is in flight, show the value the user just picked instead of snapping back to the
  // old one until the server answers. (An error clears `pending`, which reverts the display.)
  const pending = update.isPending ? update.variables : undefined;
  const shown = {
    status: pending?.status ?? task.status,
    priority: pending?.priority ?? task.priority,
    assignee_id: pending && "assignee_id" in pending ? (pending.assignee_id ?? null) : task.assignee_id,
    labels: pending?.labels ?? task.labels,
  };

  // Typing into a date input fires change events for every partial year (0002, 0020, 0202...).
  // Wait until the user pauses, and ignore anything that isn't a plausible full date.
  const [due, setDue] = useState(toDateInput(task.due_date));
  const settledDue = useDebounced(due, 700);
  useEffect(() => {
    const saved = toDateInput(task.due_date);
    if (settledDue === saved) return;
    if (settledDue === "" || (/^\d{4}-\d{2}-\d{2}$/.test(settledDue) && Number(settledDue.slice(0, 4)) >= 1900)) {
      update.mutate({ due_date: settledDue || null });
    }
  }, [settledDue]); // deliberately keyed on the settled edit only, not on every task refetch

  return (
    <>
      <div className="grid gap-6 lg:grid-cols-3">
        <div className="min-w-0 lg:col-span-2">
          {canWrite ? (
            <div className="space-y-3">
              <Field label="Title" error={errs.title}>{(id) => <Input id={id} value={title} maxLength={200} onChange={(e) => setTitle(e.target.value)} className="text-base font-semibold" />}</Field>
              <Field label="Description" error={errs.description}>{(id) => <Textarea id={id} rows={5} value={description} placeholder="Add more detail…" onChange={(e) => setDescription(e.target.value)} />}</Field>
              {dirty && (
                <div className="flex gap-2">
                  <Button size="sm" loading={update.isPending} onClick={() => update.mutate({ title, description }, { onSuccess: () => toast.success("Task saved") })}>Save changes</Button>
                  <Button size="sm" variant="secondary" onClick={() => { setTitle(task.title); setDescription(task.description); }}>Discard</Button>
                </div>
              )}
            </div>
          ) : (
            <>
              <h3 className="text-lg font-semibold text-slate-900">{task.title}</h3>
              <p className="mt-2 whitespace-pre-wrap text-sm text-slate-700">{task.description || <span className="text-slate-400">No description.</span>}</p>
            </>
          )}

          <div className="mt-8 border-b border-slate-200" role="tablist" aria-label="Task activity">
            {(["comments", "history"] as const).map((t) => (
              <button key={t} role="tab" aria-selected={tab === t} onClick={() => setTab(t)}
                className={cx("-mb-px mr-4 border-b-2 px-1 pb-2 text-sm font-medium capitalize", tab === t ? "border-indigo-600 text-indigo-700" : "border-transparent text-slate-500 hover:text-slate-700")}>
                {t}
              </button>
            ))}
          </div>
          <div className="mt-4" role="tabpanel">
            {tab === "comments" ? <Comments orgId={orgId} taskId={task.id} canWrite={canWrite} /> : <History orgId={orgId} taskId={task.id} />}
          </div>
        </div>

        <aside className="space-y-4 rounded-xl bg-slate-50 p-4 lg:self-start">
          {!canWrite && <p className="rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-800 ring-1 ring-amber-200">{project.status === "archived" ? "This project is archived (read-only)." : "You're not a member of this project, so this task is read-only."}</p>}
          {update.isError && !errs.title && !errs.description && <p role="alert" className="rounded-lg bg-rose-50 px-3 py-2 text-xs text-rose-700">{errorMessage(update.error)}</p>}
          <Field label="Status">{(id) => <Select id={id} disabled={!canWrite} value={shown.status} onChange={(e) => patch({ status: e.target.value as TaskStatus })}>{STATUSES.map((s) => <option key={s.value} value={s.value}>{s.label}</option>)}</Select>}</Field>
          <Field label="Priority">{(id) => <Select id={id} disabled={!canWrite} value={shown.priority} onChange={(e) => patch({ priority: e.target.value as Priority })}>{[...PRIORITIES].reverse().map((p) => <option key={p.value} value={p.value}>{p.label}</option>)}</Select>}</Field>
          <Field label="Assignee">{(id) => (
            <Select id={id} disabled={!canWrite} value={shown.assignee_id ?? ""} onChange={(e) => patch({ assignee_id: e.target.value || null })}>
              <option value="">Unassigned</option>
              {team.map((m) => <option key={m.user_id} value={m.user_id}>{m.name}</option>)}
            </Select>
          )}</Field>
          <Field label="Due date">{(id) => <Input id={id} type="date" disabled={!canWrite} value={due} onChange={(e) => setDue(e.target.value)} />}</Field>
          <Field label="Labels" error={errs.labels}>{() => <LabelEditor labels={shown.labels} editable={canWrite} onChange={(labels) => patch({ labels })} />}</Field>
          <dl className="space-y-1 border-t border-slate-200 pt-3 text-xs text-slate-500">
            <div className="flex justify-between gap-2"><dt>Project</dt><dd className="truncate"><Link to={`/o/${orgId}/projects/${project.id}`} onClick={onClose} className="font-medium text-indigo-600 hover:underline">{project.name}</Link></dd></div>
            <div className="flex justify-between gap-2"><dt>Reporter</dt><dd>{ctx.nameOf(task.reporter_id)}</dd></div>
            <div className="flex justify-between gap-2"><dt>Created</dt><dd>{formatDate(task.created_at)}</dd></div>
            <div className="flex justify-between gap-2"><dt>Updated</dt><dd>{timeAgo(task.updated_at)}</dd></div>
          </dl>
          {canDelete && <Button variant="danger" size="sm" className="w-full" onClick={() => setConfirmDelete(true)}>Delete task</Button>}
        </aside>
      </div>
      {confirmDelete && (
        <ConfirmDialog title="Delete this task?" confirmLabel="Delete task" loading={del.isPending} onClose={() => setConfirmDelete(false)}
          body={<>“{task.title}” and its comments will be permanently removed. This can't be undone.</>}
          onConfirm={() => del.mutate(task.id, { onSuccess: () => { toast.success("Task deleted"); onClose(); } })} />
      )}
    </>
  );
}

export default function TaskModal({ taskId, onClose }: { taskId: string; onClose: () => void }) {
  const { org } = useOrgCtx();
  const taskQ = useTask(org.id, taskId);
  const projectQ = useProject(org.id, taskQ.data?.project_id ?? "");
  return (
    <Modal title="Task" onClose={onClose} wide>
      {(taskQ.isPending || (taskQ.isSuccess && projectQ.isPending)) && <PageSpinner />}
      {taskQ.isError && <ErrorState error={taskQ.error} onRetry={() => taskQ.refetch()} />}
      {projectQ.isError && <ErrorState error={projectQ.error} onRetry={() => projectQ.refetch()} />}
      {taskQ.data && projectQ.data && <TaskDetail key={taskQ.data.id} task={taskQ.data} project={projectQ.data} onClose={onClose} />}
    </Modal>
  );
}
