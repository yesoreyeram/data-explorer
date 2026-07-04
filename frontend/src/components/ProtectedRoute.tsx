import { Navigate, Outlet, useLocation } from "react-router-dom";

import { useAuthStore } from "../state/authStore";

export function ProtectedRoute() {
  const status = useAuthStore((s) => s.status);
  const location = useLocation();

  if (status === "idle" || status === "loading") {
    return (
      <div className="auth-shell">
        <div className="spinner" />
      </div>
    );
  }

  if (status === "anonymous") {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  return <Outlet />;
}
