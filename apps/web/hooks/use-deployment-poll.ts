"use client";

import { useEffect, useState } from "react";

import { ApiError, getDeployment } from "@/lib/api/client";
import type { Deployment } from "@/lib/api/types";
import { isActiveStatus } from "@/lib/format";

const INTERVAL_MS = 2000;

export function useDeploymentPoll(initial: Deployment) {
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
    }, INTERVAL_MS);
    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
  }, [deployment.id, deployment.status]);

  return { deployment, error, setDeployment };
}
