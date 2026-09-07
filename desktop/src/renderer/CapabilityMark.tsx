import type { SVGProps } from "react";
import { isPublicIconName, type PublicIconName } from "../shared/themeContract.generated";

// Original 32-unit silhouettes: one familiar metaphor, few interior details.
// Named plugin icons retain their public meaning; no plugin IDs are special-cased.
const marks = {
  module: "M7 11h11a3 3 0 0 1 3 3v11H7a3 3 0 0 1-3-3v-8a3 3 0 0 1 3-3Z M12 11V5h12a3 3 0 0 1 3 3v11h-6",
  create: "m7 20 14-14a3 3 0 0 1 5 5L12 25l-7 2Z M19 8l5 5",
  inspect: "M23 14a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z M21 21l6 6",
  verify: "M26 16a10 10 0 1 1-7-9.5 M11 15l5 5L27 7",
  build: "m14 16-8 9a2 2 0 0 0 3 3l8-10 M11 6h10l6 6-5 5-6-6h-5l-4 3V9Z",
  branch: "M16 11v6 M7 22v-5h18v5 M19 7a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z M10 25a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z M28 25a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z",
  memory: "M9 5h14v22l-7-5-7 5V5Z",
  condense: "M8 5h11l6 6v16H8Z M19 5v6h6 M12 17h9 M12 22h6",
  cycle: "M26 16a10 10 0 1 1-20 0 10 10 0 0 1 20 0Z M16 10v7l5 3",
  rest: "M16 5a11 11 0 1 0 11 13A10 10 0 0 1 16 5Z",
  appearance: "M26 16a10 10 0 1 1-20 0 10 10 0 0 1 20 0Z M16 6c-7 6 7 14 0 20",
  dialogue: "M10 6h12a5 5 0 0 1 5 5v7a5 5 0 0 1-5 5h-8l-7 5v-6a5 5 0 0 1-2-4v-7a5 5 0 0 1 5-5Z M11 14h10",
  tasks: "m5 10 3 3 5-6 M18 10h9 m-22 12 3 3 5-6 M18 22h9",
  browser: "M8 6h16a3 3 0 0 1 3 3v15a3 3 0 0 1-3 3H8a3 3 0 0 1-3-3V9a3 3 0 0 1 3-3Z M5 12h22",
  code: "m11 8-7 8 7 8 M21 8l7 8-7 8",
  presentation: "M5 6h22v16H5Z M16 22v5 M11 27h10 M10 17l5-5 4 3 4-5",
  archive: "M5 6h22v6H5Z M7 12v14h18V12 M13 17h6",
  settings: "M5 10h7 M18 10h9 M5 22h13 M24 22h3 M18 10a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z M24 22a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z",
} as const;

type Motif = keyof typeof marks;

const namedMotifs: Record<PublicIconName, Motif> = {
  archive: "archive", bot: "branch", brain: "memory", calendar: "cycle",
  check: "verify", "check-circle": "verify", clock: "cycle", code: "code",
  database: "memory", "file-text": "condense", folder: "archive", gauge: "inspect",
  globe: "browser", inbox: "archive", "layout-grid": "module", "list-todo": "tasks",
  "message-square": "dialogue", moon: "rest", plug: "module", pulse: "inspect",
  search: "inspect", settings: "settings", shield: "verify", sliders: "settings",
  sparkles: "appearance", terminal: "code", users: "branch", workflow: "branch", wrench: "build",
};

// Skills have no icon descriptor. Prefer capability words over arbitrary hashes;
// unknown skills receive the shared module mark rather than an invented meaning.
export function skillCapability(name: string): Motif {
  const words = name.toLowerCase().split(/[^a-z0-9]+/);
  const groups: [Motif, string[]][] = [
    ["inspect", ["debug", "diagnosis", "diagnose"]],
    ["verify", ["review", "check", "test", "audit"]],
    ["create", ["creator", "create"]],
    ["build", ["build", "install", "deploy", "release"]],
    ["browser", ["browser", "web"]],
    ["presentation", ["pptx", "presentation", "slides"]],
    ["branch", ["commit", "git", "delegate"]],
    ["memory", ["memory", "remember"]],
  ];
  return groups.find(([, keywords]) => keywords.some((word) => words.includes(word)))?.[0] ?? "module";
}

export function CapabilityMark({ motif, name, ...props }: SVGProps<SVGSVGElement> & {
  motif?: Motif;
  name?: string;
}): JSX.Element {
  const resolved = motif ?? (name && isPublicIconName(name) ? namedMotifs[name] : "module");
  return (
    <svg width="24" height="24" viewBox="0 0 32 32" fill="none" stroke="currentColor"
      strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"
      aria-hidden="true" focusable="false" data-icon={name} {...props}>
      <path d={marks[resolved]} />
    </svg>
  );
}
