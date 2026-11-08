import { useEffect } from "react";
import { Link, Navigate, Outlet, Route, Routes, useLocation } from "react-router";
import { refreshSession } from "./api/client";
import { useOrgs } from "./api/hooks";
import { Toaster, PageSpinner } from "./components/ui";
import OrgLayout from "./components/OrgLayout";
import Dashboard from "./pages/Dashboard";
import { Login, Register } from "./pages/AuthPages";
import Members from "./pages/Members";
import OrgPicker from "./pages/OrgPicker";
import ProjectPage from "./pages/ProjectPage";
import Projects from "./pages/Projects";
import Settings from "./pages/Settings";
import Tasks from "./pages/Tasks";
import { useAuth } from "./store/auth";
import { useUi } from "./store/ui";

function RequireAuth() {
  const status = useAuth((s) => s.status);
  const location = useLocation();
  if (status === "loading") return <PageSpinner label="Restoring your session…" />;
  if (status === "anon") return <Navigate to="/login" replace state={{ from: location.pathname + location.search }} />;
  return <Outlet />;
}

// Where to land after sign-in: the org used last time, else the only org the user has, else the picker.
function Home() {
  const last = useUi((s) => s.lastOrgId);
  const orgs = useOrgs();
  if (last) return <Navigate to={`/o/${last}`} replace />;
  if (orgs.isPending) return <PageSpinner />;
  return <Navigate to={orgs.data?.length === 1 ? `/o/${orgs.data[0].id}` : "/orgs"} replace />;
}

function NotFound() {
  return (
    <div className="mx-auto max-w-md px-4 py-24 text-center">
      <p className="text-5xl font-bold text-indigo-600">404</p>
      <p className="mt-3 text-lg font-semibold text-slate-900">Page not found</p>
      <Link to="/" className="mt-6 inline-block text-sm font-medium text-indigo-600 hover:text-indigo-500">Go home</Link>
    </div>
  );
}

export default function App() {
  // Restore the session from the httpOnly refresh cookie. refreshSession() is single-flight,
  // so StrictMode's double-invoked effect in development performs only one request.
  useEffect(() => {
    refreshSession().then((ok) => {
      if (!ok) useAuth.getState().clear();
    });
  }, []);

  return (
    <>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="/register" element={<Register />} />
        <Route element={<RequireAuth />}>
          <Route index element={<Home />} />
          <Route path="orgs" element={<OrgPicker />} />
          <Route path="o/:orgId" element={<OrgLayout />}>
            <Route index element={<Dashboard />} />
            <Route path="projects" element={<Projects />} />
            <Route path="projects/:projectId" element={<ProjectPage />} />
            <Route path="tasks" element={<Tasks />} />
            <Route path="members" element={<Members />} />
            <Route path="settings" element={<Settings />} />
          </Route>
        </Route>
        <Route path="*" element={<NotFound />} />
      </Routes>
      <Toaster />
    </>
  );
}
