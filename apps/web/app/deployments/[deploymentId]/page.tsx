import { DeploymentDetail } from "@/components/deployment-detail";
import { StateBlock } from "@/components/state-block";
import { ApiError, getDeployment } from "@/lib/api/client";

type Props = {
  params: Promise<{ deploymentId: string }>;
};

export default async function DeploymentPage({ params }: Props) {
  const { deploymentId } = await params;
  let deployment = null;
  let message: string | null = null;
  try {
    deployment = await getDeployment(deploymentId);
  } catch (err) {
    message = err instanceof ApiError ? err.message : "Could not load the deployment.";
  }

  if (!deployment) {
    return <StateBlock title="Deployment unavailable" body={message || "Not found"} />;
  }
  return <DeploymentDetail key={deployment.updatedAt} initial={deployment} />;
}
