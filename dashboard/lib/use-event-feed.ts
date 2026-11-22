"use client";

import { useEffect, useState } from "react";

import { addEvent } from "@/lib/events";
import type { FeedEvent } from "@/lib/types";

export type FeedState = "connecting" | "live" | "reconnecting";

/**
 * Follows GET /v1/events through the proxy. EventSource reconnects on its own after a
 * drop; each reconnect replays the window, and addEvent drops the repeats.
 */
export function useEventFeed(since = "10m", limit = 300, streams?: string) {
  const [events, setEvents] = useState<FeedEvent[]>([]);
  const [state, setState] = useState<FeedState>("connecting");
  useEffect(() => {
    const q = new URLSearchParams({ since });
    if (streams) q.set("streams", streams);
    const source = new EventSource(`/api/v1/events?${q}`);
    source.onopen = () => setState("live");
    source.onerror = () => setState("reconnecting");
    source.onmessage = (m) => {
      try {
        const ev = JSON.parse(m.data) as FeedEvent;
        setEvents((list) => addEvent(list, ev, limit));
        setState("live");
      } catch {
        // A malformed event is skipped; the feed only sends JSON.
      }
    };
    return () => source.close();
  }, [since, limit, streams]);
  return { events, state };
}
