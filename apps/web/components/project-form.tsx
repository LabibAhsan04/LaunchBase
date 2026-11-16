"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState, type FormEvent } from "react";

import { ApiError, createProject, listBranches, listRepos } from "@/lib/api/client";
import type { RepoAccess } from "@/lib/api/types";
import { hasFieldErrors, validateProjectForm } from "@/lib/api/validate";

export function ProjectForm() {
  const router = useRouter();
  const [name, setName] = useState("");
  const [repos, setRepos] = useState<RepoAccess[]>([]);
  const [reposLoaded, setReposLoaded] = useState(false);
  const [fullName, setFullName] = useState("");
  const [repoUrl, setRepoUrl] = useState("");
  const [branch, setBranch] = useState("main");
  const [branches, setBranches] = useState<string[]>([]);
  const [fieldErrors, setFieldErrors] = useState<ReturnType<typeof validateProjectForm>>({});
  const [serverError, setServerError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  useEffect(() => {
    let cancelled = false;
    listRepos()
      .then((items) => {
        if (!cancelled) {
          setRepos(items);
          setReposLoaded(true);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setRepos([]);
          setReposLoaded(true);
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (!fullName) {
      return;
    }
    let cancelled = false;
    listBranches(fullName)
      .then((names) => {
        if (cancelled) {
          return;
        }
        setBranches(names);
        if (names.length > 0 && !names.includes(branch)) {
          setBranch(names[0]);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setBranches([]);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [fullName]); // eslint-disable-line react-hooks/exhaustive-deps -- reload branches when repo changes

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const next = validateProjectForm({
      name,
      repoUrl,
      fullName,
      branch,
      requireSelector: repos.length > 0,
    });
    setFieldErrors(next);
    setServerError(null);
    if (hasFieldErrors(next)) {
      return;
    }
    setPending(true);
    try {
      const project = await createProject({
        name: name.trim(),
        fullName: fullName.trim() || undefined,
        repoUrl: fullName.trim() ? undefined : repoUrl.trim() || undefined,
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

  function onSelectRepo(value: string) {
    setFullName(value);
    setBranches([]);
    const selected = repos.find((r) => r.fullName === value);
    if (selected) {
      setBranch(selected.defaultBranch || "main");
      setRepoUrl("");
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

      {reposLoaded && repos.length > 0 ? (
        <div>
          <label htmlFor="fullName" className="block text-sm font-medium text-[var(--text)]">
            Repository
          </label>
          <select
            id="fullName"
            name="fullName"
            value={fullName}
            onChange={(e) => onSelectRepo(e.target.value)}
            className="field mt-2"
            aria-invalid={fieldErrors.fullName ? true : undefined}
          >
            <option value="">Select a repository</option>
            {repos.map((repo) => (
              <option key={repo.githubRepoId} value={repo.fullName}>
                {repo.fullName}
                {repo.private ? " (private)" : ""}
              </option>
            ))}
          </select>
          {fieldErrors.fullName ? (
            <p className="mt-2 text-sm text-[var(--bad)]">{fieldErrors.fullName}</p>
          ) : (
            <p className="mt-2 text-sm text-[var(--muted)]">
              Only repositories from your GitHub App installation are listed.{" "}
              <Link href="/install-github" className="text-[var(--accent)]">
                Manage installation
              </Link>
            </p>
          )}
        </div>
      ) : (
        <Field
          id="repoUrl"
          label="Repository URL"
          value={repoUrl}
          onChange={setRepoUrl}
          error={fieldErrors.repoUrl}
          autoComplete="off"
          placeholder="https://github.com/owner/repo"
          hint={
            reposLoaded
              ? "No installed repositories yet. Install the GitHub App, or paste a public https://github.com/owner/repo URL in AUTH_MODE=dev."
              : "Loading repositories…"
          }
        />
      )}

      {branches.length > 0 ? (
        <div>
          <label htmlFor="branch" className="block text-sm font-medium text-[var(--text)]">
            Branch
          </label>
          <select
            id="branch"
            name="branch"
            value={branch}
            onChange={(e) => setBranch(e.target.value)}
            className="field mt-2"
          >
            {branches.map((b) => (
              <option key={b} value={b}>
                {b}
              </option>
            ))}
          </select>
        </div>
      ) : (
        <Field
          id="branch"
          label="Branch"
          value={branch}
          onChange={setBranch}
          error={fieldErrors.branch}
          autoComplete="off"
          placeholder="main"
        />
      )}

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
