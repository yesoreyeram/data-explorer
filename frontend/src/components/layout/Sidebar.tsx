import { NavLink } from "react-router-dom";

import { PermissionGate } from "../PermissionGate";
import { PERMISSIONS } from "../../lib/permissions";
import { IconDatabase, IconHome, IconShield, IconUsers, IconWorkflow } from "../icons";

const linkClass = ({ isActive }: { isActive: boolean }) => "nav-link" + (isActive ? " active" : "");

export function Sidebar() {
  return (
    <aside className="layout-sidebar">
      <div className="brand">
        <span className="brand-mark" aria-hidden="true" />
        Data Explorer
      </div>

      <div className="nav-section">Workspace</div>
      <NavLink to="/" end className={linkClass}>
        <IconHome className="icon" /> Overview
      </NavLink>
      <PermissionGate permission={PERMISSIONS.connectionsRead}>
        <NavLink to="/connections" className={linkClass}>
          <IconDatabase className="icon" /> Connections
        </NavLink>
      </PermissionGate>
      <PermissionGate permission={PERMISSIONS.workflowsRead}>
        <NavLink to="/workflows" className={linkClass}>
          <IconWorkflow className="icon" /> Workflows
        </NavLink>
      </PermissionGate>

      <PermissionGate permission={PERMISSIONS.auditRead}>
        <div className="nav-section">Governance</div>
        <NavLink to="/audit-log" className={linkClass}>
          <IconShield className="icon" /> Audit Log
        </NavLink>
      </PermissionGate>
      <PermissionGate permission={PERMISSIONS.usersRead}>
        <NavLink to="/users" className={linkClass}>
          <IconUsers className="icon" /> Users & Roles
        </NavLink>
      </PermissionGate>
    </aside>
  );
}
