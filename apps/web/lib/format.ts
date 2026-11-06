import type { Deployment, DeploymentStatus } from "@/lib/api/types";

const ACTIVE: ReadonlySet<DeploymentStatus> = new Set([
  "QUEUED",
  "CLONING",
  "DETECTING",
  "BUILDING",
  "STARTING",
  "HEALTH_CHECKING",
]);

export function isActiveStatus(status: DeploymentStatus): boolean {
  return ACTIVE.has(status);
}

export function formatWhen(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) {
    return iso;
  }
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(d);
}

export function formatDuration(dep: Deployment): string {
  const start = Date.parse(dep.createdAt);
  if (Number.isNaN(start)) {
    return "—";
  }
  const endRaw = dep.finishedAt || (isActiveStatus(dep.status) ? Date.now() : Date.parse(dep.updatedAt));
  const end = typeof endRaw === "number" ? endRaw : Date.parse(String(endRaw));
  if (Number.isNaN(end) || end < start) {
    return "—";
  }
  const seconds = Math.round((end - start) / 1000);
  if (seconds < 60) {
    return `${seconds}s`;
  }
  const minutes = Math.floor(seconds / 60);
  const rem = seconds % 60;
  return `${minutes}m ${rem}s`;
}

export function shortRepo(repoUrl: string): string {
  return repoUrl
    .replace(/^https:\/\/github\.com\//, "")
    .replace(/\.git$/, "");
}
