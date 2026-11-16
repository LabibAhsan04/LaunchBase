import { installURL } from "@/lib/api/client";

export default function InstallGitHubPage() {
  return (
    <div className="mx-auto max-w-xl space-y-6">
      <h1 className="text-3xl font-semibold tracking-tight text-[var(--text)]">
        Install the GitHub App
      </h1>
      <p className="text-[var(--muted)]">
        Choose the repositories LaunchBase may read. The app requests the minimum permissions needed
        to list repositories, clone source, and receive installation webhooks.
      </p>
      <ul className="list-disc space-y-2 pl-5 text-[var(--muted)]">
        <li>Repository metadata: read</li>
        <li>Contents: read (clone only)</li>
        <li>Webhooks: receive installation and repository change events</li>
      </ul>
      <p className="text-[var(--muted)]">
        No organization-wide write access is requested. Tokens stay on the server and never reach the
        browser.
      </p>
      <a href={installURL()} className="btn-primary inline-flex">
        Install on GitHub
      </a>
    </div>
  );
}
