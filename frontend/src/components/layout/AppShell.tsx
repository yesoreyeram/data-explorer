import { Outlet } from "react-router-dom";

import { useSidebarStore } from "../../state/sidebarStore";
import { Sidebar } from "./Sidebar";
import { Topbar } from "./Topbar";

export function AppShell() {
  const collapsed = useSidebarStore((s) => s.collapsed);

  return (
    <div className={"layout-shell" + (collapsed ? " sidebar-collapsed" : "")}>
      <Sidebar />
      <Topbar />
      <main className="layout-content">
        <Outlet />
      </main>
    </div>
  );
}
