import type { Metadata } from "next";
import Link from "next/link";
import { Building2 } from "lucide-react";
import { NavLinks } from "@/components/nav-links";
import { PostHogProvider } from "@/lib/posthog";
import "./globals.css";

export const metadata: Metadata = {
  title: "StaySearch — Semantic Hotel Search",
  description:
    "Find your perfect hotel with AI-powered semantic search. Zero-downtime embedding migration demo powered by Temporal.",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body className="flex min-h-screen flex-col bg-background text-foreground antialiased">
        <PostHogProvider>
        {/* Top nav */}
        <header className="sticky top-0 z-50 border-b bg-white/80 backdrop-blur-md">
          <div className="mx-auto flex h-14 max-w-6xl items-center justify-between px-4">
            {/* Brand */}
            <Link href="/" className="flex items-center gap-2">
              <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary">
                <Building2 className="h-4 w-4 text-white" />
              </div>
              <span className="text-lg font-bold tracking-tight text-foreground">
                StaySearch
              </span>
            </Link>

            {/* Navigation */}
            <NavLinks />
          </div>
        </header>

        <main className="mx-auto w-full max-w-6xl flex-1 px-4 py-6">{children}</main>

        {/* Footer */}
        <footer className="border-t bg-muted/40">
          <div className="mx-auto flex max-w-6xl items-center justify-between px-4 py-4 text-xs text-muted-foreground">
            <span>Semantic Hotel Search &mdash; Embedding Migration Demo</span>
            <span>Powered by Temporal + Ollama</span>
          </div>
        </footer>
        </PostHogProvider>
      </body>
    </html>
  );
}
