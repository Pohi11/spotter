import type { Metadata } from "next";
import Link from "next/link";

import { Demo } from "@/components/demo/demo";
import { PageTitle, link } from "@/components/ui";
import { MODEL_NAME } from "@/lib/gateway";

// The guided demo (spec: dashboard "The demo page walks through the loop step by step").

export const metadata: Metadata = { title: "Guided demo" };

export default function DemoPage() {
  return (
    <>
      <PageTitle title="Guided demo">
        <p>
          One situation, start to finish: ordinary traffic, then haze rolls in. The monitor notices without any labels, a new version is trained and
          shadowed without anyone starting it, a person approves it, and it takes over. Each step sends real tiles through the same API any client
          uses, and moves on only when the platform&apos;s own events say it happened.
        </p>
        <p className="mt-3 text-base">
          The right-hand side shows what&apos;s happening behind the scenes: who published each event and why, the monitor&apos;s arithmetic bin by
          bin, and accuracy the platform itself can&apos;t see. The <Link href="/architecture" className={link}>architecture</Link> page draws the
          parts.
        </p>
      </PageTitle>
      <Demo model={MODEL_NAME} />
    </>
  );
}
