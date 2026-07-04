import type { HTMLAttributes, ReactNode } from "react";

export interface KbdProps extends HTMLAttributes<HTMLElement> {
  children: ReactNode;
}

/** Keyboard-shortcut chip. Compose several to render sequences:
 *   <><Kbd>⌘</Kbd><Kbd>K</Kbd></>
 * Renders as a semantic <kbd> so it is announced correctly by AT. */
export function Kbd({ children, className, ...props }: KbdProps) {
  return (
    <kbd className={["kbd", className].filter(Boolean).join(" ")} {...props}>
      {children}
    </kbd>
  );
}
