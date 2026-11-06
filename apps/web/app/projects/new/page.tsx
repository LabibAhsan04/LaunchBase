import Link from "next/link";

import { ProjectForm } from "@/components/project-form";

export default function NewProjectPage() {
  return (
    <div className="space-y-8">
      <div>
        <p className="text-sm text-[var(--muted)]">
          <Link href="/" className="hover:text-[var(--text)]">
            Projects
          </Link>
        </p>
        <h1 className="mt-2 text-3xl font-semibold tracking-tight text-[var(--text)]">New project</h1>
        <p className="mt-2 max-w-xl text-[var(--muted)]">
          There is no GitHub login yet. Paste a public repository URL. The local user owns every project.
        </p>
      </div>
      <ProjectForm />
    </div>
  );
}
