"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState, type ReactNode } from "react";

import { ApiError, getMe, logout } from "@/lib/api/client";
import type { AuthMe } from "@/lib/api/types";

export function Shell({ children }: { children: ReactNode }) {
  const router = useRouter();
  const [me, setMe] = useState<AuthMe | null>(null);

  useEffect(() => {
    let cancelled = false;
    getMe()
      .then((user) => {
        if (!cancelled) {
          setMe(user);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setMe(null);
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  async function onLogout() {
    try {
      await logout();
    } catch (err) {
      if (!(err instanceof ApiError)) {
        return;
      }
    }
    setMe(null);
    router.push("/login");
    router.refresh();
  }

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
            <Link className="hover:text-[var(--text)]" href="/install-github">
              Install App
            </Link>
            <Link className="hover:text-[var(--text)]" href="/settings">
              Settings
            </Link>
            {me ? (
              <button type="button" className="hover:text-[var(--text)]" onClick={onLogout}>
                Sign out ({me.login})
              </button>
            ) : (
              <Link className="hover:text-[var(--text)]" href="/login">
                Sign in
              </Link>
            )}
          </nav>
        </div>
      </header>
      <main className="mx-auto max-w-5xl px-6 py-10">{children}</main>
    </div>
  );
}
