// Soft client checks. The Go API still rejects anything that is wrong.

export type FieldErrors = {
  name?: string;
  repoUrl?: string;
  branch?: string;
};

export function validateProjectForm(input: {
  name: string;
  repoUrl: string;
  branch: string;
}): FieldErrors {
  const errors: FieldErrors = {};
  const name = input.name.trim();
  if (!name) {
    errors.name = "Name is required.";
  } else if (name.length > 40) {
    errors.name = "Name must be at most 40 characters.";
  } else if (!/^[A-Za-z0-9 _-]+$/.test(name)) {
    errors.name = "Use letters, numbers, spaces, dashes, or underscores.";
  }

  const repoUrl = input.repoUrl.trim();
  if (!repoUrl) {
    errors.repoUrl = "Repository URL is required.";
  } else if (!/^https:\/\/github\.com\/[^/\s]+\/[^/\s]+(?:\.git)?\/?$/.test(repoUrl)) {
    errors.repoUrl = "Use https://github.com/owner/repo.";
  }

  const branch = input.branch.trim();
  if (branch && (branch.startsWith("-") || branch.includes("..") || /[^A-Za-z0-9._/-]/.test(branch))) {
    errors.branch = "Branch can use letters, numbers, ., _, /, and -.";
  }

  return errors;
}

export function validateDeployForm(input: { branch: string }): FieldErrors {
  const errors: FieldErrors = {};
  const branch = input.branch.trim();
  if (branch && (branch.startsWith("-") || branch.includes("..") || /[^A-Za-z0-9._/-]/.test(branch))) {
    errors.branch = "Branch can use letters, numbers, ., _, /, and -.";
  }
  return errors;
}

export function hasFieldErrors(errors: FieldErrors): boolean {
  return Boolean(errors.name || errors.repoUrl || errors.branch);
}
