import type { Metadata } from "next";
import { Poppins } from "next/font/google";
import Link from "next/link";
import type { ReactNode } from "react";

import { NavLinks } from "@/components/nav-links";

import "./globals.css";

const poppins = Poppins({
  variable: "--font-poppins",
  subsets: ["latin"],
  weight: ["400", "500", "600", "700"],
});

export const metadata: Metadata = {
  title: { default: "Spotter", template: "%s · Spotter" },
  description: "Deploy, serve, and monitor computer-vision models on overhead imagery",
};

// Links that leave the dashboard (spec: dashboard).
const SOURCE_URL = "https://github.com/Pohi11/spotter";
const PORTFOLIO_URL = "https://pohi.me/";

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en" className={`${poppins.variable} h-full antialiased`}>
      <body className="flex min-h-full flex-col">
        <a
          href="#main"
          className="sr-only focus:not-sr-only focus:fixed focus:top-3 focus:left-3 focus:z-50 focus:rounded-lg focus:bg-surface focus:px-4 focus:py-2 focus:ring-2 focus:ring-code"
        >
          Skip to content
        </a>
        <header className="border-b border-rule bg-surface/85 backdrop-blur">
          <nav aria-label="Main" className="mx-auto flex max-w-6xl flex-wrap items-center gap-x-6 gap-y-2 px-4 py-3 sm:px-6">
            <Link href="/" className="mr-1 flex items-center gap-2 text-lg font-semibold tracking-tight">
              <Mark />
              Spotter
            </Link>
            <NavLinks />
            <div className="ml-auto flex items-center gap-2">
              <ExternalLink href={SOURCE_URL} className="text-ink ring-1 ring-rule hover:bg-wash" icon={<GitHubMark />}>
                Source code
              </ExternalLink>
              <ExternalLink href={PORTFOLIO_URL} className="bg-ink text-white hover:bg-ink/85">
                Portfolio
              </ExternalLink>
            </div>
          </nav>
        </header>
        <main id="main" className="mx-auto w-full max-w-6xl flex-1 px-4 py-10 sm:px-6">
          {children}
        </main>
        <footer className="border-t border-rule">
          <p className="mx-auto max-w-6xl px-4 py-6 text-sm text-faint sm:px-6">
            Imagery: EuroSAT (Helber et al., 2019), Sentinel-2 tiles, MIT License. Degraded tiles are
            synthetic. No restricted or operational data is used anywhere in Spotter.
          </p>
        </footer>
      </body>
    </html>
  );
}

function ExternalLink({ href, className, icon, children }: { href: string; className: string; icon?: ReactNode; children: ReactNode }) {
  return (
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      className={`inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-sm font-medium transition-colors ${className}`}
    >
      {icon}
      {children}
      <span className="sr-only"> (opens in a new tab)</span>
    </a>
  );
}

function GitHubMark() {
  return (
    <svg aria-hidden viewBox="0 0 16 16" className="h-4 w-4" fill="currentColor">
      <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0 0 16 8c0-4.42-3.58-8-8-8z" />
    </svg>
  );
}

/** Three-by-three tiles with one marked: a classified tile in a mosaic. */
function Mark() {
  const cells = [0, 1, 2, 3, 4, 5, 6, 7, 8];
  return (
    <svg aria-hidden viewBox="0 0 24 24" className="h-6 w-6">
      {cells.map((i) => (
        <rect
          key={i}
          x={1 + (i % 3) * 7.6}
          y={1 + Math.floor(i / 3) * 7.6}
          width={6.4}
          height={6.4}
          rx={1.2}
          className={i === 4 ? "fill-pass" : i % 2 ? "fill-code/35" : "fill-bus/45"}
        />
      ))}
    </svg>
  );
}
