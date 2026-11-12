import {
  apiErrorSchema,
  deploymentLogListSchema,
  deploymentSchema,
  projectListSchema,
  projectSchema,
} from "./schema";
import type {
  CreateDeploymentInput,
  CreateProjectInput,
  Deployment,
  DeploymentLog,
  Project,
} from "./types";

export class ApiError extends Error {
  readonly code: string;
  readonly status: number;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

function baseURL(): string {
  if (typeof window === "undefined") {
    return process.env.API_URL || process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
  }
  return process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
}

async function request<T>(
  path: string,
  init: RequestInit | undefined,
  parse: (data: unknown) => T,
): Promise<T> {
  let res: Response;
  try {
    res = await fetch(`${baseURL()}${path}`, {
      ...init,
      headers: {
        Accept: "application/json",
        ...(init?.body ? { "Content-Type": "application/json" } : {}),
        ...init?.headers,
      },
      cache: "no-store",
    });
  } catch {
    throw new ApiError(0, "network", "Could not reach the API. Is make api running?");
  }

  const text = await res.text();
  let body: unknown = null;
  if (text) {
    try {
      body = JSON.parse(text) as unknown;
    } catch {
      throw new ApiError(res.status, "invalid_response", "The API returned something that was not JSON.");
    }
  }

  if (!res.ok) {
    const err = apiErrorSchema.safeParse(body);
    if (err.success) {
      throw new ApiError(res.status, err.data.error.code, err.data.error.message);
    }
    throw new ApiError(res.status, "http_error", `Request failed with status ${res.status}`);
  }

  try {
    return parse(body);
  } catch {
    throw new ApiError(res.status, "invalid_response", "The API response did not match the OpenAPI shape.");
  }
}

export function listProjects(): Promise<Project[]> {
  return request("/api/v1/projects", undefined, (data) => projectListSchema.parse(data).projects);
}

export function getProject(projectId: string): Promise<Project> {
  return request(`/api/v1/projects/${projectId}`, undefined, (data) => projectSchema.parse(data));
}

export function createProject(input: CreateProjectInput): Promise<Project> {
  return request(
    "/api/v1/projects",
    { method: "POST", body: JSON.stringify(input) },
    (data) => projectSchema.parse(data),
  );
}

export function createDeployment(
  projectId: string,
  input: CreateDeploymentInput = {},
): Promise<Deployment> {
  const body =
    input.branch && input.branch.trim() !== ""
      ? JSON.stringify({ branch: input.branch.trim() })
      : undefined;
  return request(
    `/api/v1/projects/${projectId}/deployments`,
    { method: "POST", body },
    (data) => deploymentSchema.parse(data),
  );
}

export function getDeployment(deploymentId: string): Promise<Deployment> {
  return request(`/api/v1/deployments/${deploymentId}`, undefined, (data) =>
    deploymentSchema.parse(data),
  );
}

export function stopDeployment(deploymentId: string): Promise<Deployment> {
  return request(
    `/api/v1/deployments/${deploymentId}/stop`,
    { method: "POST" },
    (data) => deploymentSchema.parse(data),
  );
}

export function listLogs(deploymentId: string, after = 0): Promise<DeploymentLog[]> {
  return request(
    `/api/v1/deployments/${deploymentId}/logs?after=${after}`,
    undefined,
    (data) => deploymentLogListSchema.parse(data).logs,
  );
}

export function logsStreamURL(deploymentId: string, after = 0): string {
  const root = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
  return `${root}/api/v1/deployments/${deploymentId}/logs/stream?after=${after}`;
}
