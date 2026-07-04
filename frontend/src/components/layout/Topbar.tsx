import { useLocation } from "react-router-dom";

import { ThemeSwitcher } from "../ThemeSwitcher";

const SECTION_TITLES: { prefix: string; title: string }[] = [
  { prefix: "/explore", title: "Explore" },
  { prefix: "/connections", title: "Connections" },
  { prefix: "/workflows", title: "Workflows" },
  { prefix: "/audit-log", title: "Audit log" },
  { prefix: "/users", title: "Users & roles" },
];

function pageTitle(pathname: string): string {
  if (pathname === "/") return "Dashboard";
  if (/^\/workflows\/[^/]+$/.test(pathname)) return "Workflow builder";
  return SECTION_TITLES.find((s) => pathname.startsWith(s.prefix))?.title ?? "";
}

export function Topbar() {
  const location = useLocation();

  return (
    <header className="layout-topbar">
      <span className="page-title">{pageTitle(location.pathname)}</span>
      <ThemeSwitcher />
    </header>
  );
}
