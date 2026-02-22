import type { Metadata } from "next";
import Link from "next/link";
import "./globals.css";

export const metadata: Metadata = {
  title: "Embedding Migration Demo",
  description: "Zero-downtime embedding model migration with Temporal",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body className="bg-zinc-950 text-zinc-100 antialiased">
        <nav className="border-b border-zinc-800 bg-zinc-900">
          <div className="mx-auto flex max-w-5xl items-center gap-6 px-4 py-3">
            <span className="text-sm font-semibold tracking-tight text-zinc-100">
              Embedding Migration
            </span>
            <div className="flex gap-4 text-sm">
              <Link
                href="/"
                className="text-zinc-400 hover:text-zinc-100 transition-colors"
              >
                Search
              </Link>
              <Link
                href="/migrations"
                className="text-zinc-400 hover:text-zinc-100 transition-colors"
              >
                Migrations
              </Link>
            </div>
          </div>
        </nav>
        <main className="mx-auto max-w-5xl px-4 py-8">{children}</main>
      </body>
    </html>
  );
}
