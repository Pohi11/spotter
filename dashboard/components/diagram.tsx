// Static SVG diagrams (spec: dashboard "Architecture diagrams have text equivalents").
// Server components: no client JavaScript. Colors follow the dashboard's tokens, so a teal
// box is the event bus here and on every other page.

import type { ReactNode } from "react";

export type BoxTone = "code" | "bus" | "model" | "pass" | "wait" | "human" | "fail" | "plain" | "store";

const BOX: Record<BoxTone, string> = {
  code: "fill-code-soft stroke-code",
  bus: "fill-bus-soft stroke-bus",
  model: "fill-model-soft stroke-model",
  pass: "fill-pass-soft stroke-pass",
  wait: "fill-wait-soft stroke-wait",
  human: "fill-human-soft stroke-human",
  fail: "fill-fail-soft stroke-fail",
  plain: "fill-surface stroke-faint",
  store: "fill-wash stroke-ink",
};

export type Ink = "ink" | "code" | "bus" | "model" | "pass" | "human" | "fail" | "faint";

const LINE: Record<Ink, string> = {
  ink: "stroke-ink",
  code: "stroke-code",
  bus: "stroke-bus",
  model: "stroke-model",
  pass: "stroke-pass",
  human: "stroke-human",
  fail: "stroke-fail",
  faint: "stroke-faint",
};

const HEAD: Record<Ink, string> = {
  ink: "fill-ink",
  code: "fill-code",
  bus: "fill-bus",
  model: "fill-model",
  pass: "fill-pass",
  human: "fill-human",
  fail: "fill-fail",
  faint: "fill-faint",
};

/**
 * One diagram: a heading that names it, the SVG, and a description screen readers get
 * with it. Below `minWidth` the drawing scrolls inside its frame, never the page.
 */
export function Diagram({
  id,
  title,
  description,
  caption,
  width,
  height,
  minWidth,
  className = "",
  children,
}: {
  id: string;
  title: string;
  description: ReactNode;
  caption?: ReactNode;
  width: number;
  height: number;
  minWidth?: number;
  className?: string;
  children: ReactNode;
}) {
  const scrolls = minWidth ? { tabIndex: 0, role: "region", "aria-labelledby": `${id}-title` } : {};
  return (
    <figure className={className}>
      <h3 id={`${id}-title`} className="text-lg font-semibold">
        {title}
      </h3>
      <div className="-mx-4 mt-4 overflow-x-auto bg-surface p-3 ring-1 ring-rule sm:mx-0 sm:rounded-2xl sm:p-6" {...scrolls}>
        <svg
          viewBox={`0 0 ${width} ${height}`}
          role="img"
          aria-labelledby={`${id}-title`}
          aria-describedby={`${id}-desc`}
          className="h-auto w-full"
          style={minWidth ? { minWidth } : undefined}
        >
          <defs>
            {(Object.keys(HEAD) as Ink[]).map((ink) => (
              <marker
                key={ink}
                id={`${id}-head-${ink}`}
                viewBox="0 0 10 10"
                refX="9"
                refY="5"
                markerUnits="userSpaceOnUse"
                markerWidth="10"
                markerHeight="10"
                orient="auto-start-reverse"
              >
                <path d="M0 0 L10 5 L0 10 z" className={HEAD[ink]} />
              </marker>
            ))}
          </defs>
          {children}
        </svg>
      </div>
      {caption && <p className="mt-3 text-sm text-faint">{caption}</p>}
      <div id={`${id}-desc`} className="sr-only">
        {description}
      </div>
    </figure>
  );
}

