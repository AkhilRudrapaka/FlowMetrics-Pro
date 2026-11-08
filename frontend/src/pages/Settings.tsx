import { useState, type FormEvent } from "react";
import { useNavigate } from "react-router";
import { ApiError } from "../api/client";
import { useDeleteOrg, useRenameOrg } from "../api/hooks";
import { useOrgCtx } from "../components/OrgLayout";
import { Button, Card, ConfirmDialog, Field, Input, PageHeader } from "../components/ui";
import { formatDate } from "../lib/format";
import { toast } from "../store/ui";

export default function Settings() {
  const { org, role, isAdmin } = useOrgCtx();
  const navigate = useNavigate();
  const rename = useRenameOrg(org.id);
  const del = useDeleteOrg(org.id);
  const [name, setName] = useState(org.name);
  const [confirming, setConfirming] = useState(false);
  const [typed, setTyped] = useState("");
  const nameError = rename.error instanceof ApiError ? rename.error.details?.name : undefined;

  function submit(e: FormEvent) {
    e.preventDefault();
    rename.mutate(name, { onSuccess: () => toast.success("Organization renamed") });
  }

  return (
    <>
      <PageHeader title="Settings" subtitle={`Created ${formatDate(org.created_at)} · you are ${role === "admin" ? "an admin" : `the ${role}`}`} />
      <div className="max-w-2xl space-y-6">
        <Card>
          <h2 className="text-base font-semibold">General</h2>
          <form onSubmit={submit} className="mt-4 flex flex-col gap-3 sm:flex-row sm:items-end">
            <div className="flex-1"><Field label="Organization name" error={nameError}>{(id) => <Input id={id} value={name} onChange={(e) => setName(e.target.value)} disabled={!isAdmin} maxLength={80} />}</Field></div>
            {isAdmin && <Button type="submit" loading={rename.isPending} disabled={name.trim() === org.name}>Save</Button>}
          </form>
          {!isAdmin && <p className="mt-2 text-xs text-slate-500">Only owners and admins can rename the organization.</p>}
        </Card>

        {role === "owner" && (
          <Card className="ring-rose-200">
            <h2 className="text-base font-semibold text-rose-700">Danger zone</h2>
            <p className="mt-1 text-sm text-slate-600">Deleting the organization permanently removes every project, task, comment and member association. This cannot be undone.</p>
            <Button variant="danger" className="mt-4" onClick={() => setConfirming(true)}>Delete organization</Button>
          </Card>
        )}
      </div>

      {confirming && (
        <ConfirmDialog
          title="Delete organization"
          confirmLabel="Delete forever"
          loading={del.isPending}
          onClose={() => { setConfirming(false); setTyped(""); }}
          onConfirm={() => typed === org.name && del.mutate(undefined, { onSuccess: () => { toast.success("Organization deleted"); navigate("/orgs"); } })}
          body={
            <div className="space-y-3">
              <p>Type <strong className="select-all text-slate-900">{org.name}</strong> to confirm.</p>
              <Input aria-label="Type the organization name to confirm" value={typed} onChange={(e) => setTyped(e.target.value)} autoFocus />
              {typed !== org.name && typed.length > 0 && <p className="text-xs text-slate-500">The name must match exactly.</p>}
            </div>
          }
        />
      )}
    </>
  );
}
