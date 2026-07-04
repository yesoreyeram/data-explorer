import { NavLink, useNavigate } from "react-router-dom";

import { PermissionGate } from "../PermissionGate";
import { PERMISSIONS } from "../../lib/permissions";
import { useAuthStore } from "../../state/authStore";
import { useSidebarStore } from "../../state/sidebarStore";
import { IconDatabase, IconHome, IconLogout, IconPanelLeft, IconSearch, IconShield, IconUsers, IconWorkflow } from "../icons";
import { IconButton } from "../ui";

const linkClass = ({ isActive }: { isActive: boolean }) => "nav-link" + (isActive ? " active" : "");

export function Sidebar() {
  const collapsed = useSidebarStore((s) => s.collapsed);
  const toggle = useSidebarStore((s) => s.toggle);
  const user = useAuthStore((s) => s.user);
  const logout = useAuthStore((s) => s.logout);
  const navigate = useNavigate();

  async function handleLogout() {
    await logout();
    navigate("/login", { replace: true });
  }

  const initial = user?.displayName?.trim()?.[0]?.toUpperCase() ?? "?";

  return (
    <aside className={"layout-sidebar" + (collapsed ? " sidebar-collapsed" : "")}>
      <div className="brand">
        <span className="brand-mark" aria-hidden="true" />
        <span>Data Explorer</span>
      </div>

      <nav className="sidebar-nav">
        <div className="nav-section">Workspace</div>
        <NavLink to="/" end className={linkClass} title="Dashboard">
          <IconHome className="icon" /> <span>Dashboard</span>
        </NavLink>
        <PermissionGate permission={PERMISSIONS.connectionsRead}>
          <NavLink to="/explore" className={linkClass} title="Explore">
            <IconSearch className="icon" /> <span>Explore</span>
          </NavLink>
        </PermissionGate>
        <PermissionGate permission={PERMISSIONS.connectionsRead}>
          <NavLink to="/connections" className={linkClass} title="Connections">
            <IconDatabase className="icon" /> <span>Connections</span>
          </NavLink>
        </PermissionGate>
        <PermissionGate permission={PERMISSIONS.workflowsRead}>
          <NavLink to="/workflows" className={linkClass} title="Workflows">
            <IconWorkflow className="icon" /> <span>Workflows</span>
          </NavLink>
        </PermissionGate>

        <PermissionGate permission={PERMISSIONS.auditRead}>
          <div className="nav-section">Governance</div>
          <NavLink to="/audit-log" className={linkClass} title="Audit log">
            <IconShield className="icon" /> <span>Audit log</span>
          </NavLink>
        </PermissionGate>
        <PermissionGate permission={PERMISSIONS.usersRead}>
          <NavLink to="/users" className={linkClass} title="Users & roles">
            <IconUsers className="icon" /> <span>Users & roles</span>
          </NavLink>
        </PermissionGate>
      </nav>

      <div className="sidebar-footer">
        {!collapsed && user && (
          <>
            <span className="sidebar-user-avatar" aria-hidden="true">
              {initial}
            </span>
            <span className="sidebar-user-info">{user.displayName}</span>
          </>
        )}
        <IconButton label="Log out" onClick={handleLogout}>
          <IconLogout width={14} height={14} />
        </IconButton>
      </div>

      <div className="sidebar-toggle-row">
        <IconButton
          label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
          onClick={toggle}
          className="sidebar-collapse-toggle"
        >
          <IconPanelLeft width={14} height={14} />
        </IconButton>
      </div>
    </aside>
  );
}
