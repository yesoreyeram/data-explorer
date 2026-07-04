import { Badge, type BadgeTone } from "./ui";

const TONE_BY_STATUS: Record<string, BadgeTone> = {
  healthy: "success",
  succeeded: "success",
  active: "success",
  unverified: "warning",
  running: "warning",
  unhealthy: "danger",
  failed: "danger",
  suspended: "danger",
  draft: "neutral",
  published: "success",
};

export function StatusBadge({ status }: { status: string }) {
  return <Badge tone={TONE_BY_STATUS[status] ?? "neutral"}>{status}</Badge>;
}
