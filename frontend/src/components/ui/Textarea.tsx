import { forwardRef, type TextareaHTMLAttributes } from "react";

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaHTMLAttributes<HTMLTextAreaElement>>(
  ({ className, rows = 5, ...props }, ref) => (
    <textarea ref={ref} rows={rows} className={["textarea", className].filter(Boolean).join(" ")} {...props} />
  ),
);
Textarea.displayName = "Textarea";
