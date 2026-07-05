import { Link, useLocation } from "react-router-dom";

import { breadcrumbsForPath, pageTitle } from "../../lib/navigation";
import { useNavigationStore } from "../../state/navigationStore";
import { ThemeSwitcher } from "../ThemeSwitcher";
import { IconClock, IconSearch, IconStar } from "../icons";
import { Button, IconButton } from "../ui";

export function Topbar() {
  const location = useLocation();
  const breadcrumbs = breadcrumbsForPath(location.pathname);
  const favorites = useNavigationStore((s) => s.favorites);
  const toggleFavorite = useNavigationStore((s) => s.toggleFavorite);
  const openCommandPalette = useNavigationStore((s) => s.openCommandPalette);
  const openRecentDrawer = useNavigationStore((s) => s.openRecentDrawer);
  const favoriteHref = breadcrumbs[1]?.href ?? location.pathname;
  const isFavorite = favorites.includes(favoriteHref);

  return (
    <header className="layout-topbar">
      <div>
        <div className="topbar-breadcrumbs">
          {breadcrumbs.map((crumb, index) => (
            <span key={crumb.href} className="topbar-breadcrumb">
              {index > 0 && <span className="topbar-breadcrumb-sep">/</span>}
              {index === breadcrumbs.length - 1 ? <span>{crumb.title}</span> : <Link to={crumb.href}>{crumb.title}</Link>}
            </span>
          ))}
        </div>
        <span className="page-title">{pageTitle(location.pathname)}</span>
      </div>
      <div className="topbar-actions">
        <IconButton label={isFavorite ? "Remove from favorites" : "Add to favorites"} onClick={() => toggleFavorite(favoriteHref)}>
          <IconStar width={14} height={14} filled={isFavorite} />
        </IconButton>
        <IconButton label="Open recent activity" onClick={openRecentDrawer}>
          <IconClock width={14} height={14} />
        </IconButton>
        <Button className="topbar-command-button" onClick={openCommandPalette}>
          <IconSearch width={13} height={13} />
          <span>Search</span>
          <span className="topbar-command-shortcut">⌘K</span>
        </Button>
        <ThemeSwitcher />
      </div>
    </header>
  );
}
