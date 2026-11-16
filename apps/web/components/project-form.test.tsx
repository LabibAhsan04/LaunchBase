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
  listRepos: vi.fn(),
  listBranches: vi.fn(),
}));

import { createProject, listBranches, listRepos } from "@/lib/api/client";
import { ProjectForm } from "./project-form";

describe("ProjectForm happy path", () => {
  afterEach(() => {
    cleanup();
  });

  beforeEach(() => {
    push.mockReset();
    refresh.mockReset();
    vi.mocked(createProject).mockReset();
    vi.mocked(listRepos).mockReset();
    vi.mocked(listBranches).mockReset();
    vi.mocked(listRepos).mockResolvedValue([]);
    vi.mocked(listBranches).mockResolvedValue([]);
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
    await waitFor(() => expect(listRepos).toHaveBeenCalled());
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
    await waitFor(() => expect(listRepos).toHaveBeenCalled());
    fireEvent.click(screen.getByRole("button", { name: "Create project" }));
    expect(await screen.findByText("Name is required.")).toBeTruthy();
    expect(createProject).not.toHaveBeenCalled();
  });

  it("uses repository and branch selectors when repos are installed", async () => {
    vi.mocked(listRepos).mockResolvedValue([
      {
        installationId: "inst",
        installationGhid: 1,
        githubRepoId: 9,
        owner: "acme",
        name: "web",
        fullName: "acme/web",
        private: false,
        defaultBranch: "main",
      },
    ]);
    vi.mocked(listBranches).mockResolvedValue(["main", "dev"]);
    vi.mocked(createProject).mockResolvedValue({
      id: "11111111-1111-4111-8111-111111111111",
      name: "Demo",
      repoUrl: "https://github.com/acme/web.git",
      branch: "dev",
      createdAt: "2026-11-06T00:00:00Z",
      updatedAt: "2026-11-06T00:00:00Z",
    });

    render(<ProjectForm />);
    await screen.findByLabelText("Repository");
    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Demo" } });
    fireEvent.change(screen.getByLabelText("Repository"), { target: { value: "acme/web" } });
    await waitFor(() => expect(listBranches).toHaveBeenCalledWith("acme/web"));
    await screen.findByLabelText("Branch");
    fireEvent.change(screen.getByLabelText("Branch"), { target: { value: "dev" } });
    fireEvent.click(screen.getByRole("button", { name: "Create project" }));

    await waitFor(() => {
      expect(createProject).toHaveBeenCalledWith({
        name: "Demo",
        fullName: "acme/web",
        branch: "dev",
      });
    });
  });
});
