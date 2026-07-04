import type { ReactNode } from "react";

export type BadgeTone = "neutral" | "success" | "warning" | "danger" | "info";
export type BadgeVariant = "dot" | "soft";

export interface BadgeProps {
  tone?: BadgeTone;
  /** Visual variant. `dot` (default) — the app's signature monochrome chip
   * with a tiny colored dot. `soft` — filled pill in the tone's soft color,
   * for row-level status like "Active" in a workflow list. */
  variant?: BadgeVariant;
  children: ReactNode;
}

/** Monochrome chip chrome with a small colored dot (or soft-filled pill)
 * carrying the semantic tone — see the ".badge" comment in app.css for the
 * rationale behind confining color to a dot in most contexts. */
export function Badge({ tone = "neutral", variant = "dot", children }: BadgeProps) {
  const classes = ["badge", `badge-${tone}`, variant === "soft" ? "badge-soft" : ""].filter(Boolean).join(" ");
  return (
    <span className={classes}>
      {variant === "dot" && tone !== "neutral" && <span className="badge-dot" aria-hidden="true" />}
      {children}
    </span>
  );
}
