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
  error: z.string().optional(),
  createdAt: z.string(),
  updatedAt: z.string(),
  finishedAt: z.string().optional(),
});

export const projectSchema = z.object({
  id: z.string(),
  name: z.string(),
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
