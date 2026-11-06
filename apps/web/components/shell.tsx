import Link from "next/link";
import type { ReactNode } from "react";

export function Shell({ children }: { children: ReactNode }) {
  return (
    <div className="min-h-full">
      <header className="border-b border-[var(--line)]">
        <div className="mx-auto flex max-w-5xl items-center justify-between gap-6 px-6 py-5">
          <Link href="/" className="text-lg font-semibold tracking-tight text-[var(--text)]">
            LaunchBase
          </Link>
          <nav className="flex items-center gap-5 text-sm text-[var(--muted)]">
            <Link className="hover:text-[var(--text)]" href="/">
              Projects
            </Link>
            <Link className="hover:text-[var(--text)]" href="/projects/new">
              New project
            </Link>
            <Link className="hover:text-[var(--text)]" href="/settings">
              Settings
            </Link>
          </nav>
        </div>
      </header>
      <main className="mx-auto max-w-5xl px-6 py-10">{children}</main>
    </div>
  );
}
