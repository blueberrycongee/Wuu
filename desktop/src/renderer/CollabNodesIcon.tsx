import type { SVGProps } from "react";

/**
 * Three fully-connected peer nodes for the 协作 (Collaboration) section:
 * every node links to every other with no hub — collaborators as equals.
 * Nodes are solid dots, not stroked rings: at the 18px header size a
 * stroked r<=3 ring's inner hole collapses to mud, while a solid dot stays
 * crisp against the 2px connector lines. Kept abstract (not people) so it
 * doesn't duplicate the Agents nav entry's UsersRound glyph.
 */
export function CollabNodesIcon(props: SVGProps<SVGSVGElement>): JSX.Element {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      data-icon="collab-nodes"
      {...props}
    >
      <path d="M10.73 7.44L6.27 15.56" />
      <path d="M13.27 7.44L17.73 15.56" />
      <path d="M7.75 18h8.5" />
      <circle cx="12" cy="5" r="2.75" fill="currentColor" stroke="none" />
      <circle cx="5" cy="18" r="2.75" fill="currentColor" stroke="none" />
      <circle cx="19" cy="18" r="2.75" fill="currentColor" stroke="none" />
    </svg>
  );
}
