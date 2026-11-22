import type { Metadata } from "next";
import Link from "next/link";

import { LiveFeed } from "@/components/live-feed";
import { Code, PageTitle, link } from "@/components/ui";

export const metadata: Metadata = { title: "Live events" };

export default function LivePage() {
  return (
    <>
      <PageTitle title="Live events">
        <p>
          Everything the services say to each other goes over NATS JetStream as a protobuf
          event. This page follows all three streams through the gateway&apos;s{" "}
          <Code>/v1/events</Code> feed, which decodes each event by its type and sends it as a
          server-sent event. Select a row to see the full payload.
        </p>
        <p className="mt-3 text-base">
          To see a job go through, <Link href="/#try" className={link}>classify a tile</Link> in
          another tab.
        </p>
      </PageTitle>
      <LiveFeed />
    </>
  );
}
