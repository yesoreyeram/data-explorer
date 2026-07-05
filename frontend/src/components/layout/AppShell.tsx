import { useEffect } from "react";
import { Outlet, useLocation } from "react-router-dom";

import { pageTitle } from "../../lib/navigation";
import { useGlobalNavigation } from "../../hooks/useGlobalNavigation";
import { useNavigationStore } from "../../state/navigationStore";
import { useSidebarStore } from "../../state/sidebarStore";
import { CommandPalette } from "./CommandPalette";
import { RecentActivityDrawer } from "./RecentActivityDrawer";
import { Sidebar } from "./Sidebar";
import { Topbar } from "./Topbar";

export function AppShell() {
  const collapsed = useSidebarStore((s) => s.collapsed);
  const recordVisit = useNavigationStore((s) => s.recordVisit);
  const location = useLocation();

  useGlobalNavigation();

  useEffect(() => {
    recordVisit(location.pathname, pageTitle(location.pathname) || location.pathname);
  }, [location.pathname, recordVisit]);

  return (
    <>
      <div className={"layout-shell" + (collapsed ? " sidebar-collapsed" : "")}>
        <Sidebar />
        <Topbar />
        <main className="layout-content">
          <Outlet />
        </main>
      </div>
      <CommandPalette />
      <RecentActivityDrawer />
    </>
  );
}
