import { useState, type FormEvent } from "react";
import { Link } from "react-router";
import { ApiError } from "../api/client";
import { useCreateProject, useProjects } from "../api/hooks";
import type { Project } from "../api/types";
import { useOrgCtx } from "../components/OrgLayout";
import { Avatar, Badge, Button, EmptyState, ErrorState, Field, Input, Modal, PageHeader, Pagination, Select, Skeleton, Textarea } from "../components/ui";
import { PROJECT_STATUSES, projectStatusMeta } from "../lib/constants";
import { useDebounced } from "../lib/hooks";
import { toast } from "../store/ui";

function ProjectCard({ p, orgId }: { p: Project; orgId: string }) {
  const { nameOf } = useOrgCtx();
  const pct = p.task_count ? Math.round((p.done_count / p.task_count) * 100) : 0;
  return (
    <Link to={`/o/${orgId}/projects/${p.id}`} className="group flex flex-col rounded-xl bg-white p-5 shadow-sm ring-1 ring-slate-200 transition hover:ring-indigo-300">
      <div className="flex items-start justify-between gap-3">
        <h3 className="font-semibold text-slate-900 group-hover:text-indigo-700">{p.name}</h3>
        <Badge className={projectStatusMeta(p.status).badge}>{projectStatusMeta(p.status).label}</Badge>
      </div>
      <p className="mt-1.5 line-clamp-2 min-h-10 text-sm text-slate-500">{p.description || "No description."}</p>
      <div className="mt-4">
        <div className="mb-1 flex justify-between text-xs text-slate-500"><span>{p.done_count} of {p.task_count} tasks done</span><span>{pct}%</span></div>
        <div className="h-1.5 overflow-hidden rounded-full bg-slate-100" role="img" aria-label={`${pct}% complete`}>
          <div className="h-full rounded-full bg-emerald-500" style={{ width: `${pct}%` }} />
        </div>
      </div>
      <div className="mt-4 flex -space-x-2">
        {p.member_ids.slice(0, 5).map((id) => <Avatar key={id} name={nameOf(id)} />)}
        {p.member_ids.length > 5 && <span className="ml-3 self-center pl-2 text-xs text-slate-500">+{p.member_ids.length - 5}</span>}
      </div>
    </Link>
  );
}

function NewProjectModal({ orgId, onClose }: { orgId: string; onClose: () => void }) {
  const { members, userId } = useOrgCtx();
  const create = useCreateProject(orgId);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [picked, setPicked] = useState<string[]>([]);
  const errs = create.error instanceof ApiError ? (create.error.details ?? {}) : {};

  function submit(e: FormEvent) {
    e.preventDefault();
    create.mutate({ name, description, member_ids: picked }, { onSuccess: () => { toast.success("Project created"); onClose(); } });
  }
  return (
    <Modal title="New project" onClose={onClose}>
      <form onSubmit={submit} className="space-y-4">
        <Field label="Name" error={errs.name}>{(id) => <Input id={id} value={name} onChange={(e) => setName(e.target.value)} required autoFocus maxLength={100} />}</Field>
        <Field label="Description" error={errs.description}>{(id) => <Textarea id={id} rows={3} value={description} onChange={(e) => setDescription(e.target.value)} maxLength={2000} />}</Field>
        <fieldset>
          <legend className="mb-1 text-sm font-medium text-slate-700">Team <span className="font-normal text-slate-500">(you're added automatically)</span></legend>
          <div className="max-h-44 space-y-1 overflow-y-auto rounded-lg p-1 ring-1 ring-slate-200">
            {members.filter((m) => m.user_id !== userId).map((m) => (
              <label key={m.user_id} className="flex cursor-pointer items-center gap-3 rounded-md px-2 py-1.5 text-sm hover:bg-slate-50">
                <input type="checkbox" className="size-4 rounded border-slate-300 text-indigo-600" checked={picked.includes(m.user_id)} onChange={(e) => setPicked((p) => (e.target.checked ? [...p, m.user_id] : p.filter((x) => x !== m.user_id)))} />
                <Avatar name={m.name} /> {m.name}
              </label>
            ))}
            {members.length <= 1 && <p className="px-2 py-1.5 text-sm text-slate-500">You're the only member. Add people from the Members page first.</p>}
          </div>
          {errs.member_ids && <p role="alert" className="mt-1 text-xs text-rose-600">{errs.member_ids}</p>}
        </fieldset>
        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="secondary" onClick={onClose}>Cancel</Button>
          <Button type="submit" loading={create.isPending}>Create project</Button>
        </div>
      </form>
    </Modal>
  );
}

export default function Projects() {
  const { org, isAdmin } = useOrgCtx();
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [sort, setSort] = useState("-updated_at");
  const [page, setPage] = useState(1);
  const [creating, setCreating] = useState(false);
  const q = useDebounced(search.trim());
  const projects = useProjects(org.id, { q, status, sort, page, page_size: 12 });

  const filtered = !!(q || status);
  return (
    <>
      <PageHeader title="Projects" subtitle="Everything your organization is working on." actions={isAdmin && <Button onClick={() => setCreating(true)}>New project</Button>} />
      <div className="mb-5 flex flex-wrap gap-3">
        <div className="min-w-48 flex-1">
          <label htmlFor="psearch" className="sr-only">Search projects</label>
          <Input id="psearch" type="search" placeholder="Search projects…" value={search} onChange={(e) => { setSearch(e.target.value); setPage(1); }} />
        </div>
        <label className="sr-only" htmlFor="pstatus">Status</label>
        <Select id="pstatus" className="w-44" value={status} onChange={(e) => { setStatus(e.target.value); setPage(1); }}>
          <option value="">All (excluding archived)</option>
          {PROJECT_STATUSES.map((s) => <option key={s.value} value={s.value}>{s.label}</option>)}
        </Select>
        <label className="sr-only" htmlFor="psort">Sort</label>
        <Select id="psort" className="w-44" value={sort} onChange={(e) => { setSort(e.target.value); setPage(1); }}>
          <option value="-updated_at">Recently updated</option>
          <option value="-created_at">Newest</option>
          <option value="name">Name (A–Z)</option>
        </Select>
      </div>

      {projects.isPending && <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">{[0, 1, 2].map((i) => <Skeleton key={i} className="h-48" />)}</div>}
      {projects.isError && <ErrorState error={projects.error} onRetry={() => projects.refetch()} />}
      {projects.data && projects.data.items.length === 0 && (
        <EmptyState
          title={filtered ? "No projects match your filters" : "No projects yet"}
          body={filtered ? "Try a different search or status." : isAdmin ? "Create your first project to start planning work." : "An owner or admin needs to create the first project."}
          action={!filtered && isAdmin ? <Button onClick={() => setCreating(true)}>New project</Button> : undefined}
        />
      )}
      {projects.data && projects.data.items.length > 0 && (
        <>
          <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">{projects.data.items.map((p) => <ProjectCard key={p.id} p={p} orgId={org.id} />)}</div>
          <Pagination meta={projects.data.meta} onPage={setPage} />
        </>
      )}
      {creating && <NewProjectModal orgId={org.id} onClose={() => setCreating(false)} />}
    </>
  );
}
