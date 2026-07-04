import { PERMISSIONS } from "./permissions";

export interface NavigationItem {
  href: string;
  title: string;
  permission?: string;
}

export const NAV_ITEMS: NavigationItem[] = [
  { href: "/", title: "Dashboard" },
  { href: "/explore", title: "Explore", permission: PERMISSIONS.connectionsRead },
  { href: "/connections", title: "Connections", permission: PERMISSIONS.connectionsRead },
  { href: "/workflows", title: "Workflows", permission: PERMISSIONS.workflowsRead },
  { href: "/audit-log", title: "Audit log", permission: PERMISSIONS.auditRead },
  { href: "/users", title: "Users & roles", permission: PERMISSIONS.usersRead },
];

export function pageTitle(pathname: string): string {
  if (pathname === "/") return "Dashboard";
  if (/^\/workflows\/[^/]+$/.test(pathname)) return "Workflow builder";
  return NAV_ITEMS.find((item) => pathname === item.href || pathname.startsWith(`${item.href}/`))?.title ?? "";
}

export function breadcrumbsForPath(pathname: string): NavigationItem[] {
  if (pathname === "/") return [{ href: "/", title: "Dashboard" }];
  if (/^\/workflows\/[^/]+$/.test(pathname)) {
    return [
      { href: "/", title: "Dashboard" },
      { href: "/workflows", title: "Workflows" },
      { href: pathname, title: "Builder" },
    ];
  }
  const item = NAV_ITEMS.find((entry) => entry.href !== "/" && (pathname === entry.href || pathname.startsWith(`${entry.href}/`)));
  return [{ href: "/", title: "Dashboard" }, ...(item ? [item] : [])];
}
