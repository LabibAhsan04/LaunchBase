import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const push = vi.fn();
const refresh = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push, refresh }),
}));

vi.mock("@/lib/api/client", () => ({
  ApiError: class ApiError extends Error {
    code: string;
    status: number;
    constructor(status: number, code: string, message: string) {
      super(message);
      this.status = status;
      this.code = code;
    }
  },
  createProject: vi.fn(),
}));

import { createProject } from "@/lib/api/client";
import { ProjectForm } from "./project-form";

describe("ProjectForm happy path", () => {
  afterEach(() => {
    cleanup();
  });

  beforeEach(() => {
    push.mockReset();
    refresh.mockReset();
    vi.mocked(createProject).mockReset();
  });

  it("creates a project and routes to it", async () => {
    vi.mocked(createProject).mockResolvedValue({
      id: "11111111-1111-4111-8111-111111111111",
      name: "Demo",
      repoUrl: "https://github.com/acme/web.git",
      branch: "main",
      createdAt: "2026-11-06T00:00:00Z",
      updatedAt: "2026-11-06T00:00:00Z",
    });

    render(<ProjectForm />);
    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Demo" } });
    fireEvent.change(screen.getByLabelText("Repository URL"), {
      target: { value: "https://github.com/acme/web" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Create project" }));

    await waitFor(() => {
      expect(createProject).toHaveBeenCalledWith({
        name: "Demo",
        repoUrl: "https://github.com/acme/web",
        branch: "main",
      });
      expect(push).toHaveBeenCalledWith("/projects/11111111-1111-4111-8111-111111111111");
    });
  });

  it("shows client validation before calling the api", async () => {
    render(<ProjectForm />);
    fireEvent.click(screen.getByRole("button", { name: "Create project" }));
    expect(await screen.findByText("Name is required.")).toBeTruthy();
    expect(createProject).not.toHaveBeenCalled();
  });
});
