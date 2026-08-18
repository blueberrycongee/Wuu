// Participant avatar: an uploaded avatar_image data URL when present,
// otherwise the deterministic blobatar fallback (same FNV-1a hue
// assignment as the desktop, so identities match across ends).

import { Blobatar } from "blobatar/react";

import { AVATAR_HUES, avatarHueIndex, avatarSeed } from "../lib/avatar";

export function BlobatarAvatar({
  id,
  name,
  size = 42,
}: {
  /** Stable participant identity; falls back to name. */
  id?: string;
  name?: string;
  size?: number;
}): React.JSX.Element {
  const seed = avatarSeed(id, name);
  return (
    <span
      className="blobatar-avatar"
      style={{ width: size, height: size }}
      aria-hidden
    >
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
