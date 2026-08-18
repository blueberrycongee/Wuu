import { Blobatar } from "blobatar/react";
import type { JSX } from "react";

/*
 * Default participant avatars.
 *
 * When a participant has no uploaded avatar_image (or the image was too
 * large to embed in a summary), surfaces fall back to a blobatar generated
 * from a stable seed (the participant id): an organic blob with eyes on a
 * matching tinted plate. Identity comes from shape + hue together, and the
 * shape is unique per seed with no fixed cast, so rosters of any size stay
 * collision-free.
 *
 * The hue is pinned to the same 12 muted hues the old mascot tints used, so
 * a participant keeps their color family across redesigns and the palette
 * stays inside the paper-ink system. Assignment is deterministic from the
 * seed (FNV-1a, the same hash the old cast used): the same participant
 * always gets the same hue, no new backend field required.
 */

// The 12 muted hues, preserved from the previous default-avatar tint
// palette (--avatar-N in styles/default-avatar.css).
export const AVATAR_HUES = [14, 33, 52, 96, 150, 182, 202, 222, 250, 288, 322, 350] as const;

export const DEFAULT_AVATAR_COUNT = AVATAR_HUES.length;

// FNV-1a over the seed. Stable across sessions and machines for a
// given id.
function fnv1a(seed: string): number {
  let h = 0x811c9dc5;
  for (let i = 0; i < seed.length; i++) {
    h ^= seed.charCodeAt(i);
    h = Math.imul(h, 0x01000193);
  }
  return h >>> 0;
}

/** Hue bucket in [0, DEFAULT_AVATAR_COUNT) for the seed. */
export function avatarHueIndex(seed: string): number {
  return fnv1a(seed) % DEFAULT_AVATAR_COUNT;
}

/**
 * Renders the default blobatar avatar for a participant, sized to fill
 * its container (the caller's circular avatar cell clips it). `seed`
 * should be a stable identity string — the participant id where
 * available, otherwise the display name. The blob shape, eyes, and
 * palette all derive deterministically from the seed.
 */
export function DefaultAvatarMark({ seed }: { seed: string }): JSX.Element {
  return (
    <span className="default-avatar" aria-hidden="true">
      <Blobatar
        name={seed}
        hue={AVATAR_HUES[avatarHueIndex(seed)]}
        background="circle"
        alt=""
        draggable={false}
      />
    </span>
  );
}
