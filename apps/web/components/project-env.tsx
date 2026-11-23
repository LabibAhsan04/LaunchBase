"use client";

import { useEffect, useState, type FormEvent } from "react";

import { StateBlock } from "@/components/state-block";
import { ApiError, deleteEnvVar, listEnvVars, upsertEnvVar } from "@/lib/api/client";
import type { ProjectEnvVar } from "@/lib/api/types";

export function ProjectEnv({ projectId }: { projectId: string }) {
  const [vars, setVars] = useState<ProjectEnvVar[] | null>(null);
  const [key, setKey] = useState("");
  const [value, setValue] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let cancelled = false;
    const id = window.setTimeout(() => {
      void (async () => {
        try {
          const next = await listEnvVars(projectId);
          if (!cancelled) {
            setVars(next);
            setError(null);
          }
        } catch (err) {
          if (!cancelled) {
            setError(err instanceof ApiError ? err.message : "Could not load environment variables.");
            setVars([]);
          }
        }
      })();
    }, 0);
    return () => {
      cancelled = true;
      window.clearTimeout(id);
    };
  }, [projectId]);

  async function reload() {
    try {
      const next = await listEnvVars(projectId);
      setVars(next);
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not load environment variables.");
    }
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await upsertEnvVar(projectId, { key: key.trim(), value });
      setKey("");
      setValue("");
      await reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not save the variable.");
    } finally {
      setBusy(false);
    }
  }

  async function onDelete(name: string) {
    setBusy(true);
    setError(null);
    try {
      await deleteEnvVar(projectId, name);
      await reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not delete the variable.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="space-y-4">
      <div>
        <h2 className="text-lg font-medium text-[var(--text)]">Environment variables</h2>
        <p className="mt-1 text-sm text-[var(--muted)]">
          Encrypted at rest and injected into the next deploy. Values are never shown in full after save.
          Redeploy to apply changes to a running app.
        </p>
      </div>

      {error ? <p className="text-sm text-[var(--bad)]">{error}</p> : null}

      <form onSubmit={onSubmit} className="flex flex-col gap-3 sm:flex-row sm:items-end">
        <label className="block min-w-0 flex-1 text-sm">
          <span className="text-[var(--muted)]">Key</span>
          <input
            className="mt-1 w-full rounded-lg border border-[var(--line)] bg-[var(--panel)] px-3 py-2 font-mono text-[var(--text)]"
            name="key"
            autoComplete="off"
            spellCheck={false}
            placeholder="FINNHUB_API_KEY"
            value={key}
            onChange={(e) => setKey(e.target.value)}
            required
          />
        </label>
        <label className="block min-w-0 flex-[1.4] text-sm">
          <span className="text-[var(--muted)]">Value</span>
          <input
            className="mt-1 w-full rounded-lg border border-[var(--line)] bg-[var(--panel)] px-3 py-2 font-mono text-[var(--text)]"
            name="value"
            type="password"
            autoComplete="off"
            placeholder="secret"
            value={value}
            onChange={(e) => setValue(e.target.value)}
            required
          />
        </label>
        <button
          type="submit"
          disabled={busy}
          className="rounded-lg bg-[var(--accent)] px-4 py-2 text-sm font-medium text-[var(--accent-fg)] disabled:opacity-60"
        >
          {busy ? "Saving…" : "Save"}
        </button>
      </form>

      {vars === null ? (
        <p className="text-sm text-[var(--muted)]">Loading…</p>
      ) : vars.length === 0 ? (
        <StateBlock
          title="No variables yet"
          body="Add keys your app reads at runtime (API tokens, public config). LaunchBase platform secrets cannot be set here."
        />
      ) : (
        <ul className="divide-y divide-[var(--line)] rounded-xl border border-[var(--line)] bg-[var(--panel)]">
          {vars.map((v) => (
            <li key={v.key} className="flex items-center justify-between gap-4 px-5 py-3">
              <div className="min-w-0">
                <p className="font-mono text-sm text-[var(--text)]">{v.key}</p>
                <p className="mt-0.5 font-mono text-xs text-[var(--muted)]">{v.hint}</p>
              </div>
              <button
                type="button"
                disabled={busy}
                onClick={() => void onDelete(v.key)}
                className="shrink-0 text-sm text-[var(--bad)] hover:underline disabled:opacity-60"
              >
                Delete
              </button>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
