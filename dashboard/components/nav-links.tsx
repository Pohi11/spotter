"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

const NAV = [
  { href: "/#try", label: "Classify a tile" },
  { href: "/live", label: "Live events" },
  { href: "/model", label: "Model" },
  { href: "/drift", label: "Drift" },
  { href: "/results", label: "Results" },
  { href: "/architecture", label: "Architecture" },
  { href: "/build", label: "How it was built" },
] as const;

export function NavLinks() {
  const path = usePathname();
  return (
    <ul className="flex flex-wrap gap-x-5 gap-y-1 text-sm text-muted">
      {NAV.map((item) => {
        const current = item.href !== "/#try" && (path === item.href || path.startsWith(`${item.href}/`));
        return (
          <li key={item.href}>
            <Link
              href={item.href}
              aria-current={current ? "page" : undefined}
              className={`hover:text-ink ${current ? "font-medium text-ink" : ""}`}
            >
              {item.label}
            </Link>
          </li>
        );
      })}
    </ul>
  );
}
