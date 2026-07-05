import { useNavigate } from "react-router-dom";

import { Button } from "../ui";
import { loadExploreHistory } from "../../lib/exploreHistory";
import { useNavigationStore } from "../../state/navigationStore";

export function RecentActivityDrawer() {
  const navigate = useNavigate();
  const open = useNavigationStore((s) => s.recentDrawerOpen);
  const close = useNavigationStore((s) => s.closeRecentDrawer);
  const recentRoutes = useNavigationStore((s) => s.recentRoutes);
  const recentQueries = loadExploreHistory().slice(0, 6);

  return (
    <>
      <div className={"drawer-backdrop" + (open ? " open" : "")} onClick={close} aria-hidden={!open} />
      <aside className={"activity-drawer" + (open ? " open" : "")} aria-hidden={!open}>
        <div className="activity-drawer-header">
          <strong>Recent activity</strong>
          <Button size="sm" variant="ghost" onClick={close}>
            Close
          </Button>
        </div>
        <div className="activity-drawer-section">
          <div className="nav-section">Recent pages</div>
          {recentRoutes.length === 0 ? (
            <p className="field-hint">Navigate a few screens to build a quick jump list.</p>
          ) : (
            recentRoutes.map((item) => (
              <button
                key={item.href}
                type="button"
                className="activity-row activity-row-button"
                onClick={() => {
                  navigate(item.href);
                  close();
                }}
              >
                <span>{item.title}</span>
                <span>{new Date(item.visitedAt).toLocaleTimeString()}</span>
              </button>
            ))
          )}
        </div>
        <div className="activity-drawer-section">
          <div className="nav-section">Recent queries</div>
          {recentQueries.length === 0 ? (
            <p className="field-hint">Run an Explore query to pin it here.</p>
          ) : (
            recentQueries.map((item) => (
              <div key={item.id} className="activity-row">
                <span className="mono">{item.summary || "(empty query)"}</span>
                <span>{new Date(item.ranAt).toLocaleTimeString()}</span>
              </div>
            ))
          )}
        </div>
      </aside>
    </>
  );
}
