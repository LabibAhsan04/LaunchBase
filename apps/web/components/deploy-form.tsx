"use client";

import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";

import { ApiError, createDeployment } from "@/lib/api/client";
import { hasFieldErrors, validateDeployForm } from "@/lib/api/validate";

export function DeployForm({
  projectId,
  defaultBranch,
}: {
  projectId: string;
  defaultBranch: string;
}) {
  const router = useRouter();
  const [branch, setBranch] = useState(defaultBranch);
  const [fieldErrors, setFieldErrors] = useState<ReturnType<typeof validateDeployForm>>({});
  const [serverError, setServerError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const next = validateDeployForm({ branch });
    setFieldErrors(next);
    setServerError(null);
    if (hasFieldErrors(next)) {
      return;
    }
    setPending(true);
    try {
      const trimmed = branch.trim();
      const deployment = await createDeployment(projectId, {
        branch: trimmed && trimmed !== defaultBranch ? trimmed : undefined,
      });
      router.push(`/deployments/${deployment.id}`);
      router.refresh();
    } catch (err) {
      setServerError(err instanceof ApiError ? err.message : "Could not queue the deployment.");
    } finally {
      setPending(false);
    }
  }

  return (
    <form onSubmit={onSubmit} className="flex flex-col gap-4 sm:flex-row sm:items-end" noValidate>
      <div className="min-w-0 flex-1">
        <label htmlFor="deploy-branch" className="block text-sm font-medium text-[var(--text)]">
          Branch
        </label>
        <input
          id="deploy-branch"
          name="branch"
          value={branch}
          onChange={(e) => setBranch(e.target.value)}
          className="field mt-2"
          aria-invalid={fieldErrors.branch ? true : undefined}
          aria-describedby={fieldErrors.branch ? "deploy-branch-error" : undefined}
        />
        {fieldErrors.branch ? (
          <p id="deploy-branch-error" className="mt-2 text-sm text-[var(--bad)]">
            {fieldErrors.branch}
          </p>
        ) : null}
      </div>
      <button type="submit" className="btn-primary shrink-0" disabled={pending}>
        {pending ? "Queuing…" : "Deploy"}
      </button>
      {serverError ? <p className="basis-full text-sm text-[var(--bad)]">{serverError}</p> : null}
    </form>
  );
}
