const TONE_BY_STATUS: Record<string, "success" | "warning" | "danger" | "neutral"> = {
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
  const tone = TONE_BY_STATUS[status] ?? "neutral";
  return <span className={`badge badge-${tone}`}>{status}</span>;
}
