import type { DeploymentStatus } from "@/lib/api/types";

const LABELS: Record<DeploymentStatus, string> = {
  QUEUED: "Queued",
  CLONING: "Cloning",
  DETECTING: "Detecting",
  BUILDING: "Building",
  STARTING: "Starting",
  HEALTH_CHECKING: "Health check",
  READY: "Ready",
  FAILED: "Failed",
  STOPPED: "Stopped",
  SUPERSEDED: "Superseded",
};

const TONE: Record<DeploymentStatus, string> = {
  QUEUED: "tone-muted",
  CLONING: "tone-info",
  DETECTING: "tone-info",
  BUILDING: "tone-info",
  STARTING: "tone-info",
  HEALTH_CHECKING: "tone-info",
  READY: "tone-ok",
  FAILED: "tone-bad",
  STOPPED: "tone-muted",
  SUPERSEDED: "tone-muted",
};

export function DeploymentStatusBadge({ status }: { status: DeploymentStatus }) {
  return (
    <span
      className={`inline-flex items-center rounded-md px-2.5 py-1 text-xs font-medium tracking-wide ${TONE[status]}`}
      data-status={status}
    >
      {LABELS[status]}
    </span>
  );
}

export function statusLabel(status: DeploymentStatus): string {
  return LABELS[status];
}
