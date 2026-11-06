import Link from "next/link";

import { ProjectList } from "@/components/project-list";
import { ApiError, listProjects } from "@/lib/api/client";
import type { Project } from "@/lib/api/types";

export default async function HomePage() {
  let projects: Project[] | null = null;
  let initialError: string | null = null;
  try {
    projects = await listProjects();
  } catch (err) {
    initialError = err instanceof ApiError ? err.message : "Could not load projects.";
  }

  return (
    <div className="space-y-8">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-3xl font-semibold tracking-tight text-[var(--text)]">Projects</h1>
          <p className="mt-2 max-w-2xl text-[var(--muted)]">
            Create a project from a public GitHub URL, deploy it, and watch the worker move through the
            real status list.
          </p>
        </div>
        <Link href="/projects/new" className="btn-primary">
          New project
        </Link>
      </div>
      <ProjectList initial={projects} initialError={initialError} />
    </div>
  );
}
