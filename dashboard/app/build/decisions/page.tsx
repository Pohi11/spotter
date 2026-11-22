import type { Metadata } from "next";

import { BuildNav } from "@/components/build-nav";
import { Md } from "@/components/markdown";
import { PageTitle } from "@/components/ui";
import data from "@/content/walkthrough.json";
import { type Walkthrough, decisionAnchor } from "@/lib/walkthrough";

// Every decision record in full (spec: build-walkthrough).

export const metadata: Metadata = { title: "Decision records" };

const walkthrough = data as unknown as Walkthrough;

export default function DecisionsPage() {
  return (
    <>
      <PageTitle title="Decision records">
        <p>
          One record per decision that outlives a change: the context, the options considered, the
          decision, and its consequences. Records are never rewritten; a changed decision gets a new
          record that supersedes the old one.
        </p>
      </PageTitle>
      <BuildNav current="/build/decisions" />

      <ol className="space-y-6">
        {walkthrough.decisions.map((d) => (
          <li key={d.number} id={decisionAnchor(d.number)} className="scroll-mt-6 rounded-2xl bg-surface p-5 ring-1 ring-rule target:ring-2 target:ring-code sm:p-7">
            <details>
              <summary className="cursor-pointer list-none">
                <span className="text-sm text-faint">
                  {d.number}, {d.date}, {d.status}
                </span>
                <h2 className="mt-1 text-xl font-semibold">{d.title}</h2>
                <span className="mt-2 inline-block text-sm text-code">Read the record</span>
              </summary>
              <div className="mt-4 border-t border-rule pt-4">
                <Md base={`docs/decisions/${d.number}.md`}>{d.markdown.replace(/^# .*\n/, "")}</Md>
              </div>
            </details>
          </li>
        ))}
      </ol>
    </>
  );
}
