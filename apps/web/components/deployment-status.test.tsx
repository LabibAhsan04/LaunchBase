import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { DeploymentStatusBadge } from "./deployment-status";

describe("DeploymentStatusBadge", () => {
  it("shows the backend status labels", () => {
    render(<DeploymentStatusBadge status="HEALTH_CHECKING" />);
    expect(screen.getByText("Health check").getAttribute("data-status")).toBe("HEALTH_CHECKING");
  });

  it("marks a failure clearly", () => {
    render(<DeploymentStatusBadge status="FAILED" />);
    expect(screen.getByText("Failed").getAttribute("data-status")).toBe("FAILED");
  });
});
