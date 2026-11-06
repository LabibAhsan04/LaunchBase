"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

import { DeployForm } from "@/components/deploy-form";
import { DeploymentStatusBadge } from "@/components/deployment-status";
import { StateBlock } from "@/components/state-block";
import { ApiError, getProject } from "@/lib/api/client";
import type { Project } from "@/lib/api/types";
import { formatDuration, formatWhen, isActiveStatus, shortRepo } from "@/lib/format";

export function ProjectDetail({ initial }: { initial: Project }) {
  const [project, setProject] = useState(initial);
  const [error, setError] = useState<string | null>(null);

  const hasActive = Boolean(project.deployments?.some((d) => isActiveStatus(d.status)));

  useEffect(() => {
    if (!hasActive) {
      return;
    }
    let cancelled = false;
    const id = window.setInterval(() => {
      void (async () => {
        try {
          const next = await getProject(project.id);
          if (!cancelled) {
            setProject(next);
            setError(null);
          }
        } catch (err) {
          if (!cancelled) {
            setError(err instanceof ApiError ? err.message : "Could not refresh the project.");
          }
        }
      })();
    }, 2000);
    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
  }, [project.id, hasActive]);

  const deployments = project.deployments ?? [];

  return (
    <div className="space-y-10">
      <div>
        <p className="text-sm text-[var(--muted)]">
          <Link href="/" className="hover:text-[var(--text)]">
            Projects
          </Link>
        </p>
        <h1 className="mt-2 text-3xl font-semibold tracking-tight text-[var(--text)]">{project.name}</h1>
        <p className="mt-2 text-[var(--muted)]">
          {shortRepo(project.repoUrl)} · {project.branch || "default branch"}
        </p>
      </div>

      {error ? <p className="text-sm text-[var(--bad)]">{error}</p> : null}

      <section className="space-y-4">
        <h2 className="text-lg font-medium text-[var(--text)]">Deploy</h2>
        <p className="text-sm text-[var(--muted)]">
          Queues a build. Keep <code className="text-[var(--text)]">make worker</code> running so it can leave QUEUED.
        </p>
        <DeployForm projectId={project.id} defaultBranch={project.branch || "main"} />
      </section>

      <section className="space-y-4">
        <h2 className="text-lg font-medium text-[var(--text)]">Deployments</h2>
        {deployments.length === 0 ? (
          <StateBlock title="No deployments" body="Deploy this project to see status, URL, and errors here." />
        ) : (
          <ul className="divide-y divide-[var(--line)] rounded-xl border border-[var(--line)] bg-[var(--panel)]">
            {deployments.map((dep) => (
              <li key={dep.id}>
                <Link
                  href={`/deployments/${dep.id}`}
                  className="flex flex-col gap-3 px-5 py-4 hover:bg-[var(--panel-hover)] sm:flex-row sm:items-center sm:justify-between"
                >
                  <div className="min-w-0 space-y-1">
                    <div className="flex flex-wrap items-center gap-3">
                      <DeploymentStatusBadge status={dep.status} />
                      <span className="font-mono text-sm text-[var(--muted)]">{dep.branch}</span>
                    </div>
                    <p className="truncate text-sm text-[var(--muted)]">
                      {dep.url || dep.error || "Waiting for the worker"}
                    </p>
                  </div>
                  <div className="shrink-0 text-sm text-[var(--muted)]">
                    <p>{formatWhen(dep.createdAt)}</p>
                    <p className="mt-1">{formatDuration(dep)}</p>
                  </div>
                </Link>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
