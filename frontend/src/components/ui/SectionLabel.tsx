import type { HTMLAttributes, ReactNode } from "react";

export interface SectionLabelProps extends HTMLAttributes<HTMLDivElement> {
  children: ReactNode;
}

/** Uppercase eyebrow label used above content groups (sidebar groups,
 * form sections, dashboard subsections). Same visual language as the
 * sidebar's `.nav-section` header but usable in any content area. */
export function SectionLabel({ children, className, ...props }: SectionLabelProps) {
  return (
    <div className={["section-label", className].filter(Boolean).join(" ")} {...props}>
      {children}
    </div>
  );
}
