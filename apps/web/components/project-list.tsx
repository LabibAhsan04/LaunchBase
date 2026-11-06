"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";

import { LoadingBlock, StateBlock } from "@/components/state-block";
import { ApiError, listProjects } from "@/lib/api/client";
import type { Project } from "@/lib/api/types";
import { formatWhen, shortRepo } from "@/lib/format";

export function ProjectList({
  initial,
  initialError,
}: {
  initial: Project[] | null;
  initialError: string | null;
}) {
  const router = useRouter();
  const [projects, setProjects] = useState(initial);
  const [error, setError] = useState(initialError);
  const [loading, setLoading] = useState(false);

  async function retry() {
    setLoading(true);
    setError(null);
    try {
      setProjects(await listProjects());
    } catch (err) {
      setProjects(null);
      setError(err instanceof ApiError ? err.message : "Could not load projects.");
    } finally {
      setLoading(false);
    }
  }

  if (loading) {
    return <LoadingBlock label="Loading projects…" />;
  }

  if (error) {
    return (
      <StateBlock title="Could not load projects" body={error} actionLabel="Retry" onAction={() => void retry()} />
    );
  }

  if (!projects || projects.length === 0) {
    return (
      <StateBlock
        title="No projects yet"
        body="Create a project with a public GitHub repository URL. Deployments are queued by the API and built by the worker."
        actionLabel="New project"
        onAction={() => router.push("/projects/new")}
      />
    );
  }

  return (
    <ul className="divide-y divide-[var(--line)] rounded-xl border border-[var(--line)] bg-[var(--panel)]">
      {projects.map((project) => (
        <li key={project.id}>
          <Link
            href={`/projects/${project.id}`}
            className="flex flex-col gap-1 px-5 py-4 hover:bg-[var(--panel-hover)] sm:flex-row sm:items-center sm:justify-between"
          >
            <div>
              <p className="font-medium text-[var(--text)]">{project.name}</p>
              <p className="mt-1 text-sm text-[var(--muted)]">{shortRepo(project.repoUrl)}</p>
            </div>
            <p className="text-sm text-[var(--muted)]">{formatWhen(project.createdAt)}</p>
          </Link>
        </li>
      ))}
    </ul>
  );
}
