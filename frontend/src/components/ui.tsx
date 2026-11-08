import { useEffect, useId, useRef, type ButtonHTMLAttributes, type InputHTMLAttributes, type ReactNode, type SelectHTMLAttributes, type TextareaHTMLAttributes } from "react";
import { ApiError, errorMessage } from "../api/client";
import type { PageMeta } from "../api/types";
import { initials } from "../lib/format";
import { useToasts } from "../store/ui";

export const cx = (...c: (string | false | null | undefined)[]) => c.filter(Boolean).join(" ");

// ---------- buttons ----------
const variants = {
  primary: "bg-indigo-600 text-white hover:bg-indigo-700 focus-visible:outline-indigo-600 disabled:bg-indigo-300",
  secondary: "bg-white text-slate-700 ring-1 ring-inset ring-slate-300 hover:bg-slate-50 focus-visible:outline-indigo-600 disabled:text-slate-400",
  danger: "bg-rose-600 text-white hover:bg-rose-700 focus-visible:outline-rose-600 disabled:bg-rose-300",
  ghost: "text-slate-600 hover:bg-slate-100 focus-visible:outline-indigo-600 disabled:text-slate-300",
};

export function Button({
  variant = "primary", size = "md", loading, className, children, disabled, ...rest
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: keyof typeof variants; size?: "sm" | "md"; loading?: boolean }) {
  return (
    <button
      {...rest}
      disabled={disabled || loading}
      className={cx(
        "inline-flex items-center justify-center gap-2 rounded-lg font-medium transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 disabled:cursor-not-allowed",
        size === "sm" ? "px-2.5 py-1.5 text-xs" : "px-3.5 py-2 text-sm",
        variants[variant],
        className,
      )}
    >
      {loading && <Spinner className="size-4" />}
      {children}
    </button>
  );
}

// ---------- form controls ----------
const control =
  "block w-full rounded-lg border-0 bg-white px-3 py-2 text-sm text-slate-900 ring-1 ring-inset ring-slate-300 placeholder:text-slate-400 focus:ring-2 focus:ring-inset focus:ring-indigo-600 disabled:bg-slate-50 disabled:text-slate-500";

export function Field({ label, error, hint, children }: { label: string; error?: string; hint?: string; children: (id: string) => ReactNode }) {
  const id = useId();
  return (
    <div>
      <label htmlFor={id} className="mb-1 block text-sm font-medium text-slate-700">{label}</label>
      {children(id)}
      {hint && !error && <p className="mt-1 text-xs text-slate-500">{hint}</p>}
      {error && <p role="alert" className="mt-1 text-xs text-rose-600">{error}</p>}
    </div>
  );
}

export const Input = ({ className, ...p }: InputHTMLAttributes<HTMLInputElement>) => <input {...p} className={cx(control, className)} />;
export const Textarea = ({ className, ...p }: TextareaHTMLAttributes<HTMLTextAreaElement>) => <textarea {...p} className={cx(control, "resize-y", className)} />;
export const Select = ({ className, ...p }: SelectHTMLAttributes<HTMLSelectElement>) => <select {...p} className={cx(control, "pr-8", className)} />;

// ---------- small display pieces ----------
export const Badge = ({ className, children }: { className: string; children: ReactNode }) => (
  <span className={cx("inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ring-1 ring-inset", className)}>{children}</span>
);

export function Avatar({ name, className }: { name: string; className?: string }) {
  return (
    <span title={name} aria-label={name} className={cx("inline-flex size-7 shrink-0 items-center justify-center rounded-full bg-indigo-100 text-[11px] font-semibold text-indigo-700 ring-2 ring-white", className)}>
      {initials(name) || "?"}
    </span>
  );
}

export function Spinner({ className = "size-5" }: { className?: string }) {
  return (
    <svg className={cx("animate-spin", className)} viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <circle cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" className="opacity-25" />
      <path d="M4 12a8 8 0 018-8" stroke="currentColor" strokeWidth="4" strokeLinecap="round" className="opacity-75" />
    </svg>
  );
}

export const PageSpinner = ({ label = "Loading…" }: { label?: string }) => (
  <div role="status" className="flex items-center justify-center gap-3 py-20 text-slate-500">
    <Spinner className="size-6 text-indigo-600" /> <span className="text-sm">{label}</span>
  </div>
);

export const Skeleton = ({ className }: { className?: string }) => <div aria-hidden="true" className={cx("animate-pulse rounded-lg bg-slate-200/70", className)} />;

export function EmptyState({ title, body, action }: { title: string; body?: string; action?: ReactNode }) {
  return (
    <div className="rounded-xl border border-dashed border-slate-300 bg-white px-6 py-14 text-center">
      <p className="text-base font-semibold text-slate-900">{title}</p>
      {body && <p className="mx-auto mt-1 max-w-md text-sm text-slate-500">{body}</p>}
      {action && <div className="mt-5">{action}</div>}
    </div>
  );
}

