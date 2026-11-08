import { create } from "zustand";
import { persist } from "zustand/middleware";

// Only a harmless convenience is persisted: which organization to open next time.
export const useUi = create<{ lastOrgId: string | null; setLastOrg: (id: string | null) => void }>()(
  persist((set) => ({ lastOrgId: null, setLastOrg: (id) => set({ lastOrgId: id }) }), { name: "fm-ui" }),
);

export interface Toast {
  id: number;
  kind: "success" | "error";
  message: string;
}

let nextId = 1;

export const useToasts = create<{ toasts: Toast[]; dismiss: (id: number) => void }>((set) => ({
  toasts: [],
  dismiss: (id) => set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) })),
}));

function push(kind: Toast["kind"], message: string) {
  const id = nextId++;
  useToasts.setState((s) => ({ toasts: [...s.toasts, { id, kind, message }] }));
  setTimeout(() => useToasts.getState().dismiss(id), 4500);
}

export const toast = {
  success: (m: string) => push("success", m),
  error: (m: string) => push("error", m),
};
