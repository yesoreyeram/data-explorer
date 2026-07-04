import { forwardRef, type ButtonHTMLAttributes } from "react";

export type ButtonVariant = "default" | "primary" | "danger" | "ghost";
export type ButtonSize = "md" | "sm";

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
}

const VARIANT_CLASS: Record<ButtonVariant, string> = {
  default: "",
  primary: "btn-primary",
  danger: "btn-danger",
  ghost: "btn-ghost",
};

/** Every button in the app should be this, not a raw <button className="btn">
 * - it fixes type="button" by default (forms opt into submit explicitly) and
 * keeps variant/size as a closed set instead of ad hoc className strings. */
export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ variant = "default", size = "md", type = "button", className, ...props }, ref) => {
    const classes = ["btn", VARIANT_CLASS[variant], size === "sm" ? "btn-sm" : "", className].filter(Boolean).join(" ");
    return <button ref={ref} type={type} className={classes} {...props} />;
  },
);
Button.displayName = "Button";
