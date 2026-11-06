"use client";

import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";

import { ApiError, createProject } from "@/lib/api/client";
import { hasFieldErrors, validateProjectForm } from "@/lib/api/validate";

export function ProjectForm() {
  const router = useRouter();
  const [name, setName] = useState("");
  const [repoUrl, setRepoUrl] = useState("");
  const [branch, setBranch] = useState("main");
  const [fieldErrors, setFieldErrors] = useState<ReturnType<typeof validateProjectForm>>({});
  const [serverError, setServerError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const next = validateProjectForm({ name, repoUrl, branch });
    setFieldErrors(next);
    setServerError(null);
    if (hasFieldErrors(next)) {
      return;
    }
    setPending(true);
    try {
      const project = await createProject({
        name: name.trim(),
        repoUrl: repoUrl.trim(),
        branch: branch.trim() || undefined,
      });
      router.push(`/projects/${project.id}`);
      router.refresh();
    } catch (err) {
      setServerError(err instanceof ApiError ? err.message : "Could not create the project.");
    } finally {
      setPending(false);
    }
  }

  return (
    <form onSubmit={onSubmit} className="max-w-xl space-y-6" noValidate>
      <Field
        id="name"
        label="Name"
        value={name}
        onChange={setName}
        error={fieldErrors.name}
        autoComplete="off"
        placeholder="Demo"
      />
      <Field
        id="repoUrl"
        label="Repository URL"
        value={repoUrl}
        onChange={setRepoUrl}
        error={fieldErrors.repoUrl}
        autoComplete="off"
        placeholder="https://github.com/owner/repo"
        hint="Public https://github.com/owner/repo only. SSH and local paths are rejected."
      />
      <Field
        id="branch"
        label="Branch"
        value={branch}
        onChange={setBranch}
        error={fieldErrors.branch}
        autoComplete="off"
        placeholder="main"
      />
      {serverError ? <p className="text-sm text-[var(--bad)]">{serverError}</p> : null}
      <button type="submit" className="btn-primary" disabled={pending}>
        {pending ? "Creating…" : "Create project"}
      </button>
    </form>
  );
}

function Field(props: {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  error?: string;
  hint?: string;
  placeholder?: string;
  autoComplete?: string;
}) {
  return (
    <div>
      <label htmlFor={props.id} className="block text-sm font-medium text-[var(--text)]">
        {props.label}
      </label>
      <input
        id={props.id}
        name={props.id}
        value={props.value}
        onChange={(e) => props.onChange(e.target.value)}
        placeholder={props.placeholder}
        autoComplete={props.autoComplete}
        aria-invalid={props.error ? true : undefined}
        aria-describedby={props.error ? `${props.id}-error` : props.hint ? `${props.id}-hint` : undefined}
        className="field mt-2"
      />
      {props.hint && !props.error ? (
        <p id={`${props.id}-hint`} className="mt-2 text-sm text-[var(--muted)]">
          {props.hint}
        </p>
      ) : null}
      {props.error ? (
        <p id={`${props.id}-error`} className="mt-2 text-sm text-[var(--bad)]">
          {props.error}
        </p>
      ) : null}
    </div>
  );
}
