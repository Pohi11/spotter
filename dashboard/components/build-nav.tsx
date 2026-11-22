import Link from "next/link";

import type { RequirementStatus } from "@/lib/walkthrough";

import { Chip } from "./ui";

const PAGES = [
  { href: "/build", label: "Changes" },
  { href: "/build/specs", label: "Requirements and tests" },
  { href: "/build/decisions", label: "Decision records" },
] as const;

export function BuildNav({ current }: { current: (typeof PAGES)[number]["href"] }) {
  return (
    <nav aria-label="How it was built" className="mt-8 mb-12 flex flex-wrap gap-2">
      {PAGES.map((p) => (
        <Link
          key={p.href}
          href={p.href}
          aria-current={p.href === current ? "page" : undefined}
          className={`rounded-full px-4 py-1.5 text-sm font-medium ring-1 ${p.href === current ? "bg-ink text-white ring-ink" : "ring-rule hover:bg-surface"}`}
        >
          {p.label}
        </Link>
      ))}
    </nav>
  );
}

const STATUS: Record<RequirementStatus, { tone: "pass" | "fail" | "wait"; label: string }> = {
  verified: { tone: "pass", label: "verified" },
  failing: { tone: "fail", label: "failing" },
  untested: { tone: "wait", label: "untested" },
};

export function RequirementBadge({ status }: { status: RequirementStatus }) {
  return <Chip tone={STATUS[status].tone}>{STATUS[status].label}</Chip>;
}
