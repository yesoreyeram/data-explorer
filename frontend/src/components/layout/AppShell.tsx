import { Outlet } from "react-router-dom";

import { Sidebar } from "./Sidebar";
import { Topbar } from "./Topbar";

export function AppShell() {
  return (
    <div className="layout-shell">
      <Sidebar />
      <Topbar />
      <main className="layout-content">
        <Outlet />
      </main>
    </div>
  );
}
