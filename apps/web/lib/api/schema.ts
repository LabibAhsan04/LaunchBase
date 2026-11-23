import { z } from "zod";

import { DEPLOYMENT_STATUSES } from "./types";

const statusSchema = z.enum(DEPLOYMENT_STATUSES);

export const deploymentSchema = z.object({
  id: z.string(),
  projectId: z.string(),
  status: statusSchema,
  repoUrl: z.string(),
  branch: z.string(),
  image: z.string().optional(),
  container: z.string().optional(),
  url: z.string().optional(),
  hostname: z.string().optional(),
  error: z.string().optional(),
  createdAt: z.string(),
  updatedAt: z.string(),
  finishedAt: z.string().optional(),
});

export const projectSchema = z.object({
  id: z.string(),
  name: z.string(),
  slug: z.string(),
  repoUrl: z.string(),
  branch: z.string(),
  createdAt: z.string(),
  updatedAt: z.string(),
  deployments: z.array(deploymentSchema).optional(),
});

export const projectListSchema = z.object({
  projects: z.array(projectSchema),
});

export const apiErrorSchema = z.object({
  error: z.object({
    code: z.string(),
    message: z.string(),
  }),
});

export const deploymentLogSchema = z.object({
  seq: z.number(),
  deploymentId: z.string(),
  ts: z.string(),
  source: z.string(),
  level: z.string(),
  message: z.string(),
});

export const deploymentLogListSchema = z.object({
  logs: z.array(deploymentLogSchema),
});

export const authMeSchema = z.object({
  id: z.string(),
  login: z.string(),
  displayName: z.string(),
  avatarUrl: z.string(),
  authMode: z.string(),
});

export const repoAccessSchema = z.object({
  installationId: z.string(),
  installationGhid: z.number(),
  githubRepoId: z.number(),
  owner: z.string(),
  name: z.string(),
  fullName: z.string(),
  private: z.boolean(),
  defaultBranch: z.string(),
});

export const repoListSchema = z.object({
  repos: z.array(repoAccessSchema),
});

export const branchListSchema = z.object({
  branches: z.array(z.string()),
});

export const installationListSchema = z.object({
  installations: z.array(
    z.object({
      id: z.string(),
      installationId: z.number(),
      accountLogin: z.string(),
      accountType: z.string(),
      suspended: z.boolean(),
    }),
  ),
  installUrl: z.string(),
});

export const projectEnvVarSchema = z.object({
  key: z.string(),
  environment: z.string(),
  hint: z.string(),
  updatedAt: z.string(),
});

export const projectEnvListSchema = z.object({
  vars: z.array(projectEnvVarSchema),
});
