import { createContext, useContext, useEffect, useMemo, useState } from "react";
import { Link, NavLink, Outlet, useNavigate, useParams } from "react-router";
import { logout } from "../api/client";
import { useMembers, useOrg, useOrgs } from "../api/hooks";
import type { Member, Org, Project, Role } from "../api/types";
import { ROLE_BADGE } from "../lib/constants";
import { useAuth } from "../store/auth";
import { useUi } from "../store/ui";
import { Avatar, Badge, Button, ErrorState, PageSpinner, Select, cx } from "./ui";

interface OrgContext {
  org: Org;
  role: Role;
  isAdmin: boolean; // owner or admin
  members: Member[];
  userId: string;
  /** Display name for a user id; removed members are shown as "Former member". */
  nameOf: (id: string | null | undefined) => string;
  /** UI mirror of the server's rule (the server remains the authority): can I write tasks in this project? */
  canWrite: (p: Pick<Project, "member_ids" | "status">) => boolean;
}

const Ctx = createContext<OrgContext | null>(null);

export function useOrgCtx() {
  const v = useContext(Ctx);
  if (!v) throw new Error("useOrgCtx outside OrgLayout");
  return v;
}

/** Mirrors permissions.CanManage on the server. */
export const canManage = (actor: Role, target: Role) => (actor === "owner" ? target !== "owner" : actor === "admin" && target === "member");

const links = [
  { to: "", label: "Dashboard", end: true, d: "M3 12l9-9 9 9M5 10v10h5v-6h4v6h5V10" },
  { to: "projects", label: "Projects", d: "M3 7a2 2 0 012-2h4l2 2h8a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2V7z" },
  { to: "tasks", label: "Tasks", d: "M9 11l3 3L22 4M21 12v7a2 2 0 01-2 2H5a2 2 0 01-2-2V5a2 2 0 012-2h11" },
  { to: "members", label: "Members", d: "M17 21v-2a4 4 0 00-4-4H5a4 4 0 00-4 4v2M9 11a4 4 0 100-8 4 4 0 000 8zM23 21v-2a4 4 0 00-3-3.87M16 3.13a4 4 0 010 7.75" },
  { to: "settings", label: "Settings", d: "M12 15a3 3 0 100-6 3 3 0 000 6zM19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 11-2.83 2.83l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 11-4 0v-.09a1.65 1.65 0 00-1.08-1.51 1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 11-2.83-2.83l.06-.06a1.65 1.65 0 00.33-1.82 1.65 1.65 0 00-1.51-1H3a2 2 0 110-4h.09a1.65 1.65 0 001.51-1.08 1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 112.83-2.83l.06.06a1.65 1.65 0 001.82.33h0a1.65 1.65 0 001-1.51V3a2 2 0 114 0v.09a1.65 1.65 0 001 1.51h0a1.65 1.65 0 001.82-.33l.06-.06a2 2 0 112.83 2.83l-.06.06a1.65 1.65 0 00-.33 1.82v0a1.65 1.65 0 001.51 1H21a2 2 0 110 4h-.09a1.65 1.65 0 00-1.51 1z" },
];

