import type { SVGProps } from "react";

/** A pair of offset, connectable blocks for the Plugins navigation entry. */
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
      <path d="M4 7.5h2.5v-1a1.5 1.5 0 0 1 3 0v1H14v6H4z" />
      <path d="M10 13.5h2.5v-1a1.5 1.5 0 0 1 3 0v1H20v6H10z" />
    </svg>
  );
}
