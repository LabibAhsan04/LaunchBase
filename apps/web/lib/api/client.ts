import {
  apiErrorSchema,
  authMeSchema,
  branchListSchema,
  deploymentLogListSchema,
  deploymentSchema,
  installationListSchema,
  projectEnvListSchema,
  projectEnvVarSchema,
  projectListSchema,
  projectSchema,
  repoListSchema,
} from "./schema";
import type {
  AuthMe,
  CreateDeploymentInput,
  CreateProjectInput,
  Deployment,
  DeploymentLog,
  InstallationList,
  Project,
  ProjectEnvVar,
  RepoAccess,
  UpsertEnvInput,
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

export function apiOrigin(): string {
  return baseURL();
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
      credentials: "include",
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

export function getMe(): Promise<AuthMe> {
  return request("/api/v1/auth/me", undefined, (data) => authMeSchema.parse(data));
}

export function logout(): Promise<{ ok: boolean }> {
  return request("/api/v1/auth/logout?json=1", { method: "POST" }, () => ({ ok: true }));
}

export function loginURL(): string {
  return `${baseURL()}/api/v1/auth/login`;
}

export function installURL(): string {
  return `${baseURL()}/api/v1/auth/install`;
}

export function listRepos(): Promise<RepoAccess[]> {
  return request("/api/v1/github/repos", undefined, (data) => repoListSchema.parse(data).repos);
}

export function listBranches(fullName: string): Promise<string[]> {
  const [owner, repo] = fullName.split("/");
  return request(
    `/api/v1/github/repos/${encodeURIComponent(owner)}/${encodeURIComponent(repo)}/branches`,
    undefined,
    (data) => branchListSchema.parse(data).branches,
  );
}

export function listInstallations(): Promise<InstallationList> {
  return request("/api/v1/github/installations", undefined, (data) =>
    installationListSchema.parse(data),
  );
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

export function redeployDeployment(deploymentId: string): Promise<Deployment> {
  return request(
    `/api/v1/deployments/${deploymentId}/redeploy`,
    { method: "POST" },
    (data) => deploymentSchema.parse(data),
  );
}

export function rollbackDeployment(deploymentId: string): Promise<Deployment> {
  return request(
    `/api/v1/deployments/${deploymentId}/rollback`,
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

export function listEnvVars(projectId: string): Promise<ProjectEnvVar[]> {
  return request(`/api/v1/projects/${projectId}/env`, undefined, (data) =>
    projectEnvListSchema.parse(data).vars,
  );
}

export function upsertEnvVar(projectId: string, input: UpsertEnvInput): Promise<ProjectEnvVar> {
  return request(
    `/api/v1/projects/${projectId}/env`,
    { method: "PUT", body: JSON.stringify(input) },
    (data) => projectEnvVarSchema.parse(data),
  );
}

export function deleteEnvVar(projectId: string, key: string, environment = "production"): Promise<void> {
  const q = environment ? `?environment=${encodeURIComponent(environment)}` : "";
  return request(
    `/api/v1/projects/${projectId}/env/${encodeURIComponent(key)}${q}`,
    { method: "DELETE" },
    () => undefined,
  );
}
