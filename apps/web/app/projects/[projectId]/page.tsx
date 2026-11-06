import { ProjectDetail } from "@/components/project-detail";
import { StateBlock } from "@/components/state-block";
import { ApiError, getProject } from "@/lib/api/client";

type Props = {
  params: Promise<{ projectId: string }>;
};

export default async function ProjectPage({ params }: Props) {
  const { projectId } = await params;
  let project = null;
  let message: string | null = null;
  try {
    project = await getProject(projectId);
  } catch (err) {
    message = err instanceof ApiError ? err.message : "Could not load the project.";
  }

  if (!project) {
    return <StateBlock title="Project unavailable" body={message || "Not found"} />;
  }
  return <ProjectDetail key={project.updatedAt} initial={project} />;
}
