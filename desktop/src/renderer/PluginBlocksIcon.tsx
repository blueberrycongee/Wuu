import type { SVGProps } from "react";

/** Three connectable blocks for the Plugins navigation entry. */
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
      <path d="M3 7.5h2.5v-1a1.5 1.5 0 0 1 3 0v1H13v6H3z" />
      <path d="M9 15h2.5v-1a1.5 1.5 0 0 1 3 0v1H19v6H9z" />
      <path d="M16 3h5v9.5h-5V10h-1a1.5 1.5 0 0 1 0-3h1z" />
    </svg>
  );
}
