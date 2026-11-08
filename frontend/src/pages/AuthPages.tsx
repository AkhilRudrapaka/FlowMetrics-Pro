import { useState, type FormEvent, type ReactNode } from "react";
import { Link, Navigate, useLocation, useNavigate } from "react-router";
import { ApiError, errorMessage, login, register } from "../api/client";
import { Brand } from "../components/OrgLayout";
import { Button, Field, Input } from "../components/ui";
import { useAuth } from "../store/auth";

function Shell({ title, subtitle, children, footer }: { title: string; subtitle: string; children: ReactNode; footer: ReactNode }) {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center px-4 py-12">
      <div className="mb-8"><Brand /></div>
      <div className="w-full max-w-sm rounded-2xl bg-white p-7 shadow-sm ring-1 ring-slate-200">
        <h1 className="text-xl font-semibold tracking-tight text-slate-900">{title}</h1>
        <p className="mt-1 text-sm text-slate-500">{subtitle}</p>
        <div className="mt-6">{children}</div>
      </div>
      <p className="mt-6 text-sm text-slate-500">{footer}</p>
    </div>
  );
}

function useAuthForm(action: (f: FormData) => Promise<void>) {
  const navigate = useNavigate();
  const from = (useLocation().state as { from?: string } | null)?.from ?? "/";
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const [fields, setFields] = useState<Record<string, string>>({});

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setError(undefined);
    setFields({});
    try {
      await action(new FormData(e.currentTarget));
      navigate(from, { replace: true });
    } catch (err) {
      if (err instanceof ApiError && err.details) setFields(err.details);
      else setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }
  return { submit, busy, error, fields };
}

const str = (f: FormData, k: string) => String(f.get(k) ?? "");

export function Login() {
  const status = useAuth((s) => s.status);
  const { submit, busy, error } = useAuthForm((f) => login(str(f, "email"), str(f, "password")));
  const from = (useLocation().state as { from?: string } | null)?.from ?? "/";
  if (status === "authed") return <Navigate to={from} replace />;
  return (
    <Shell title="Welcome back" subtitle="Sign in to your workspace." footer={<>New here? <Link to="/register" className="font-medium text-indigo-600 hover:text-indigo-500">Create an account</Link></>}>
      <form onSubmit={submit} className="space-y-4" noValidate>
        <Field label="Email">{(id) => <Input id={id} name="email" type="email" autoComplete="email" required autoFocus />}</Field>
        <Field label="Password">{(id) => <Input id={id} name="password" type="password" autoComplete="current-password" required />}</Field>
        {error && <p role="alert" className="rounded-lg bg-rose-50 px-3 py-2 text-sm text-rose-700">{error}</p>}
        <Button type="submit" loading={busy} className="w-full">Sign in</Button>
      </form>
      {import.meta.env.VITE_DEMO_HINT === "true" && (
        <div className="mt-6 rounded-lg bg-slate-50 p-3 text-xs text-slate-600 ring-1 ring-slate-200">
          <p className="font-semibold text-slate-700">Demo data is loaded</p>
          <p className="mt-1"><code>alice@acme.test</code> (owner), <code>bob@acme.test</code> (admin), <code>carol@acme.test</code> (member), <code>dave@globex.test</code> (other tenant)</p>
          <p className="mt-1">Password: <code>Password123!</code></p>
        </div>
      )}
    </Shell>
  );
}

export function Register() {
  const status = useAuth((s) => s.status);
  const { submit, busy, error, fields } = useAuthForm((f) => register(str(f, "name"), str(f, "email"), str(f, "password")));
  if (status === "authed") return <Navigate to="/" replace />;
  return (
    <Shell title="Create your account" subtitle="Start organizing work in minutes." footer={<>Already registered? <Link to="/login" className="font-medium text-indigo-600 hover:text-indigo-500">Sign in</Link></>}>
      <form onSubmit={submit} className="space-y-4" noValidate>
        <Field label="Full name" error={fields.name}>{(id) => <Input id={id} name="name" autoComplete="name" required autoFocus />}</Field>
        <Field label="Email" error={fields.email}>{(id) => <Input id={id} name="email" type="email" autoComplete="email" required />}</Field>
        <Field label="Password" hint="At least 8 characters." error={fields.password}>{(id) => <Input id={id} name="password" type="password" autoComplete="new-password" required />}</Field>
        {error && <p role="alert" className="rounded-lg bg-rose-50 px-3 py-2 text-sm text-rose-700">{error}</p>}
        <Button type="submit" loading={busy} className="w-full">Create account</Button>
      </form>
    </Shell>
  );
}
