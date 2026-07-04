import { forwardRef, type SelectHTMLAttributes } from "react";

export const Select = forwardRef<HTMLSelectElement, SelectHTMLAttributes<HTMLSelectElement>>(
  ({ className, ...props }, ref) => (
    <select ref={ref} className={["select", className].filter(Boolean).join(" ")} {...props} />
  ),
);
Select.displayName = "Select";
