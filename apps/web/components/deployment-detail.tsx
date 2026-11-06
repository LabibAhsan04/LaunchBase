"use client";

import Link from "next/link";
import { useState, type ReactNode } from "react";

import { DeploymentStatusBadge } from "@/components/deployment-status";
import { ApiError, stopDeployment } from "@/lib/api/client";
import type { Deployment } from "@/lib/api/types";
import { formatDuration, formatWhen, shortRepo } from "@/lib/format";
import { useDeploymentPoll } from "@/hooks/use-deployment-poll";

export function DeploymentDetail({ initial }: { initial: Deployment }) {
  const { deployment, error: pollError, setDeployment } = useDeploymentPoll(initial);
  const [stopError, setStopError] = useState<string | null>(null);
  const [stopping, setStopping] = useState(false);

  async function onStop() {
    setStopError(null);
    setStopping(true);
    try {
      setDeployment(await stopDeployment(deployment.id));
    } catch (err) {
      setStopError(err instanceof ApiError ? err.message : "Could not stop the deployment.");
    } finally {
      setStopping(false);
    }
  }

  return (
    <div className="space-y-8">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <p className="text-sm text-[var(--muted)]">
            <Link href={`/projects/${deployment.projectId}`} className="hover:text-[var(--text)]">
              Back to project
            </Link>
          </p>
          <h1 className="mt-2 text-3xl font-semibold tracking-tight text-[var(--text)]">
            Deployment
          </h1>
          <p className="mt-2 font-mono text-sm text-[var(--muted)]">{deployment.id}</p>
        </div>
        <DeploymentStatusBadge status={deployment.status} />
      </div>

      {pollError ? <p className="text-sm text-[var(--bad)]">{pollError}</p> : null}

      <dl className="grid gap-5 rounded-xl border border-[var(--line)] bg-[var(--panel)] p-6 sm:grid-cols-2">
        <Item label="Repository" value={shortRepo(deployment.repoUrl)} />
        <Item label="Branch" value={deployment.branch || "—"} />
        <Item label="Framework" value="—" hint="Not returned by the API yet." />
        <Item label="Created" value={formatWhen(deployment.createdAt)} />
        <Item label="Duration" value={formatDuration(deployment)} />
        <Item
          label="URL"
          value={
            deployment.url ? (
              <a href={deployment.url} className="text-[var(--accent)] hover:underline" target="_blank" rel="noreferrer">
                {deployment.url}
              </a>
            ) : (
              "—"
            )
          }
        />
        {deployment.error ? (
          <div className="sm:col-span-2">
            <dt className="text-sm text-[var(--muted)]">Error</dt>
            <dd className="mt-1 text-[var(--bad)]">{deployment.error}</dd>
          </div>
        ) : null}
        {deployment.image ? <Item label="Image" value={deployment.image} mono /> : null}
        {deployment.container ? <Item label="Container" value={deployment.container} mono /> : null}
      </dl>

      {deployment.status === "READY" ? (
        <div className="flex flex-wrap items-center gap-3">
          {deployment.url ? (
            <a className="btn-primary" href={deployment.url} target="_blank" rel="noreferrer">
              Open app
            </a>
          ) : null}
          <button type="button" className="btn-secondary" onClick={() => void onStop()} disabled={stopping}>
            {stopping ? "Stopping…" : "Stop"}
          </button>
          {stopError ? <p className="basis-full text-sm text-[var(--bad)]">{stopError}</p> : null}
        </div>
      ) : null}
    </div>
  );
}

function Item({
  label,
  value,
  hint,
  mono,
}: {
  label: string;
  value: ReactNode;
  hint?: string;
  mono?: boolean;
}) {
  return (
    <div>
      <dt className="text-sm text-[var(--muted)]">{label}</dt>
      <dd className={`mt-1 text-[var(--text)] ${mono ? "font-mono text-sm break-all" : ""}`}>{value}</dd>
      {hint ? <p className="mt-1 text-xs text-[var(--muted)]">{hint}</p> : null}
    </div>
  );
}
