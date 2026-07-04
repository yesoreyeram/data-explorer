import { useEffect } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";

import { AppShell } from "./components/layout/AppShell";
import { ProtectedRoute } from "./components/ProtectedRoute";
import { LoginPage } from "./pages/LoginPage";
import { RegisterPage } from "./pages/RegisterPage";
import { DashboardPage } from "./pages/DashboardPage";
import { ExplorePage } from "./pages/ExplorePage";
import { ConnectionsPage } from "./pages/ConnectionsPage";
import { WorkflowsPage } from "./pages/WorkflowsPage";
import { WorkflowBuilderPage } from "./pages/WorkflowBuilderPage";
import { AuditLogPage } from "./pages/AuditLogPage";
import { UsersPage } from "./pages/UsersPage";
import { NotFoundPage } from "./pages/NotFoundPage";
import { useAuthStore } from "./state/authStore";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: 1, refetchOnWindowFocus: false, staleTime: 15_000 },
  },
});

export default function App() {
  const bootstrap = useAuthStore((s) => s.bootstrap);

  useEffect(() => {
    bootstrap();
  }, [bootstrap]);

  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/register" element={<RegisterPage />} />

          <Route element={<ProtectedRoute />}>
            <Route element={<AppShell />}>
              <Route path="/" element={<DashboardPage />} />
              <Route path="/explore" element={<ExplorePage />} />
              <Route path="/connections" element={<ConnectionsPage />} />
              <Route path="/workflows" element={<WorkflowsPage />} />
              <Route path="/workflows/:id" element={<WorkflowBuilderPage />} />
              <Route path="/audit-log" element={<AuditLogPage />} />
              <Route path="/users" element={<UsersPage />} />
              <Route path="*" element={<NotFoundPage />} />
            </Route>
          </Route>

          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  );
}
