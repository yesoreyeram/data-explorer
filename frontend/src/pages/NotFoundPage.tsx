import { Link } from "react-router-dom";

export function NotFoundPage() {
  return (
    <div className="empty-state">
      <p className="panel-title">Page not found</p>
      <p className="panel-subtitle">
        <Link to="/">Go back to the dashboard</Link>
      </p>
    </div>
  );
}
