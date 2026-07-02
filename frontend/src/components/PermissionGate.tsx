import type { ReactNode } from "react";

import { useAuthStore } from "../state/authStore";

interface PermissionGateProps {
  permission: string;
  children: ReactNode;
  fallback?: ReactNode;
}

export function PermissionGate({ permission, children, fallback = null }: PermissionGateProps) {
  const hasPermission = useAuthStore((s) => s.hasPermission(permission));
  return hasPermission ? <>{children}</> : <>{fallback}</>;
}
