// Shapes match docs/openapi.yaml. The API is the source of truth.

export const DEPLOYMENT_STATUSES = [
  "QUEUED",
  "CLONING",
  "DETECTING",
  "BUILDING",
  "STARTING",
  "HEALTH_CHECKING",
  "READY",
  "FAILED",
  "STOPPED",
  "SUPERSEDED",
] as const;

export type DeploymentStatus = (typeof DEPLOYMENT_STATUSES)[number];

export type Project = {
  id: string;
  name: string;
  slug: string;
  repoUrl: string;
  branch: string;
  createdAt: string;
  updatedAt: string;
  deployments?: Deployment[];
};

export type Deployment = {
  id: string;
  projectId: string;
  status: DeploymentStatus;
  repoUrl: string;
  branch: string;
  image?: string;
  container?: string;
  url?: string;
  hostname?: string;
  error?: string;
  createdAt: string;
  updatedAt: string;
  finishedAt?: string;
};

export type ApiErrorBody = {
  error: {
    code: string;
    message: string;
  };
};

export type CreateProjectInput = {
  name: string;
  repoUrl?: string;
  fullName?: string;
  branch?: string;
};

export type AuthMe = {
  id: string;
  login: string;
  displayName: string;
  avatarUrl: string;
  authMode: string;
};

export type RepoAccess = {
  installationId: string;
  installationGhid: number;
  githubRepoId: number;
  owner: string;
  name: string;
  fullName: string;
  private: boolean;
  defaultBranch: string;
};

export type InstallationList = {
  installations: Array<{
    id: string;
    installationId: number;
    accountLogin: string;
    accountType: string;
    suspended: boolean;
  }>;
  installUrl: string;
};

export type CreateDeploymentInput = {
  branch?: string;
};

export type DeploymentLog = {
  seq: number;
  deploymentId: string;
  ts: string;
  source: string;
  level: string;
  message: string;
};
