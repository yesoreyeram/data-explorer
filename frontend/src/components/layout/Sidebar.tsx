import { NavLink, useNavigate } from "react-router-dom";

import { PERMISSIONS } from "../../lib/permissions";
import { NAV_ITEMS } from "../../lib/navigation";
import { useAuthStore } from "../../state/authStore";
import { useNavigationStore } from "../../state/navigationStore";
import { useSidebarStore } from "../../state/sidebarStore";
import { PermissionGate } from "../PermissionGate";
import { IconDatabase, IconHome, IconLogout, IconPanelLeft, IconSearch, IconShield, IconStar, IconUsers, IconWorkflow } from "../icons";
import { IconButton } from "../ui";

const linkClass = ({ isActive }: { isActive: boolean }) => "nav-link" + (isActive ? " active" : "");

function navIcon(href: string) {
  switch (href) {
    case "/":
      return <IconHome className="icon" />;
    case "/explore":
      return <IconSearch className="icon" />;
    case "/connections":
      return <IconDatabase className="icon" />;
    case "/workflows":
      return <IconWorkflow className="icon" />;
    case "/audit-log":
      return <IconShield className="icon" />;
    case "/users":
      return <IconUsers className="icon" />;
    default:
      return <IconStar className="icon" />;
  }
}

export function Sidebar() {
  const collapsed = useSidebarStore((s) => s.collapsed);
  const toggle = useSidebarStore((s) => s.toggle);
  const favorites = useNavigationStore((s) => s.favorites);
  const hasPermission = useAuthStore((s) => s.hasPermission);
  const user = useAuthStore((s) => s.user);
  const logout = useAuthStore((s) => s.logout);
  const navigate = useNavigate();

  async function handleLogout() {
    await logout();
    navigate("/login", { replace: true });
  }

  const initial = user?.displayName?.trim()?.[0]?.toUpperCase() ?? "?";
  const favoriteItems = NAV_ITEMS.filter((item) => favorites.includes(item.href) && (!item.permission || hasPermission(item.permission)));

  return (
    <aside className={"layout-sidebar" + (collapsed ? " sidebar-collapsed" : "")}>
      <div className="brand">
        <span className="brand-mark" aria-hidden="true" />
        <span>Data Explorer</span>
      </div>

      <nav className="sidebar-nav">
        {favoriteItems.length > 0 && (
          <>
            <div className="nav-section">Favorites</div>
            {favoriteItems.map((item) => (
              <NavLink key={item.href} to={item.href} end={item.href === "/"} className={linkClass} title={item.title}>
                {navIcon(item.href)} <span>{item.title}</span>
              </NavLink>
            ))}
          </>
        )}

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
        <IconButton label={collapsed ? "Expand sidebar" : "Collapse sidebar"} onClick={toggle} className="sidebar-collapse-toggle">
          <IconPanelLeft width={14} height={14} />
        </IconButton>
      </div>
    </aside>
  );
}
