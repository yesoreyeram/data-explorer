import { forwardRef, type ButtonHTMLAttributes } from "react";

export interface IconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  /** Accessible name - also used as the tooltip (title). Required since the
   * button has no visible text label. */
  label: string;
}

export const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(
  ({ label, type = "button", className, children, ...props }, ref) => (
    <button
      ref={ref}
      type={type}
      className={["icon-btn", className].filter(Boolean).join(" ")}
      aria-label={label}
      title={label}
      {...props}
    >
      {children}
    </button>
  ),
);
IconButton.displayName = "IconButton";
