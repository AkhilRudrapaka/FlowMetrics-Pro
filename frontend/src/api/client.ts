import { useAuth } from "../store/auth";
import { useUi } from "../store/ui";
import type { Paged, PageMeta, User } from "./types";

const BASE = "/api/v1";

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    public details?: Record<string, string>,
  ) {
    super(message);
  }
}

type Params = Record<string, string | number | boolean | undefined | null>;

function query(params?: Params) {
  const u = new URLSearchParams();
  for (const [k, v] of Object.entries(params ?? {})) {
    if (v === undefined || v === null || v === "" || v === false) continue;
    u.set(k, String(v));
  }
  const s = u.toString();
  return s ? `?${s}` : "";
}

interface Envelope<T> {
  data: T;
  meta?: PageMeta;
}

// ---- silent token refresh ----
// Every concurrent 401 (and React StrictMode's double effect) shares ONE refresh request.
// That matters: refresh tokens rotate, so two parallel refreshes would look like token theft.
let inflight: Promise<boolean> | null = null;

async function doRefresh(): Promise<boolean> {
  try {
    const res = await fetch(`${BASE}/auth/refresh`, { method: "POST", credentials: "same-origin" });
    if (!res.ok) return false;
    const { data } = (await res.json()) as Envelope<{ user: User; access_token: string }>;
    useAuth.getState().setSession(data.user, data.access_token);
    return true;
  } catch {
    return false;
  }
}

export function refreshSession(): Promise<boolean> {
  if (!inflight) inflight = doRefresh().finally(() => (inflight = null));
  return inflight;
}

async function request<T>(method: string, path: string, body?: unknown, params?: Params, canRetry = true): Promise<Envelope<T>> {
  const token = useAuth.getState().token;
  let res: Response;
  try {
    res = await fetch(BASE + path + query(params), {
      method,
      credentials: "same-origin",
      headers: {
        ...(body !== undefined ? { "Content-Type": "application/json" } : {}),
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
      },
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });
  } catch {
    throw new ApiError(0, "network_error", "Cannot reach the server. Check your connection and try again.");
  }

  // Expired access token: refresh once, replay the request once. /auth/* is excluded so a wrong
  // password (401) is reported as such instead of triggering a refresh.
  if (res.status === 401 && canRetry && !path.startsWith("/auth/")) {
    if (await refreshSession()) return request<T>(method, path, body, params, false);
    useAuth.getState().clear();
  }
  if (res.status === 204) return { data: undefined as T };

  const json = await res.json().catch(() => null);
  if (!res.ok) {
    const e = json?.error;
    throw new ApiError(res.status, e?.code ?? "unknown", e?.message ?? res.statusText, e?.details);
  }
  return json as Envelope<T>;
}

export const api = {
  get: <T>(path: string, params?: Params) => request<T>("GET", path, undefined, params).then((r) => r.data),
  page: <T>(path: string, params?: Params) =>
    request<T[]>("GET", path, undefined, params).then((r): Paged<T> => ({ items: r.data, meta: r.meta! })),
  post: <T>(path: string, body?: unknown) => request<T>("POST", path, body).then((r) => r.data),
  put: <T>(path: string, body?: unknown) => request<T>("PUT", path, body).then((r) => r.data),
  patch: <T>(path: string, body: unknown) => request<T>("PATCH", path, body).then((r) => r.data),
  del: (path: string) => request<void>("DELETE", path).then(() => undefined),
};

export async function login(email: string, password: string) {
  const d = await api.post<{ user: User; access_token: string }>("/auth/login", { email, password });
  useAuth.getState().setSession(d.user, d.access_token);
}

export async function register(name: string, email: string, password: string) {
  const d = await api.post<{ user: User; access_token: string }>("/auth/register", { name, email, password });
  useAuth.getState().setSession(d.user, d.access_token);
}

export async function logout() {
  try {
    await api.post("/auth/logout");
  } finally {
    useAuth.getState().clear();
    useUi.getState().setLastOrg(null); // the next person on this browser must not be sent to this user's workspace
  }
}

/** Human-readable text for any thrown value. */
export function errorMessage(e: unknown): string {
  if (e instanceof ApiError) {
    const first = e.details && Object.entries(e.details)[0];
    return first ? `${first[0].replace(/_/g, " ")} ${first[1]}` : e.message;
  }
  return "Something went wrong. Please try again.";
}
