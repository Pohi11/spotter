import { className } from "@/lib/classes";

// The home page's opening: the ten sample tiles as the production model classified them
// in the recorded run, laid out like a contact sheet of satellite frames.

export interface ContactTile {
  id: string;
  file: string;
  true_class: string;
  label: string | null;
  confidence: number | null;
  version: number | null;
}

export function ContactSheet({ tiles, highlight, recordedAt }: { tiles: ContactTile[]; highlight: string; recordedAt: string }) {
  const version = tiles.find((t) => t.version)?.version;
  return (
    <figure aria-label="Ten sample tiles and how the production model classified them" className="rounded-2xl bg-ink p-4 text-white sm:p-5">
      <ol className="grid grid-cols-2 gap-3 min-[420px]:grid-cols-3 sm:grid-cols-5 lg:grid-cols-2">
        {tiles.map((t, i) => {
          const right = t.label === t.true_class;
          return (
            <li key={t.id} className="develop" style={{ animationDelay: `${120 + i * 70}ms` }}>
              <div className={`relative overflow-hidden rounded-md ${t.id === highlight ? "ring-2 ring-pass ring-offset-2 ring-offset-ink" : ""}`}>
                {/* eslint-disable-next-line @next/next/no-img-element -- 64-pixel EuroSAT tiles, shown pixelated */}
                <img src={`/samples/${t.file}`} alt="" width={64} height={64} className="aspect-square w-full [image-rendering:pixelated]" />
              </div>
              <p className="mt-1.5 truncate text-[13px] font-medium" title={className(t.label)}>
                {className(t.label)}
              </p>
              <p className={`text-xs ${right ? "text-white/60" : "text-[#ff9c94]"}`}>
                {t.confidence !== null ? `${(t.confidence * 100).toFixed(1)}%` : "?"}
                {right ? "" : `, true class ${className(t.true_class)}`}
              </p>
            </li>
          );
        })}
      </ol>
      <figcaption className="mt-4 border-t border-white/15 pt-3 text-xs leading-relaxed text-white/65">
        Ten holdout tiles, one per class, as production version {version} classified them on{" "}
        {new Date(recordedAt).toLocaleDateString("en-US", { month: "long", day: "numeric", year: "numeric" })}. The outlined
        tile is followed step by step below.
      </figcaption>
    </figure>
  );
}
