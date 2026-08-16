// Participant avatar: a die-cut mascot from the fixed 12-character cast on
// a muted background tint, mirroring the desktop DefaultAvatar assignment
// (same FNV-1a seeds → the same participant renders the same mascot + tint
// on both ends). Logic lives in ../lib/avatar; this file only owns the
// static image table, which Vite resolves at build time.

import mascot0 from "../assets/avatars/mascot-0.png";
import mascot1 from "../assets/avatars/mascot-1.png";
import mascot2 from "../assets/avatars/mascot-2.png";
import mascot3 from "../assets/avatars/mascot-3.png";
import mascot4 from "../assets/avatars/mascot-4.png";
import mascot5 from "../assets/avatars/mascot-5.png";
import mascot6 from "../assets/avatars/mascot-6.png";
import mascot7 from "../assets/avatars/mascot-7.png";
import mascot8 from "../assets/avatars/mascot-8.png";
import mascot9 from "../assets/avatars/mascot-9.png";
import mascot10 from "../assets/avatars/mascot-10.png";
import mascot11 from "../assets/avatars/mascot-11.png";

import { avatarMascotIndex, avatarSeed, avatarTintIndex } from "../lib/avatar";

const MASCOTS = [
  mascot0,
  mascot1,
  mascot2,
  mascot3,
  mascot4,
  mascot5,
  mascot6,
  mascot7,
  mascot8,
  mascot9,
  mascot10,
  mascot11,
] as const;

// Muted tints, mirrored from the desktop default-avatar palette.
const TINTS = [
  "hsl(14 35% 90%)",
  "hsl(33 37% 90%)",
  "hsl(52 29% 90%)",
  "hsl(96 24% 90%)",
  "hsl(150 24% 90%)",
  "hsl(182 25% 90%)",
  "hsl(202 29% 90%)",
  "hsl(222 28% 90%)",
  "hsl(250 25% 90%)",
  "hsl(288 22% 90%)",
  "hsl(322 24% 90%)",
  "hsl(350 31% 90%)",
] as const;

export function MascotAvatar({
  id,
  name,
  size = 42,
  kind,
}: {
  /** Stable participant identity; falls back to name. */
  id?: string;
  name?: string;
  size?: number;
  /** "named" draws from the dressed cast (agent personas). */
  kind?: "named";
}): React.JSX.Element {
  const seed = avatarSeed(id, name);
  const mascot = MASCOTS[avatarMascotIndex(seed, kind)];
  const tint = TINTS[avatarTintIndex(seed)];
  return (
    <span
      className="mascot-avatar"
      style={{ width: size, height: size, background: tint }}
      aria-hidden
    >
      <img src={mascot} alt="" draggable={false} />
    </span>
  );
}
