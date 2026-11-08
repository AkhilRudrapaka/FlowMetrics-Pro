import { useState } from "react";
import { Link, useSearchParams } from "react-router";
import { useProjects, useTasks } from "../api/hooks";
import { useOrgCtx } from "../components/OrgLayout";
import TaskModal from "../components/TaskModal";
import { Avatar, Badge, Button, EmptyState, ErrorState, Input, PageHeader, Pagination, Select, Skeleton, cx } from "../components/ui";
import { PRIORITIES, STATUSES, priorityMeta } from "../lib/constants";
import { formatDate, isOverdue, timeAgo } from "../lib/format";
import { useDebounced } from "../lib/hooks";

export default function Tasks() {
  const { org, nameOf, members } = useOrgCtx();
  const [sp, setSp] = useSearchParams();
  const openTask = sp.get("task");
  const [f, setF] = useState({ q: "", status: "", priority: "", assignee: "", project_id: "", sort: "-updated_at", overdue: false });
  const [page, setPage] = useState(1);
  const set = (patch: Partial<typeof f>) => { setF((p) => ({ ...p, ...patch })); setPage(1); };
  const q = useDebounced(f.q.trim());

  const tasks = useTasks(org.id, { ...f, q, page, page_size: 15 });
  const projects = useProjects(org.id, { page_size: 100, sort: "name" });
  const filtered = !!(q || f.status || f.priority || f.assignee || f.project_id || f.overdue);

  return (
    <>
      <PageHeader title="Tasks" subtitle="Search and filter across every active project." />
      <div className="mb-5 grid grid-cols-2 gap-3 lg:grid-cols-4 xl:grid-cols-7">
        <div className="col-span-2"><label htmlFor="q" className="sr-only">Search</label><Input id="q" type="search" placeholder="Search title or description…" value={f.q} onChange={(e) => set({ q: e.target.value })} /></div>
        <div><label htmlFor="fs" className="sr-only">Status</label><Select id="fs" value={f.status} onChange={(e) => set({ status: e.target.value })}><option value="">Any status</option>{STATUSES.map((s) => <option key={s.value} value={s.value}>{s.label}</option>)}</Select></div>
        <div><label htmlFor="fp" className="sr-only">Priority</label><Select id="fp" value={f.priority} onChange={(e) => set({ priority: e.target.value })}><option value="">Any priority</option>{PRIORITIES.map((p) => <option key={p.value} value={p.value}>{p.label}</option>)}</Select></div>
        <div><label htmlFor="fa" className="sr-only">Assignee</label>
          <Select id="fa" value={f.assignee} onChange={(e) => set({ assignee: e.target.value })}>
            <option value="">Anyone</option><option value="me">Assigned to me</option><option value="unassigned">Unassigned</option>
            {members.map((m) => <option key={m.user_id} value={m.user_id}>{m.name}</option>)}
          </Select>
        </div>
        <div><label htmlFor="fj" className="sr-only">Project</label><Select id="fj" value={f.project_id} onChange={(e) => set({ project_id: e.target.value })}><option value="">All projects</option>{projects.data?.items.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}</Select></div>
        <div><label htmlFor="fo" className="sr-only">Sort</label><Select id="fo" value={f.sort} onChange={(e) => set({ sort: e.target.value })}><option value="-updated_at">Recently updated</option><option value="-created_at">Newest</option><option value="due_date">Due soonest</option><option value="title">Title (A–Z)</option></Select></div>
      </div>
      <div className="mb-4 flex items-center gap-4">
        <label className="flex cursor-pointer items-center gap-2 text-sm text-slate-700"><input type="checkbox" className="size-4 rounded border-slate-300 text-indigo-600" checked={f.overdue} onChange={(e) => set({ overdue: e.target.checked })} /> Overdue only</label>
        {filtered && <Button variant="ghost" size="sm" onClick={() => { setF({ q: "", status: "", priority: "", assignee: "", project_id: "", sort: "-updated_at", overdue: false }); setPage(1); }}>Clear filters</Button>}
      </div>

      {tasks.isPending && <div className="space-y-2">{[0, 1, 2, 3, 4].map((i) => <Skeleton key={i} className="h-14" />)}</div>}
      {tasks.isError && <ErrorState error={tasks.error} onRetry={() => tasks.refetch()} />}
      {tasks.data && tasks.data.items.length === 0 && (
        <EmptyState title={filtered ? "No tasks match your filters" : "No tasks yet"} body={filtered ? "Try removing a filter or searching for something else." : "Open a project and add the first task to your board."}
          action={!filtered ? <Link to={`/o/${org.id}/projects`}><Button>Go to projects</Button></Link> : undefined} />
      )}
      {tasks.data && tasks.data.items.length > 0 && (
        <>
          <div className={cx("relative overflow-x-auto rounded-xl bg-white shadow-sm ring-1 ring-slate-200 transition-opacity", tasks.isPlaceholderData && "opacity-60")}>
            <table className="min-w-full divide-y divide-slate-200 text-left text-sm">
              <thead className="bg-slate-50 text-xs uppercase tracking-wide text-slate-500">
                <tr><th scope="col" className="px-4 py-3 font-medium">Task</th><th scope="col" className="px-4 py-3 font-medium">Status</th><th scope="col" className="px-4 py-3 font-medium">Priority</th><th scope="col" className="px-4 py-3 font-medium">Assignee</th><th scope="col" className="px-4 py-3 font-medium">Due</th><th scope="col" className="px-4 py-3 font-medium">Updated</th></tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {tasks.data.items.map((t) => {
                  const st = STATUSES.find((s) => s.value === t.status)!;
                  const overdue = isOverdue(t.due_date, t.status);
                  return (
                    <tr key={t.id} className="hover:bg-slate-50">
                      <td className="max-w-xs px-4 py-3">
                        <button onClick={() => setSp((p) => { p.set("task", t.id); return p; })} className="block max-w-full truncate text-left font-medium text-slate-900 hover:text-indigo-700">{t.title}</button>
                        <p className="truncate text-xs text-slate-500">{t.project_name}</p>
                      </td>
                      <td className="whitespace-nowrap px-4 py-3"><span className="inline-flex items-center gap-2"><span className={cx("size-2 rounded-full", st.dot)} aria-hidden="true" />{st.label}</span></td>
                      <td className="px-4 py-3"><Badge className={priorityMeta(t.priority).badge}>{priorityMeta(t.priority).label}</Badge></td>
                      <td className="whitespace-nowrap px-4 py-3">{t.assignee_id ? <span className="inline-flex items-center gap-2"><Avatar name={nameOf(t.assignee_id)} />{nameOf(t.assignee_id)}</span> : <span className="text-slate-400">Unassigned</span>}</td>
                      <td className={cx("whitespace-nowrap px-4 py-3", overdue ? "font-semibold text-rose-600" : "text-slate-600")}>{t.due_date ? <>{overdue && <span className="sr-only">Overdue: </span>}{formatDate(t.due_date)}</> : "—"}</td>
                      <td className="whitespace-nowrap px-4 py-3 text-slate-500">{timeAgo(t.updated_at)}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
          <Pagination meta={tasks.data.meta} onPage={setPage} />
        </>
      )}
      {openTask && <TaskModal taskId={openTask} onClose={() => setSp((p) => { p.delete("task"); return p; }, { replace: true })} />}
    </>
  );
}