export function ErrorState({ error, onRetry }: { error: unknown; onRetry?: () => void }) {
  const notFound = error instanceof ApiError && error.status === 404;
  return (
    <div role="alert" className="rounded-xl border border-rose-200 bg-rose-50 px-6 py-10 text-center">
      <p className="text-base font-semibold text-rose-800">{notFound ? "Not found" : "Something went wrong"}</p>
      <p className="mx-auto mt-1 max-w-md text-sm text-rose-700">{notFound ? "This item doesn't exist or you don't have access to it." : errorMessage(error)}</p>
      {onRetry && !notFound && <Button variant="secondary" className="mt-4" onClick={onRetry}>Try again</Button>}
    </div>
  );
}

export const Card = ({ className, children }: { className?: string; children: ReactNode }) => (
  <div className={cx("rounded-xl bg-white p-5 shadow-sm ring-1 ring-slate-200", className)}>{children}</div>
);

export function PageHeader({ title, subtitle, actions }: { title: string; subtitle?: ReactNode; actions?: ReactNode }) {
  return (
    <div className="mb-6 flex flex-wrap items-start justify-between gap-3">
      <div className="min-w-0">
        <h1 className="truncate text-2xl font-semibold tracking-tight text-slate-900">{title}</h1>
        {subtitle && <div className="mt-1 text-sm text-slate-500">{subtitle}</div>}
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
    </div>
  );
}

// ---------- modal ----------
export function Modal({ title, onClose, children, wide }: { title: string; onClose: () => void; children: ReactNode; wide?: boolean }) {
  const ref = useRef<HTMLDivElement>(null);
  const titleId = useId();
  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null;
    ref.current?.focus();
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    document.addEventListener("keydown", onKey);
    document.body.style.overflow = "hidden";
    return () => {
      document.removeEventListener("keydown", onKey);
      document.body.style.overflow = "";
      previous?.focus();
    };
  }, [onClose]);
  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-slate-900/50 p-4 sm:p-8" onMouseDown={(e) => e.target === e.currentTarget && onClose()}>
      <div ref={ref} tabIndex={-1} role="dialog" aria-modal="true" aria-labelledby={titleId} className={cx("w-full rounded-2xl bg-white shadow-xl outline-none", wide ? "max-w-4xl" : "max-w-lg")}>
        <div className="flex items-center justify-between border-b border-slate-200 px-5 py-3.5">
          <h2 id={titleId} className="text-base font-semibold text-slate-900">{title}</h2>
          <button onClick={onClose} aria-label="Close dialog" className="rounded-md p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600">
            <svg className="size-5" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true"><path d="M6.28 5.22a.75.75 0 00-1.06 1.06L8.94 10l-3.72 3.72a.75.75 0 101.06 1.06L10 11.06l3.72 3.72a.75.75 0 101.06-1.06L11.06 10l3.72-3.72a.75.75 0 00-1.06-1.06L10 8.94 6.28 5.22z" /></svg>
          </button>
        </div>
        <div className="p-5">{children}</div>
      </div>
    </div>
  );
}

export function ConfirmDialog({ title, body, confirmLabel, onConfirm, onClose, loading }: { title: string; body: ReactNode; confirmLabel: string; onConfirm: () => void; onClose: () => void; loading?: boolean }) {
  return (
    <Modal title={title} onClose={onClose}>
      <div className="text-sm text-slate-600">{body}</div>
      <div className="mt-5 flex justify-end gap-2">
        <Button variant="secondary" onClick={onClose}>Cancel</Button>
        <Button variant="danger" onClick={onConfirm} loading={loading}>{confirmLabel}</Button>
      </div>
    </Modal>
  );
}

// ---------- pagination ----------
export function Pagination({ meta, onPage }: { meta: PageMeta; onPage: (p: number) => void }) {
  if (meta.total_pages <= 1) return <p className="mt-4 text-xs text-slate-500">{meta.total} result{meta.total === 1 ? "" : "s"}</p>;
  const from = (meta.page - 1) * meta.page_size + 1;
  const to = Math.min(meta.page * meta.page_size, meta.total);
  return (
    <nav aria-label="Pagination" className="mt-4 flex items-center justify-between gap-3">
      <p className="text-xs text-slate-500">{from}–{to} of {meta.total}</p>
      <div className="flex items-center gap-2">
        <Button variant="secondary" size="sm" disabled={meta.page <= 1} onClick={() => onPage(meta.page - 1)}>Previous</Button>
        <span className="text-xs text-slate-500">Page {meta.page} of {meta.total_pages}</span>
        <Button variant="secondary" size="sm" disabled={meta.page >= meta.total_pages} onClick={() => onPage(meta.page + 1)}>Next</Button>
      </div>
    </nav>
  );
}

// ---------- toasts ----------
export function Toaster() {
  const { toasts, dismiss } = useToasts();
  return (
    <div aria-live="polite" className="pointer-events-none fixed inset-x-0 bottom-4 z-[60] flex flex-col items-center gap-2 px-4">
      {toasts.map((t) => (
        <div key={t.id} role={t.kind === "error" ? "alert" : "status"} className={cx("pointer-events-auto flex max-w-md items-start gap-3 rounded-lg px-4 py-3 text-sm shadow-lg ring-1", t.kind === "error" ? "bg-rose-50 text-rose-800 ring-rose-200" : "bg-emerald-50 text-emerald-800 ring-emerald-200")}>
          <span>{t.message}</span>
          <button onClick={() => dismiss(t.id)} aria-label="Dismiss" className="ml-2 opacity-60 hover:opacity-100">×</button>
        </div>
      ))}
    </div>
  );
}
