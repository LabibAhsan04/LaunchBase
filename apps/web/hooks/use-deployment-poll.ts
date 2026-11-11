"use client";

import { useEffect, useState } from "react";

import { ApiError, getDeployment } from "@/lib/api/client";
import type { Deployment } from "@/lib/api/types";
import { isActiveStatus } from "@/lib/format";

export function useDeploymentPoll(initial: Deployment, intervalMs = 2000) {
  const [deployment, setDeployment] = useState(initial);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!isActiveStatus(deployment.status)) {
      return;
    }
    let cancelled = false;
    const id = window.setInterval(() => {
      void (async () => {
        try {
          const next = await getDeployment(deployment.id);
          if (!cancelled) {
            setDeployment(next);
            setError(null);
          }
        } catch (err) {
          if (!cancelled) {
            setError(err instanceof ApiError ? err.message : "Could not refresh status.");
          }
        }
      })();
    }, intervalMs);
    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
  }, [deployment.id, deployment.status, intervalMs]);

  return { deployment, error, setDeployment };
}
