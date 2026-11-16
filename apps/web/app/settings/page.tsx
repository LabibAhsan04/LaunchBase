import Link from "next/link";

export default function SettingsPage() {
  return (
    <div className="space-y-4">
      <h1 className="text-3xl font-semibold tracking-tight text-[var(--text)]">Settings</h1>
      <p className="max-w-xl text-[var(--muted)]">
        Sign in with GitHub and install the LaunchBase GitHub App on the repositories you want to
        deploy. Sessions use an HTTP-only cookie. Installation tokens stay on the API and worker.
      </p>
      <div className="flex flex-wrap gap-4 text-sm">
        <Link href="/login" className="text-[var(--accent)]">
          Sign in
        </Link>
        <Link href="/install-github" className="text-[var(--accent)]">
          Install GitHub App
        </Link>
      </div>
    </div>
  );
}
