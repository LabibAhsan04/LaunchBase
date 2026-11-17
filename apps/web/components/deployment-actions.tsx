"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

import { ApiError, redeployDeployment, rollbackDeployment, stopDeployment } from "@/lib/api/client";
import type { Deployment } from "@/lib/api/types";

export function DeploymentActions({
  deployment,
  onUpdate,
}: {
  deployment: Deployment;
  onUpdate: (next: Deployment) => void;
}) {
  const router = useRouter();
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<"stop" | "redeploy" | "rollback" | null>(null);

  async function run(kind: "stop" | "redeploy" | "rollback", action: () => Promise<Deployment>) {
    setError(null);
    setBusy(kind);
    try {
      const next = await action();
      if (kind === "stop") {
        onUpdate(next);
      } else {
        router.push(`/deployments/${next.id}`);
        router.refresh();
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Action failed.");
    } finally {
      setBusy(null);
    }
  }

  const canStop = deployment.status === "READY";
  const canRedeploy = ["READY", "FAILED", "STOPPED", "SUPERSEDED"].includes(deployment.status);
  const canRollback = ["READY", "FAILED", "STOPPED", "SUPERSEDED"].includes(deployment.status);

  return (
    <div className="flex flex-wrap items-center gap-3">
      {deployment.status === "READY" && deployment.url ? (
        <a className="btn-primary" href={deployment.url} target="_blank" rel="noreferrer">
          Open app
        </a>
      ) : null}
      {canStop ? (
        <button
          type="button"
          className="btn-secondary"
          disabled={busy !== null}
          onClick={() => void run("stop", () => stopDeployment(deployment.id))}
        >
          {busy === "stop" ? "Stopping…" : "Stop"}
        </button>
      ) : null}
      {canRedeploy ? (
        <button
          type="button"
          className="btn-secondary"
          disabled={busy !== null}
          onClick={() => void run("redeploy", () => redeployDeployment(deployment.id))}
        >
          {busy === "redeploy" ? "Queuing…" : "Redeploy"}
        </button>
      ) : null}
      {canRollback ? (
        <button
          type="button"
          className="btn-secondary"
          disabled={busy !== null}
          onClick={() => void run("rollback", () => rollbackDeployment(deployment.id))}
        >
          {busy === "rollback" ? "Queuing…" : "Roll back"}
        </button>
      ) : null}
      {error ? <p className="basis-full text-sm text-[var(--bad)]">{error}</p> : null}
    </div>
  );
}
