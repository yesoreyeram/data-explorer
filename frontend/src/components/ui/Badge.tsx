import type { ReactNode } from "react";

export type BadgeTone = "neutral" | "success" | "warning" | "danger";

export interface BadgeProps {
  tone?: BadgeTone;
  children: ReactNode;
}

/** Monochrome chip chrome with a small colored dot carrying the semantic
 * tone - see the ".badge" comment in app.css for why. */
export function Badge({ tone = "neutral", children }: BadgeProps) {
  return (
    <span className={`badge badge-${tone}`}>
      {tone !== "neutral" && <span className="badge-dot" aria-hidden="true" />}
      {children}
    </span>
  );
}