export default function OrgLayout() {
  const { orgId = "" } = useParams();
  const navigate = useNavigate();
  const user = useAuth((s) => s.user)!;
  const setLastOrg = useUi((s) => s.setLastOrg);
  const orgQ = useOrg(orgId);
  const membersQ = useMembers(orgId);
  const orgsQ = useOrgs();
  const [open, setOpen] = useState(false);

  useEffect(() => {
    if (orgQ.data) setLastOrg(orgId);
    if (orgQ.isError) setLastOrg(null);
  }, [orgQ.data, orgQ.isError, orgId, setLastOrg]);

  const ctx = useMemo<OrgContext | null>(() => {
    if (!orgQ.data || !membersQ.data) return null;
    const byId = new Map(membersQ.data.map((m) => [m.user_id, m.name]));
    const role = orgQ.data.role;
    return {
      org: orgQ.data,
      role,
      isAdmin: role === "owner" || role === "admin",
      members: membersQ.data,
      userId: user.id,
      nameOf: (id) => (id ? (byId.get(id) ?? "Former member") : "Unassigned"),
      canWrite: (p) => p.status !== "archived" && (role !== "member" || p.member_ids.includes(user.id)),
    };
  }, [orgQ.data, membersQ.data, user.id]);

  if (orgQ.isError) {
    return (
      <div className="mx-auto max-w-lg px-4 py-20">
        <ErrorState error={orgQ.error} onRetry={() => orgQ.refetch()} />
        <p className="mt-4 text-center"><Link to="/orgs" className="text-sm font-medium text-indigo-600 hover:text-indigo-500">Choose another organization</Link></p>
      </div>
    );
  }
  if (!ctx) return <PageSpinner label="Loading workspace…" />;

  const nav = (
    <nav aria-label="Main" className="flex flex-col gap-1">
      {links.map((l) => (
        <NavLink
          key={l.to}
          to={`/o/${orgId}/${l.to}`}
          end={l.end}
          onClick={() => setOpen(false)}
          className={({ isActive }) => cx("flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium", isActive ? "bg-indigo-50 text-indigo-700" : "text-slate-600 hover:bg-slate-100 hover:text-slate-900")}
        >
          <svg className="size-5 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d={l.d} /></svg>
          {l.label}
        </NavLink>
      ))}
    </nav>
  );

  return (
    <Ctx.Provider value={ctx}>
      <div className="min-h-screen lg:flex">
        {/* mobile top bar */}
        <header className="sticky top-0 z-30 flex items-center justify-between border-b border-slate-200 bg-white px-4 py-3 lg:hidden">
          <Brand />
          <button onClick={() => setOpen((o) => !o)} aria-expanded={open} aria-label="Toggle navigation" className="rounded-md p-2 text-slate-600 hover:bg-slate-100">
            <svg className="size-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="1.8" aria-hidden="true"><path strokeLinecap="round" d="M4 6h16M4 12h16M4 18h16" /></svg>
          </button>
        </header>

        <aside className={cx("fixed inset-y-0 left-0 z-20 w-64 transform flex-col gap-6 border-r border-slate-200 bg-white p-4 pt-20 transition-transform lg:static lg:flex lg:translate-x-0 lg:pt-4", open ? "flex translate-x-0" : "hidden -translate-x-full lg:flex")}>
          <div className="hidden lg:block"><Brand /></div>
          <div>
            <label htmlFor="org-switch" className="mb-1 block text-xs font-medium uppercase tracking-wide text-slate-500">Organization</label>
            <Select id="org-switch" value={orgId} onChange={(e) => (e.target.value === "__all" ? navigate("/orgs") : navigate(`/o/${e.target.value}`))}>
              {orgsQ.data?.map((o) => <option key={o.id} value={o.id}>{o.name}</option>)}
              {!orgsQ.data?.some((o) => o.id === orgId) && <option value={orgId}>{ctx.org.name}</option>}
              <option value="__all">Manage organizations…</option>
            </Select>
            <p className="mt-2 flex items-center gap-2 text-xs text-slate-500">Your role <Badge className={ROLE_BADGE[ctx.role]}>{ctx.role}</Badge></p>
          </div>
          {nav}
          <div className="mt-auto flex items-center gap-3 border-t border-slate-200 pt-4">
            <Avatar name={user.name} className="size-9 text-xs" />
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium text-slate-900">{user.name}</p>
              <p className="truncate text-xs text-slate-500">{user.email}</p>
            </div>
            <Button variant="ghost" size="sm" onClick={() => logout().then(() => navigate("/login"))}>Sign out</Button>
          </div>
        </aside>

        <main className="min-w-0 flex-1 px-4 py-6 sm:px-6 lg:px-8">
          <div className="mx-auto max-w-7xl"><Outlet /></div>
        </main>
      </div>
    </Ctx.Provider>
  );
}

export function Brand() {
  return (
    <Link to="/" className="flex items-center gap-2 text-base font-bold tracking-tight text-slate-900">
      <span className="flex size-8 items-center justify-center rounded-lg bg-indigo-600 text-white">
        <svg className="size-5" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M5 18V8h3v10zm5.5 0v-6h3v6zM16 18V5h3v13z" /></svg>
      </span>
      FlowMetrics <span className="font-medium text-indigo-600">Pro</span>
    </Link>
  );
}
