import type { ReactNode } from "react";

export interface PageHeaderProps {
  title: ReactNode;
  subtitle?: ReactNode;
  /** Right-aligned actions slot — buttons, filters, date pickers. Stays on
   * the same row as the title (aligned to the title's top edge) on wide
   * viewports; wraps naturally on narrow. */
  actions?: ReactNode;
}

/** Consistent page-level heading block. Replaces the ad-hoc
 * `<div className="page-header"><h1>…</h1><p>…</p></div>` markup that used
 * to be duplicated at every page. */
export function PageHeader({ title, subtitle, actions }: PageHeaderProps) {
  return (
    <div className="page-header">
      <div>
        <h1 className="panel-title">{title}</h1>
        {subtitle && <p className="panel-subtitle">{subtitle}</p>}
      </div>
      {actions && <div className="page-header-actions">{actions}</div>}
    </div>
  );
}
