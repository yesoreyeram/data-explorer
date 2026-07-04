import type { HTMLAttributes, ReactNode } from "react";

export function Card({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={["card", className].filter(Boolean).join(" ")} {...props} />;
}

export function CardHeader({ children, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div className="card-header" {...props}>
      {children}
    </div>
  );
}

export function CardBody({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={["card-body", className].filter(Boolean).join(" ")} {...props} />;
}

export function CardTitle({ children }: { children: ReactNode }) {
  return <h3>{children}</h3>;
}
