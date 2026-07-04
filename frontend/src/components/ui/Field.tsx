import type { ReactNode } from "react";

export interface FieldProps {
  /** Must match the id of the control rendered as children. */
  htmlFor: string;
  label: ReactNode;
  hint?: ReactNode;
  children: ReactNode;
  style?: React.CSSProperties;
}

/** Label + control + hint, laid out consistently - replaces the hand-rolled
 * `<div className="field"><label>...</label>{control}</div>` markup that
 * used to be repeated at every call site. */
export function Field({ htmlFor, label, hint, children, style }: FieldProps) {
  return (
    <div className="field" style={style}>
      <label htmlFor={htmlFor}>{label}</label>
      {children}
      {hint && <span className="field-hint">{hint}</span>}
    </div>
  );
}
