import { Link } from "react-router";
import { useDashboard } from "../api/hooks";
import { useOrgCtx } from "../components/OrgLayout";
import { Avatar, Card, EmptyState, ErrorState, PageHeader, Skeleton, cx } from "../components/ui";
import { PRIORITIES, STATUSES } from "../lib/constants";
import { describeActivity, timeAgo } from "../lib/format";
import { useAuth } from "../store/auth";

function Stat({ label, value, hint, tone = "text-slate-900", to }: { label: string; value: number; hint?: string; tone?: string; to?: string }) {
  const body = (
    <Card className={cx("h-full", to && "transition hover:ring-indigo-300")}>
      <p className="text-sm font-medium text-slate-500">{label}</p>
      <p className={cx("mt-2 text-3xl font-semibold tabular-nums", tone)}>{value}</p>
      {hint && <p className="mt-1 text-xs text-slate-500">{hint}</p>}
    </Card>
  );
  return to ? <Link to={to}>{body}</Link> : body;
}

export default function Dashboard() {
  const { org } = useOrgCtx();
  const user = useAuth((s) => s.user)!;
  const q = useDashboard(org.id);
  const d = q.data;
  const max = d ? Math.max(1, ...Object.values(d.tasks_by_status)) : 1;

  return (
    <>
      <PageHeader title="Dashboard" subtitle={`Welcome back, ${user.name.split(" ")[0]}. Here's where ${org.name} stands.`} />
      {q.isError && <ErrorState error={q.error} onRetry={() => q.refetch()} />}
      {q.isPending && (
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-5">{[0, 1, 2, 3, 4].map((i) => <Skeleton key={i} className="h-28" />)}</div>
      )}
      {d && (
        <div className="space-y-6">
          <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-5">
            <Stat label="Projects" value={d.total_projects} hint={`${d.active_projects} active`} to={`/o/${org.id}/projects`} />
            <Stat label="Active tasks" value={d.active_tasks} hint="To do, in progress, review, blocked" to={`/o/${org.id}/tasks`} />
            <Stat label="Completed" value={d.completed_tasks} tone="text-emerald-600" hint={`of ${d.total_tasks} total tasks`} />
            <Stat label="Overdue" value={d.overdue_tasks} tone={d.overdue_tasks > 0 ? "text-rose-600" : "text-slate-900"} hint={d.overdue_tasks > 0 ? "Needs attention" : "Nothing is late"} />
            <Stat label="Assigned to me" value={d.my_open_tasks} tone="text-indigo-600" hint="Open tasks" />
          </div>

          <div className="grid gap-6 lg:grid-cols-5">
            <Card className="lg:col-span-2">
              <h2 className="text-base font-semibold">Tasks by status</h2>
              {d.total_tasks === 0 ? (
                <p className="mt-4 text-sm text-slate-500">No tasks yet.</p>
              ) : (
                <ul className="mt-4 space-y-3">
                  {STATUSES.map((s) => {
                    const n = d.tasks_by_status[s.value] ?? 0;
                    return (
                      <li key={s.value}>
                        <div className="mb-1 flex justify-between text-sm"><span className="text-slate-700">{s.label}</span><span className="tabular-nums text-slate-500">{n}</span></div>
                        <div className="h-2 overflow-hidden rounded-full bg-slate-100" role="img" aria-label={`${s.label}: ${n}`}>
                          <div className={cx("h-full rounded-full", s.dot)} style={{ width: `${(n / max) * 100}%` }} />
                        </div>
                      </li>
                    );
                  })}
                </ul>
              )}
              <h3 className="mt-6 text-sm font-semibold text-slate-700">By priority</h3>
              <div className="mt-2 flex flex-wrap gap-2">
                {PRIORITIES.map((p) => (
                  <span key={p.value} className={cx("rounded-full px-2.5 py-1 text-xs font-medium ring-1 ring-inset", p.badge)}>{p.label}: {d.tasks_by_priority[p.value] ?? 0}</span>
                ))}
              </div>
            </Card>

            <Card className="lg:col-span-3">
              <h2 className="text-base font-semibold">Recent activity</h2>
              {d.recent_activity.length === 0 ? (
                <div className="mt-4"><EmptyState title="No activity yet" body="Create a project and add some tasks. Everything that happens shows up here." /></div>
              ) : (
                <ul className="mt-4 divide-y divide-slate-100">
                  {d.recent_activity.map((a) => (
                    <li key={a.id} className="flex items-start gap-3 py-3">
                      <Avatar name={a.actor_name} />
                      <div className="min-w-0 flex-1 text-sm">
                        <p className="text-slate-700"><span className="font-medium text-slate-900">{a.actor_name}</span> {describeActivity(a)}</p>
                        <p className="mt-0.5 text-xs text-slate-400">{timeAgo(a.created_at)}</p>
                      </div>
                    </li>
                  ))}
                </ul>
              )}
            </Card>
          </div>
        </div>
      )}
    </>
  );
}
