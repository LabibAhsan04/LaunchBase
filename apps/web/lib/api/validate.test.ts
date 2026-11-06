import { describe, expect, it } from "vitest";

import { hasFieldErrors, validateProjectForm } from "./validate";

describe("validateProjectForm", () => {
  it("accepts a normal github url", () => {
    const errors = validateProjectForm({
      name: "Demo",
      repoUrl: "https://github.com/acme/web",
      branch: "main",
    });
    expect(hasFieldErrors(errors)).toBe(false);
  });

  it("rejects ssh urls", () => {
    const errors = validateProjectForm({
      name: "Demo",
      repoUrl: "git@github.com:acme/web.git",
      branch: "main",
    });
    expect(errors.repoUrl).toMatch(/https:\/\/github.com/);
  });

  it("rejects an empty name", () => {
    const errors = validateProjectForm({
      name: "  ",
      repoUrl: "https://github.com/acme/web",
      branch: "main",
    });
    expect(errors.name).toBeTruthy();
  });
});
