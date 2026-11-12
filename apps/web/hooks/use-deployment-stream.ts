"use client";

import { useEffect, useRef, useState } from "react";

import { getDeployment, listLogs, logsStreamURL } from "@/lib/api/client";
import type { Deployment, DeploymentLog } from "@/lib/api/types";
import { isActiveStatus } from "@/lib/format";

export type StreamState = "connecting" | "live" | "reconnecting" | "done" | "error";

export function useDeploymentStream(
  deploymentId: string,
  onStatus?: (deployment: Deployment) => void,
) {
  const [lines, setLines] = useState<DeploymentLog[]>([]);
  const [state, setState] = useState<StreamState>("connecting");
  const [error, setError] = useState<string | null>(null);
  const lastSeq = useRef(0);
  const seen = useRef(new Set<number>());
  const onStatusRef = useRef(onStatus);

  useEffect(() => {
    onStatusRef.current = onStatus;
  }, [onStatus]);

  useEffect(() => {
    let cancelled = false;
    let source: EventSource | null = null;
    let reconnectTimer: number | undefined;

    const append = (entry: DeploymentLog) => {
      if (seen.current.has(entry.seq)) {
        return;
      }
      seen.current.add(entry.seq);
      lastSeq.current = Math.max(lastSeq.current, entry.seq);
      setLines((prev) => [...prev, entry]);
    };

    const finish = async () => {
      setState("done");
      try {
        const dep = await getDeployment(deploymentId);
        if (!cancelled) {
          onStatusRef.current?.(dep);
        }
      } catch {
        // status poll is the fallback
      }
    };

    const open = async () => {
      try {
        const history = await listLogs(deploymentId, lastSeq.current);
        if (cancelled) {
          return;
        }
        for (const entry of history) {
          append(entry);
        }
        if (history.some((e) => e.level === "done")) {
          await finish();
          return;
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Could not load logs.");
          setState("error");
        }
        return;
      }

      setState(lastSeq.current > 0 ? "reconnecting" : "connecting");
      source = new EventSource(logsStreamURL(deploymentId, lastSeq.current));

      source.addEventListener("log", (event) => {
        try {
          const entry = JSON.parse(event.data) as DeploymentLog;
          append(entry);
          setState("live");
          setError(null);
          if (entry.level === "done") {
            source?.close();
            void finish();
          }
        } catch {
          setError("Bad log event from the API.");
        }
      });

      source.addEventListener("done", () => {
        source?.close();
        void finish();
      });

      source.addEventListener("heartbeat", () => {
        setState((prev) => (prev === "done" ? prev : "live"));
      });

      source.onerror = () => {
        source?.close();
        source = null;
        if (cancelled) {
          return;
        }
        void getDeployment(deploymentId)
          .then((dep) => {
            if (!isActiveStatus(dep.status)) {
              onStatusRef.current?.(dep);
              setState("done");
              return;
            }
            setState("reconnecting");
            reconnectTimer = window.setTimeout(() => {
              void open();
            }, 1500);
          })
          .catch(() => {
            setState("reconnecting");
            reconnectTimer = window.setTimeout(() => {
              void open();
            }, 1500);
          });
      };
    };

    void open();

    return () => {
      cancelled = true;
      source?.close();
      if (reconnectTimer) {
        window.clearTimeout(reconnectTimer);
      }
    };
  }, [deploymentId]);

  return { lines, state, error };
}
