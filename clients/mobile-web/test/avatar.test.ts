// Avatar assignment must be identical to desktop DefaultAvatar.tsx — the
// same participant gets the same hue bucket (and therefore the same
// blobatar, since blobatar itself is deterministic per name) on both ends.
// The pinned values below were computed from the desktop algorithm; if this
// test ever needs regenerating, the two implementations have diverged and
// that is the bug.

import { describe, expect, it } from "vitest";

import { AVATAR_HUES, avatarHueIndex, avatarSeed, fnv1a } from "../src/lib/avatar";

describe("fnv1a", () => {
  it("matches the desktop hash for pinned seeds", () => {
    expect(fnv1a("")).toBe(2166136261);
    expect(fnv1a("andy")).toBe(3183330573);
    expect(fnv1a("participant-42")).toBe(3254981055);
    expect(fnv1a("墨白")).toBe(3064423518);
    expect(fnv1a("shitou")).toBe(3280474227);
  });
});

describe("avatar assignment", () => {
  it("hue spans all 12 muted hues", () => {
    expect(AVATAR_HUES).toHaveLength(12);
    expect(AVATAR_HUES[0]).toBe(14);
    expect(AVATAR_HUES[11]).toBe(350);
    expect(avatarHueIndex("andy")).toBe(9);
    expect(avatarHueIndex("participant-42")).toBe(3);
    expect(avatarHueIndex("墨白")).toBe(6);
  });

  it("is deterministic", () => {
    for (const seed of ["a", "b", "长长的中文名字", "participant-1"]) {
      expect(avatarHueIndex(seed)).toBe(avatarHueIndex(seed));
    }
  });

  it("seeds from id first, then display name", () => {
    expect(avatarSeed("id-1", "Name")).toBe("id-1");
    expect(avatarSeed("  ", "Name")).toBe("Name");
    expect(avatarSeed(undefined, " Name ")).toBe("Name");
    expect(avatarSeed(undefined, undefined)).toBe("");
  });
});
