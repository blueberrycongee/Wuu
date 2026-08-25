import type { JSX } from "react";

/**
 * The line-start marker for a tool activity row.
 *
 * It is deliberately non-semantic: not a gear, not a brain. The shape is a
 * small rounded square — an "action cell" — and the only thing it encodes is
 * execution state:
 *
 *   - running: a bright segment chases around the perimeter, like a cursor
 *     on a track, so the eye can find the still-active row among settled ones;
 *   - settled: a quiet hollow square, the row's resting state;
 *
 * The chase is pure stroke-dash animation on a `pathLength`-normalized path,
 * so there is no rotation and no "spinner" reading at 12px. Under
 * `prefers-reduced-motion` it degrades to the static outline.
 */
export function ToolActivityMarker({
  running = false,
}: {
  running?: boolean;
}): JSX.Element {
  const stateClass = running ? "is-running" : "is-settled";
  return (
    <svg
      className={`tool-activity-marker ${stateClass}`}
      width="12"
      height="12"
      viewBox="0 0 12 12"
      aria-hidden
      focusable="false"
    >
      <rect
        className="tool-activity-marker-track"
        x="2"
        y="2"
        width="8"
        height="8"
        rx="1.5"
        pathLength={100}
      />
    </svg>
  );
}
