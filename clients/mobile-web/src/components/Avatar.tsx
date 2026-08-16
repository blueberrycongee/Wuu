// Deterministic initials avatar: one surface shared by hosts and threads.

export function Avatar({
  name,
  small = false,
  round = false,
}: {
  name: string;
  small?: boolean;
  round?: boolean;
}): React.JSX.Element {
  const clean = name.trim();
  const initial = clean ? [...clean][0].toUpperCase() : "?";
  const hue = hueOf(clean || "?");
  const cls = ["avatar", small ? "small" : "", round ? "round" : ""]
    .filter(Boolean)
    .join(" ");
  return (
    <div className={cls} style={{ background: `hsl(${hue} 55% 45%)` }} aria-hidden>
      {initial}
    </div>
  );
}

function hueOf(text: string): number {
  let hash = 0;
  for (let i = 0; i < text.length; i++) {
    hash = (hash * 31 + text.codePointAt(i)!) >>> 0;
  }
  return hash % 360;
}