/** A rounded box with a title and up to three lines under it, centered. */
export function Box({
  x,
  y,
  w,
  h,
  tone,
  title,
  sub = [],
  dashed = false,
}: {
  x: number;
  y: number;
  w: number;
  h: number;
  tone: BoxTone;
  title: string;
  sub?: string[];
  dashed?: boolean;
}) {
  const titleY = y + h / 2 + 5 - sub.length * 8.5;
  return (
    <g>
      <rect x={x} y={y} width={w} height={h} rx={10} className={BOX[tone]} strokeWidth={1.5} strokeDasharray={dashed ? "5 4" : undefined} />
      <text x={x + w / 2} y={titleY} textAnchor="middle" className="fill-ink text-[15px] font-semibold">
        {title}
      </text>
      {sub.map((line, i) => (
        <text key={line} x={x + w / 2} y={titleY + 19 + i * 16} textAnchor="middle" className="fill-muted text-[13px]">
          {line}
        </text>
      ))}
    </g>
  );
}

/** A data store: a cylinder with a title and one line under it. */
export function Store({ x, y, w, h, title, sub }: { x: number; y: number; w: number; h: number; title: string; sub?: string }) {
  const ry = 9;
  return (
    <g>
      <path
        d={`M${x} ${y + ry} A${w / 2} ${ry} 0 0 1 ${x + w} ${y + ry} V${y + h - ry} A${w / 2} ${ry} 0 0 1 ${x} ${y + h - ry} Z`}
        className={BOX.store}
        strokeWidth={1.5}
      />
      <path d={`M${x} ${y + ry} A${w / 2} ${ry} 0 0 0 ${x + w} ${y + ry}`} className="fill-none stroke-ink" strokeWidth={1.5} />
      <text x={x + w / 2} y={y + h / 2 + 8} textAnchor="middle" className="fill-ink text-[15px] font-semibold">
        {title}
      </text>
      {sub && (
        <text x={x + w / 2} y={y + h / 2 + 26} textAnchor="middle" className="fill-muted text-[13px]">
          {sub}
        </text>
      )}
    </g>
  );
}

/** A line along `d` with an arrowhead at its end, optionally labeled at (lx, ly). */
export function Arrow({
  diagram,
  d,
  ink = "ink",
  heavy = false,
  dashed = false,
  label,
  lx,
  ly,
  anchor = "middle",
}: {
  diagram: string;
  d: string;
  ink?: Ink;
  heavy?: boolean;
  dashed?: boolean;
  label?: string;
  lx?: number;
  ly?: number;
  anchor?: "start" | "middle" | "end";
}) {
  return (
    <g>
      <path
        d={d}
        className={`fill-none ${LINE[ink]}`}
        strokeWidth={heavy ? 2.5 : 1.5}
        strokeDasharray={dashed ? "5 4" : undefined}
        markerEnd={`url(#${diagram}-head-${ink})`}
      />
      {label && lx !== undefined && ly !== undefined && (
        <Note x={lx} y={ly} anchor={anchor}>
          {label}
        </Note>
      )}
    </g>
  );
}

/** Small text on the drawing, with a white halo so it stays readable over lines. */
export function Note({
  x,
  y,
  anchor = "start",
  muted = true,
  bold = false,
  children,
}: {
  x: number;
  y: number;
  anchor?: "start" | "middle" | "end";
  muted?: boolean;
  bold?: boolean;
  children: ReactNode;
}) {
  return (
    <text
      x={x}
      y={y}
      textAnchor={anchor}
      className={`${muted ? "fill-muted" : "fill-ink"} ${bold ? "font-semibold" : ""} stroke-surface text-[13px]`}
      strokeWidth={4}
      paintOrder="stroke"
      strokeLinejoin="round"
    >
      {children}
    </text>
  );
}

/** A labeled region behind a group of boxes (a process, a cluster, a layer). */
export function Region({ x, y, w, h, label, dashed = false }: { x: number; y: number; w: number; h: number; label: string; dashed?: boolean }) {
  return (
    <g>
      <rect x={x} y={y} width={w} height={h} rx={16} className="fill-wash stroke-rule" strokeWidth={1.5} strokeDasharray={dashed ? "6 5" : undefined} />
      <Note x={x + 16} y={y + 24} muted={false} bold>
        {label}
      </Note>
    </g>
  );
}
