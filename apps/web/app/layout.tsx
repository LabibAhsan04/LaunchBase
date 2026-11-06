import type { Metadata } from "next";
import type { ReactNode } from "react";

import { Shell } from "@/components/shell";
import "./globals.css";

export const metadata: Metadata = {
  title: "LaunchBase",
  description: "Deploy a GitHub repository and get a URL.",
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en" className="h-full">
      <body className="min-h-full antialiased">
        <Shell>{children}</Shell>
      </body>
    </html>
  );
}
