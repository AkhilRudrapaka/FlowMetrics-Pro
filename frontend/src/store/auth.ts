import { create } from "zustand";
import type { User } from "../api/types";

// The access token lives ONLY in memory (never localStorage), so XSS cannot exfiltrate a long-lived
// credential. A page reload recovers the session through the httpOnly refresh cookie.
interface AuthState {
  user: User | null;
  token: string | null;
  status: "loading" | "authed" | "anon";
  setSession: (user: User, token: string) => void;
  clear: () => void;
}

export const useAuth = create<AuthState>((set) => ({
  user: null,
  token: null,
  status: "loading",
  setSession: (user, token) => set({ user, token, status: "authed" }),
  clear: () => set({ user: null, token: null, status: "anon" }),
}));
