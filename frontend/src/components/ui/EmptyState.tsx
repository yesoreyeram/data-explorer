import type { ReactNode } from "react";

export interface EmptyStateProps {
  /** Optional decorative glyph shown in a rounded square above the title. */
  icon?: ReactNode;
  title: ReactNode;
  /** Explanatory line — one sentence, not a paragraph. */
  description?: ReactNode;
  /** Primary CTA (usually a `<Button variant="primary">`). */
  action?: ReactNode;
}

/** Structured empty state used inside cards, tables, and page bodies when
 * there is no data yet. Replaces the plain `.empty-state` text block for
 * callers that want an icon + title + description + CTA. */
export function EmptyState({ icon, title, description, action }: EmptyStateProps) {
  return (
    <div className="empty-state-block">
      {icon && (
        <span className="empty-state-icon" aria-hidden="true">
          {icon}
        </span>
      )}
      <div className="empty-state-title">{title}</div>
      {description && <div className="empty-state-description">{description}</div>}
      {action}
    </div>
  );
}
