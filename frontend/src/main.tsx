import { MutationCache, QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router";
import App from "./App";
import { ApiError, errorMessage } from "./api/client";
import "./index.css";
import { useAuth } from "./store/auth";
import { toast } from "./store/ui";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 15_000,
      refetchOnWindowFocus: false,
      // Client errors (4xx) will not fix themselves on retry; network/5xx blips get two more tries.
      retry: (count, e) => count < 2 && !(e instanceof ApiError && e.status >= 400 && e.status < 500),
    },
  },
  mutationCache: new MutationCache({
    onError: (e) => {
      // 422 is shown inline by the form that caused it; 401 is handled by the auth redirect.
      if (e instanceof ApiError && (e.status === 422 || e.status === 401)) return;
      toast.error(errorMessage(e));
    },
  }),
});

// Never let one user's cached data survive into the next session on a shared browser.
useAuth.subscribe((s, prev) => {
  if (s.status === "anon" && prev.status !== "anon") queryClient.clear();
});

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </QueryClientProvider>
  </StrictMode>,
);
