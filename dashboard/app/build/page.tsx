import type { Metadata } from "next";
import Link from "next/link";

import { BuildNav, RequirementBadge } from "@/components/build-nav";
import { Md } from "@/components/markdown";
import { Figure, PageTitle } from "@/components/ui";
import data from "@/content/walkthrough.json";
import { SUITE_NAMES, type SuiteName, type Walkthrough, currentRequirement, decisionHref, requirementHref, totalTests } from "@/lib/walkthrough";

// "How it was built" (spec: build-walkthrough). Everything below the method comes from
// content/walkthrough.json, which tools/walkthrough generates from the repository.

export const metadata: Metadata = { title: "How it was built" };

const walkthrough = data as unknown as Walkthrough;

const METHOD = [
  {
    title: "Each change starts as a spec",
    text: "Every piece of work is an OpenSpec change: a proposal saying why, requirements written as when-then scenarios, a design that weighs the options, and tasks that each name the test proving them. A finished change is archived, and its requirements join the current specs.",
  },
  {
    title: "Decisions get a numbered record",
    text: "A choice that outlives one change gets a record with its context, the options considered, the decision, and the consequences. A committed record is never rewritten; when a decision changes, a new record supersedes it.",
  },
  {
    title: "Tests name the requirement they prove",
    text: "Go, Python, TypeScript, and browser tests each name the requirement they verify, in their standard reports. CI fails when a requirement has no passing test or a test names one that doesn't exist.",
  },
  {
    title: "Numbers are measured, then reported",
    text: "Every number in the README and on the results page comes from a dated report written by the code that measured it, with how it was measured. What went wrong along the way is recorded too.",
  },
];

const date = (d: string) => new Date(`${d}T00:00:00`).toLocaleDateString("en-US", { month: "long", day: "numeric", year: "numeric" });

export default function BuildPage() {
  const { totals, changes } = walkthrough;
  return (
    <>
      <PageTitle title="How Spotter was built">
        <p>
          The changes, requirements, tests, and decision records on these pages are generated from
          the repository and the latest test run; nothing is typed by hand except the method below.
        </p>
      </PageTitle>
      <BuildNav current="/build" />

      <dl className="grid grid-cols-2 gap-x-8 gap-y-6 md:grid-cols-4" data-testid="totals">
        <Figure label="Requirements verified" value={`${totals.verified} of ${totals.requirements}`} />
        <Figure
          label="Tests"
          value={totalTests(walkthrough).toLocaleString()}
          note={(Object.keys(totals.tests) as SuiteName[]).map((s) => `${totals.tests[s]} ${SUITE_NAMES[s]}`).join(", ")}
        />
        <Figure label="Changes" value={totals.changes} />
        <Figure label="Decision records" value={walkthrough.decisions.length} />
      </dl>
      <p className="mt-3 text-sm text-faint">
        Generated {new Date(walkthrough.generated_at).toLocaleString("en-US", { dateStyle: "long", timeStyle: "short", timeZone: "UTC" })} UTC
        from the latest test reports.
      </p>

      <section className="mt-14 grid gap-x-10 gap-y-8 md:grid-cols-2">
        {METHOD.map((m) => (
          <div key={m.title}>
            <h2 className="text-xl font-semibold">{m.title}</h2>
            <p className="mt-2 leading-relaxed text-muted">{m.text}</p>
          </div>
        ))}
      </section>

      <h2 className="mt-16 mb-6 text-3xl font-semibold">Changes, in order</h2>
      <ol aria-label="Changes" className="space-y-4 border-l-2 border-rule pl-6 sm:pl-8">
        {changes.map((change) => {
          const items = (change.tasks ?? []).flatMap((g) => g.items);
          return (
            <li key={change.id} className="relative" data-change={change.slug}>
              <span aria-hidden className="absolute top-2 -left-[33px] h-4 w-4 rounded-full border-4 border-paper bg-pass sm:-left-[41px]" />
              <article className="rounded-2xl bg-surface p-5 ring-1 ring-rule sm:p-6">
                <p className="text-sm text-faint">{date(change.date)}</p>
                <h3 className="mt-0.5 text-xl font-semibold">{change.title}</h3>
                <div className="mt-2">
                  <Md base={`openspec/changes/archive/${change.id}/proposal.md`}>{change.why}</Md>
                </div>
                <details className="mt-4">
                  <summary className="cursor-pointer text-sm font-medium text-code">
                    What changed: {change.requirements.length} requirements, {items.filter((i) => i.done).length} of {items.length} tasks done
                  </summary>
                  <div className="mt-4 space-y-5">
                    <Md base={`openspec/changes/archive/${change.id}/proposal.md`}>{change.what_changes}</Md>
                    {change.requirements.length > 0 && (
                      <div>
                        <h4 className="font-medium">Requirements</h4>
                        <ul className="mt-2 space-y-1.5 text-sm">
                          {change.requirements.map((ref) => {
                            const current = currentRequirement(walkthrough, ref.capability, ref.name);
                            return (
                              <li key={`${ref.kind}-${ref.capability}-${ref.name}`} className="flex flex-wrap items-center gap-2">
                                <span className="w-16 text-faint">{ref.kind}</span>
                                {current ? (
                                  <Link href={requirementHref(ref.capability, ref.name)} className="text-code hover:underline">
                                    {ref.name}
                                  </Link>
                                ) : (
                                  <span className="text-muted">{ref.name}</span>
                                )}
                                {current && <RequirementBadge status={current.status} />}
                              </li>
                            );
                          })}
                        </ul>
                      </div>
                    )}
                    {change.decisions.length > 0 && (
                      <div>
                        <h4 className="font-medium">Decision records it relied on</h4>
                        <ul className="mt-2 space-y-1 text-sm">
                          {change.decisions.map((n) => {
                            const record = walkthrough.decisions.find((d) => d.number === n);
                            return (
                              <li key={n}>
                                <Link href={decisionHref(n)} className="text-code hover:underline">
                                  {n}: {record?.title ?? "decision record"}
                                </Link>
                              </li>
                            );
                          })}
                        </ul>
                      </div>
                    )}
                    <div>
                      <h4 className="font-medium">Tasks</h4>
                      <ul className="mt-2 space-y-1.5">
                        {items.map((item) => (
                          <li key={item.id} className="flex gap-2 text-sm">
                            <span aria-label={item.done ? "done" : "not done"} className={item.done ? "text-pass" : "text-faint"}>
                              {item.done ? "✓" : "○"}
                            </span>
                            <span className="text-muted">
                              <Md>{item.text}</Md>
                            </span>
                          </li>
                        ))}
                      </ul>
                    </div>
                  </div>
                </details>
              </article>
            </li>
          );
        })}
      </ol>
    </>
  );
}
