import { useState, type FormEvent } from "react";
import { Link, useNavigate } from "react-router";
import { ApiError, logout } from "../api/client";
import { useCreateOrg, useOrgs } from "../api/hooks";
import { Brand } from "../components/OrgLayout";
import { Badge, Button, Card, EmptyState, ErrorState, Field, Input, Skeleton } from "../components/ui";
import { ROLE_BADGE } from "../lib/constants";
import { formatDate } from "../lib/format";
import { useAuth } from "../store/auth";

export default function OrgPicker() {
  const navigate = useNavigate();
  const user = useAuth((s) => s.user)!;
  const orgs = useOrgs();
  const create = useCreateOrg();
  const [name, setName] = useState("");
  const nameError = create.error instanceof ApiError ? create.error.details?.name : undefined;

  function submit(e: FormEvent) {
    e.preventDefault();
    create.mutate(name, { onSuccess: (org) => navigate(`/o/${org.id}`) });
  }

  return (
    <div className="mx-auto max-w-3xl px-4 py-10">
      <header className="mb-8 flex items-center justify-between">
        <Brand />
        <div className="flex items-center gap-3 text-sm text-slate-600">
          <span className="hidden sm:inline">{user.email}</span>
          <Button variant="secondary" size="sm" onClick={() => logout().then(() => navigate("/login"))}>Sign out</Button>
        </div>
      </header>

      <h1 className="text-2xl font-semibold tracking-tight">Your organizations</h1>
      <p className="mt-1 text-sm text-slate-500">Each organization is a separate workspace. Its projects, tasks and members are invisible to everyone outside it.</p>

      <div className="mt-6 space-y-3">
        {orgs.isPending && [0, 1].map((i) => <Skeleton key={i} className="h-20" />)}
        {orgs.isError && <ErrorState error={orgs.error} onRetry={() => orgs.refetch()} />}
        {orgs.data?.length === 0 && <EmptyState title="You're not in any organization yet" body="Create your first one below, or ask an owner or admin to add you by email." />}
        {orgs.data?.map((o) => (
          <Link key={o.id} to={`/o/${o.id}`} className="flex items-center justify-between rounded-xl bg-white p-5 shadow-sm ring-1 ring-slate-200 transition hover:ring-indigo-300">
            <div>
              <p className="font-semibold text-slate-900">{o.name}</p>
              <p className="mt-0.5 text-xs text-slate-500">Created {formatDate(o.created_at)}</p>
            </div>
            <Badge className={ROLE_BADGE[o.role]}>{o.role}</Badge>
          </Link>
        ))}
      </div>

      <Card className="mt-8">
        <h2 className="text-base font-semibold">Create an organization</h2>
        <form onSubmit={submit} className="mt-4 flex flex-col gap-3 sm:flex-row sm:items-end">
          <div className="flex-1">
            <Field label="Name" error={nameError}>{(id) => <Input id={id} value={name} onChange={(e) => setName(e.target.value)} placeholder="Acme Corp" required />}</Field>
          </div>
          <Button type="submit" loading={create.isPending}>Create</Button>
        </form>
      </Card>
    </div>
  );
}
