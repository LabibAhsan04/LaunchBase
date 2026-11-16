import { apiOrigin, loginURL } from "@/lib/api/client";

export default function LoginPage() {
  return (
    <div className="mx-auto max-w-lg space-y-6">
      <h1 className="text-3xl font-semibold tracking-tight text-[var(--text)]">Sign in</h1>
      <p className="text-[var(--muted)]">
        LaunchBase uses a GitHub App for identity and repository access. Personal access tokens are
        not accepted.
      </p>
      <a href={loginURL()} className="btn-primary inline-flex">
        Continue with GitHub
      </a>
      <p className="text-sm text-[var(--muted)]">
        Local development can keep <code className="text-[var(--text)]">AUTH_MODE=dev</code> and use
        the seeded local user without signing in. API origin: {apiOrigin()}.
      </p>
    </div>
  );
}
