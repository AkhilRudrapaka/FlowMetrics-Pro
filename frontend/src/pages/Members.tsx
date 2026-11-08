import { useState, type FormEvent } from "react";
import { useNavigate } from "react-router";
import { ApiError } from "../api/client";
import { useAddMember, useRemoveMember, useUpdateMemberRole } from "../api/hooks";
import type { Member, Role } from "../api/types";
import { canManage, useOrgCtx } from "../components/OrgLayout";
import { Avatar, Badge, Button, Card, ConfirmDialog, Field, Input, PageHeader, Select } from "../components/ui";
import { ROLE_BADGE } from "../lib/constants";
import { formatDate } from "../lib/format";
import { toast } from "../store/ui";

export default function Members() {
  const { org, role, isAdmin, members, userId } = useOrgCtx();
  const navigate = useNavigate();
  const add = useAddMember(org.id);
  const setRole = useUpdateMemberRole(org.id);
  const remove = useRemoveMember(org.id);
  const [email, setEmail] = useState("");
  const [newRole, setNewRole] = useState<Role>("member");
  const [target, setTarget] = useState<Member | null>(null);
  const errs = add.error instanceof ApiError ? (add.error.details ?? {}) : {};

  function submit(e: FormEvent) {
    e.preventDefault();
    add.mutate({ email, role: newRole }, { onSuccess: () => { toast.success("Member added"); setEmail(""); } });
  }

  function confirmRemove() {
    if (!target) return;
    const leaving = target.user_id === userId;
    remove.mutate(target.user_id, {
      onSuccess: () => {
        setTarget(null);
        if (leaving) { toast.success("You left the organization"); navigate("/orgs"); } else toast.success("Member removed");
      },
    });
  }

  return (
    <>
      <PageHeader title="Members" subtitle={`${members.length} ${members.length === 1 ? "person" : "people"} in ${org.name}`} />

      {isAdmin && (
        <Card className="mb-6">
          <h2 className="text-base font-semibold">Add a member</h2>
          <p className="mt-0.5 text-sm text-slate-500">The person must already have a FlowMetrics Pro account. Use the email they registered with.</p>
          <form onSubmit={submit} className="mt-4 grid gap-3 sm:grid-cols-[1fr_10rem_auto] sm:items-end">
            <Field label="Email" error={errs.email}>{(id) => <Input id={id} type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="teammate@company.com" required />}</Field>
            <Field label="Role" error={errs.role}>
              {(id) => (
                <Select id={id} value={newRole} onChange={(e) => setNewRole(e.target.value as Role)}>
                  <option value="member">Member</option>
                  {role === "owner" && <option value="admin">Admin</option>}
                </Select>
              )}
            </Field>
            <Button type="submit" loading={add.isPending}>Add member</Button>
          </form>
        </Card>
      )}

      <div className="relative overflow-x-auto rounded-xl bg-white shadow-sm ring-1 ring-slate-200">
        <table className="min-w-full divide-y divide-slate-200 text-left text-sm">
          <thead className="bg-slate-50 text-xs uppercase tracking-wide text-slate-500">
            <tr><th scope="col" className="px-4 py-3 font-medium">Member</th><th scope="col" className="px-4 py-3 font-medium">Role</th><th scope="col" className="px-4 py-3 font-medium">Joined</th><th scope="col" className="px-4 py-3"><span className="sr-only">Actions</span></th></tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {members.map((m) => {
              const isSelf = m.user_id === userId;
              const canChangeRole = role === "owner" && m.role !== "owner";
              const canRemove = isSelf ? m.role !== "owner" : isAdmin && canManage(role, m.role);
              return (
                <tr key={m.user_id}>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-3">
                      <Avatar name={m.name} className="size-9" />
                      <div className="min-w-0"><p className="truncate font-medium text-slate-900">{m.name}{isSelf && <span className="ml-2 text-xs font-normal text-slate-400">(you)</span>}</p><p className="truncate text-xs text-slate-500">{m.email}</p></div>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    {canChangeRole ? (
                      <>
                        <label htmlFor={`role-${m.user_id}`} className="sr-only">Role for {m.name}</label>
                        <Select id={`role-${m.user_id}`} className="w-32 py-1" value={m.role} disabled={setRole.isPending} onChange={(e) => setRole.mutate({ userId: m.user_id, role: e.target.value as Role }, { onSuccess: () => toast.success("Role updated") })}>
                          <option value="admin">Admin</option><option value="member">Member</option>
                        </Select>
                      </>
                    ) : <Badge className={ROLE_BADGE[m.role]}>{m.role}</Badge>}
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 text-slate-500">{formatDate(m.joined_at)}</td>
                  <td className="px-4 py-3 text-right">{canRemove && <Button variant="ghost" size="sm" onClick={() => setTarget(m)}>{isSelf ? "Leave" : "Remove"}</Button>}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
      <p className="mt-3 text-xs text-slate-500">Owners can promote members to admin and manage everyone. Admins can add and remove regular members. Members can view everything and work in the projects they belong to.</p>

      {target && (
        <ConfirmDialog
          title={target.user_id === userId ? "Leave this organization?" : `Remove ${target.name}?`}
          confirmLabel={target.user_id === userId ? "Leave" : "Remove"}
          loading={remove.isPending}
          onClose={() => setTarget(null)}
          onConfirm={confirmRemove}
          body={target.user_id === userId ? "You will lose access to all of its projects and tasks." : "They will lose access immediately, be removed from every project, and their tasks will become unassigned."}
        />
      )}
    </>
  );
}
