// Inline stroke icons (24px grid, currentColor). The UI avoids emoji and
// external icon fonts so touch targets stay crisp in both color schemes.

type IconProps = {
  size?: number;
  className?: string;
};

function base(size: number) {
  return {
    width: size,
    height: size,
    viewBox: "0 0 24 24",
    fill: "none",
    stroke: "currentColor",
    strokeWidth: 2,
    strokeLinecap: "round" as const,
    strokeLinejoin: "round" as const,
    "aria-hidden": true,
  };
}

export function IconMenu({ size = 22, className }: IconProps): React.JSX.Element {
  return (
    <svg {...base(size)} className={className}>
      <line x1="4" y1="7" x2="20" y2="7" />
      <line x1="4" y1="17" x2="16" y2="17" />
    </svg>
  );
}

export function IconPlus({ size = 20, className }: IconProps): React.JSX.Element {
  return (
    <svg {...base(size)} className={className}>
      <line x1="12" y1="5" x2="12" y2="19" />
      <line x1="5" y1="12" x2="19" y2="12" />
    </svg>
  );
}

export function IconChevronDown({ size = 16, className }: IconProps): React.JSX.Element {
  return (
    <svg {...base(size)} className={className}>
      <polyline points="6 9 12 15 18 9" />
    </svg>
  );
}

export function IconArrowUp({ size = 18, className }: IconProps): React.JSX.Element {
  return (
    <svg {...base(size)} className={className}>
      <line x1="12" y1="19" x2="12" y2="5" />
      <polyline points="5 12 12 5 19 12" />
    </svg>
  );
}

export function IconStop({ size = 16, className }: IconProps): React.JSX.Element {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="currentColor"
      stroke="none"
      aria-hidden
      className={className}
    >
      <rect x="6" y="6" width="12" height="12" rx="2.5" />
    </svg>
  );
}

export function IconPin({ size = 16, className }: IconProps): React.JSX.Element {
  return (
    <svg {...base(size)} className={className}>
      <path d="M12 17v5" />
      <path d="M9 10.76a2 2 0 0 1-1.11 1.79l-1.78.9A2 2 0 0 0 5 15.24V16a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-.76a2 2 0 0 0-1.11-1.79l-1.78-.9A2 2 0 0 1 15 10.76V6h1a2 2 0 0 0 0-4H8a2 2 0 0 0 0 4h1z" />
    </svg>
  );
}

export function IconCheck({ size = 18, className }: IconProps): React.JSX.Element {
  return (
    <svg {...base(size)} className={className}>
      <polyline points="20 6 9 17 4 12" />
    </svg>
  );
}

export function IconRefresh({ size = 16, className }: IconProps): React.JSX.Element {
  return (
    <svg {...base(size)} className={className}>
      <path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8" />
      <path d="M21 3v5h-5" />
      <path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16" />
      <path d="M8 16H3v5" />
    </svg>
  );
}
