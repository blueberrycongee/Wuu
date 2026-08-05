import type { SVGProps } from "react";

/**
 * Stacked building blocks (积木) for the Plugins navigation entry: a wide
 * base brick with a narrower brick on top, studs marking the connection
 * points. Drawn as one silhouette path — at the 18px sidebar size separate
 * brick outlines read as noise, while a single stepped outline with studs
 * stays crisp. Symmetric about x=12, ink spans 3..21 like its lucide
 * neighbours.
 */
export function PluginBlocksIcon(props: SVGProps<SVGSVGElement>): JSX.Element {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      data-icon="plugin-blocks"
      {...props}
    >
      <path d="M3 21h18v-8h-.75v-2h-2.5v2H17V6h-1.5V4h-2.5v2H11V4H8.5v2H7v7h-.75v-2h-2.5v2H3v8z" />
    </svg>
  );
}
