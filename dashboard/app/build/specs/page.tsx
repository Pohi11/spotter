import type { Metadata } from "next";

import { BuildNav, RequirementBadge } from "@/components/build-nav";
import { Md } from "@/components/markdown";
import { Chip, PageTitle } from "@/components/ui";
import data from "@/content/walkthrough.json";
import { SUITE_NAMES, type Walkthrough, requirementAnchor } from "@/lib/walkthrough";

// Every current requirement with its scenarios and linked tests (spec: build-walkthrough).

export const metadata: Metadata = { title: "Requirements and tests" };

const walkthrough = data as unknown as Walkthrough;

export default function SpecsPage() {
  const { totals } = walkthrough;
  return (
    <>
      <PageTitle title="Requirements and tests">
        <p>
          Every requirement in the current specs, grouped by capability, with its scenarios and the
          tests that name it. {totals.verified} of {totals.requirements} are verified: at least one
          linked test passed in the latest run and none failed.
        </p>
      </PageTitle>
      <BuildNav current="/build/specs" />

      <nav aria-label="Capabilities" className="mb-12 flex flex-wrap gap-x-4 gap-y-1 text-sm">
        {walkthrough.specs.map((c) => (
          <a key={c.capability} href={`#cap-${c.capability}`} className="text-code hover:underline">
            {c.capability}
          </a>
        ))}
      </nav>

      <div className="space-y-16">
        {walkthrough.specs.map((cap) => (
          <section key={cap.capability} id={`cap-${cap.capability}`} aria-labelledby={`cap-${cap.capability}-h`} className="scroll-mt-6">
            <h2 id={`cap-${cap.capability}-h`} className="font-mono text-2xl font-semibold">
              {cap.capability}
            </h2>
            <div className="mt-2 max-w-3xl">
              <Md base={`openspec/specs/${cap.capability}/spec.md`}>{cap.purpose}</Md>
            </div>
            <ul className="mt-6 space-y-4">
              {cap.requirements.map((r) => (
                <li
                  key={r.name}
                  id={requirementAnchor(cap.capability, r.name)}
                  data-status={r.status}
                  className="scroll-mt-6 rounded-2xl bg-surface p-5 ring-1 ring-rule target:ring-2 target:ring-code sm:p-6"
                >
                  <div className="flex flex-wrap items-center gap-3">
                    <h3 className="text-lg font-semibold">{r.name}</h3>
                    <RequirementBadge status={r.status} />
                  </div>
                  <div className="mt-2">
                    <Md base={`openspec/specs/${cap.capability}/spec.md`}>{r.text}</Md>
                  </div>
                  <ul className="mt-4 space-y-1.5 text-sm" aria-label={`Tests for ${r.name}`}>
                    {r.tests.map((t) => (
                      <li key={`${t.suite}-${t.id}`} className="flex items-start gap-2">
                        <Chip tone={t.status === "passed" ? "pass" : t.status === "failed" ? "fail" : "wait"}>{t.status}</Chip>
                        <span className="min-w-0">
                          <span className="text-faint">{SUITE_NAMES[t.suite]} </span>
                          <span className="font-mono text-xs [overflow-wrap:anywhere]">{t.id}</span>
                        </span>
                      </li>
                    ))}
                    {r.tests.length === 0 && <li className="text-fail">No test names this requirement.</li>}
                  </ul>
                  <details className="mt-3">
                    <summary className="cursor-pointer text-sm font-medium text-code">{r.scenarios.length} {r.scenarios.length === 1 ? "scenario" : "scenarios"}</summary>
                    <ul className="mt-3 space-y-3 text-sm">
                      {r.scenarios.map((s) => (
                        <li key={s.name}>
                          <p className="font-medium">{s.name}</p>
                          {s.steps.map((step) => (
                            <div key={step} className="text-muted">
                              <Md>{step}</Md>
                            </div>
                          ))}
                        </li>
                      ))}
                    </ul>
                  </details>
                </li>
              ))}
            </ul>
          </section>
        ))}
      </div>
    </>
  );
}
