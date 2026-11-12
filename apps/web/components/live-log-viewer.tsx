"use client";

import { useEffect, useRef, useState } from "react";

import type { DeploymentLog } from "@/lib/api/types";
import type { StreamState } from "@/hooks/use-deployment-stream";

const STATE_LABEL: Record<StreamState, string> = {
  connecting: "Connecting",
  live: "Live",
  reconnecting: "Reconnecting",
  done: "Complete",
  error: "Error",
};

export function LiveLogViewer({
  lines,
  state,
  error,
}: {
  lines: DeploymentLog[];
  state: StreamState;
  error: string | null;
}) {
  const scroller = useRef<HTMLDivElement>(null);
  const [autoScroll, setAutoScroll] = useState(true);

  useEffect(() => {
    if (!autoScroll || !scroller.current) {
      return;
    }
    scroller.current.scrollTop = scroller.current.scrollHeight;
  }, [lines, autoScroll]);

  return (
    <section className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="text-lg font-medium text-[var(--text)]">Build logs</h2>
          <p className="mt-1 text-sm text-[var(--muted)]">
            {STATE_LABEL[state]}
            {error ? ` · ${error}` : ""}
          </p>
        </div>
        <label className="flex items-center gap-2 text-sm text-[var(--muted)]">
          <input
            type="checkbox"
            checked={autoScroll}
            onChange={(e) => setAutoScroll(e.target.checked)}
          />
          Auto-scroll
        </label>
      </div>
      <div
        ref={scroller}
        className="h-80 overflow-auto rounded-xl border border-[var(--line)] bg-[#070d18] p-4 font-mono text-xs leading-5 text-[#d7e3f4]"
        role="log"
        aria-live="polite"
      >
        {lines.length === 0 ? (
          <p className="text-[var(--muted)]">Waiting for worker output…</p>
        ) : (
          lines.map((line) => (
            <div key={line.seq} className="whitespace-pre-wrap break-all">
              <span className="text-[var(--muted)]">{String(line.seq).padStart(4, " ")} </span>
              <span className={tone(line.level)}>{line.message}</span>
            </div>
          ))
        )}
      </div>
    </section>
  );
}

function tone(level: string): string {
  if (level === "error") {
    return "text-[var(--bad)]";
  }
  if (level === "done") {
    return "text-[var(--ok)]";
  }
  return "";
}
