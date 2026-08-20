import { VERSION as BLOBATAR_VERSION, _layout } from "blobatar";
import { describe, expect, it } from "vitest";
import { AVATAR_HUES } from "./DefaultAvatar";
import { providerMascotHue } from "./WuuMascot";
import {
  WUU_MASCOT_ACTIVITY_PERSPECTIVES,
  WUU_MASCOT_NAME,
  WUU_MASCOT_TRAITS,
  type WuuMascotActivity,
} from "./wuu-mascot-spec";

describe("vendored mascot geometry", () => {
  const flat = _layout(WUU_MASCOT_NAME, { traits: WUU_MASCOT_TRAITS });
  const forActivity = (activity: WuuMascotActivity) =>
    _layout(WUU_MASCOT_NAME, {
      traits: WUU_MASCOT_TRAITS,
      perspective: WUU_MASCOT_ACTIVITY_PERSPECTIVES[activity],
    });

  it("keeps both Wuu eyes as crisp capsules at compact process-row sizes", () => {
    const read = forActivity("read");

    expect(BLOBATAR_VERSION).toBe("0.2.0-wuu.7");
    for (const eye of read.eyes) {
      expect(eye.rx / read.body.rx).toBeGreaterThan(0.055);
    }
    // The projection moves the pair but never touches the capsules: same
    // size, same lean, no warp — at these sizes a shrunk capsule reads as a
    // smaller eye, not as a turned surface.
    expect(read.eyes.map((e) => [e.rx, e.ry, e.rot])).toEqual(
      flat.eyes.map((e) => [e.rx, e.ry, e.rot]),
    );
  });

  it("carries a distinct gaze in every activity, and none of them warp the eyes", () => {
    const pairCenter = (l: typeof flat) => ({
      x: (l.eyes[0]!.cx + l.eyes[1]!.cx) / 2 - l.body.cx,
      y: (l.eyes[0]!.cy + l.eyes[1]!.cy) / 2 - l.body.cy,
    });
    const flatCenter = pairCenter(flat);

    for (const [activity, perspective] of Object.entries(WUU_MASCOT_ACTIVITY_PERSPECTIVES)) {
      const layout = forActivity(activity as WuuMascotActivity);
      // Capsules stay capsules: same size, same lean, no warp, whatever the gaze.
      expect(layout.eyes.map((e) => [e.rx, e.ry, e.rot]), activity).toEqual(
        flat.eyes.map((e) => [e.rx, e.ry, e.rot]),
      );
      if (!perspective) continue;
      const center = pairCenter(layout);
      // The pair moves the way the perspective names: yaw sideways, pitch
      // vertically (SVG y grows downward, so a positive pitch looks up).
      if (perspective.yaw > 0) expect(center.x, activity).toBeGreaterThan(flatCenter.x);
      if (perspective.yaw < 0) expect(center.x, activity).toBeLessThan(flatCenter.x);
      if (perspective.pitch > 0) expect(center.y, activity).toBeLessThan(flatCenter.y);
      if (perspective.pitch < 0) expect(center.y, activity).toBeGreaterThan(flatCenter.y);
    }
  });

  it("keeps the read pose on its long-established glance back at the conversation", () => {
    expect(WUU_MASCOT_ACTIVITY_PERSPECTIVES.read).toEqual({ yaw: -32, pitch: 16, strength: 1 });
    // And idle greets by looking down toward the composer rather than staring ahead.
    expect(WUU_MASCOT_ACTIVITY_PERSPECTIVES.idle?.pitch).toBeLessThan(0);
  });

  it("lifts the mascot's head from the composer when a draft exists", () => {
    const idle = forActivity("idle");
    const compose = forActivity("compose");
    const idleY = (idle.eyes[0]!.cy + idle.eyes[1]!.cy) / 2;
    const composeY = (compose.eyes[0]!.cy + compose.eyes[1]!.cy) / 2;
    expect(composeY).toBeLessThan(idleY);
  });
});

describe("providerMascotHue", () => {
  it("gives configured providers distinct colours until the palette is exhausted", () => {
    const providers = Array.from({ length: AVATAR_HUES.length + 1 }, (_, index) => `provider-${index + 1}`);
    const firstPalette = providers
      .slice(0, AVATAR_HUES.length)
      .map((provider) => providerMascotHue(provider, providers));

    expect(new Set(firstPalette).size).toBe(AVATAR_HUES.length);
    expect(providerMascotHue(providers.at(-1), providers)).toBe(firstPalette[0]);
  });

  it("normalizes provider names when assigning a configured colour", () => {
    const providers = ["OpenAI", "Anthropic"];

    expect(providerMascotHue(" openai ", providers)).toBe(providerMascotHue("OpenAI", providers));
    expect(providerMascotHue("ANTHROPIC", providers)).not.toBe(providerMascotHue("OpenAI", providers));
  });

  it("uses the next free colour for a provider missing from the catalog", () => {
    const providers = ["OpenAI", "Anthropic"];

    expect(providerMascotHue("custom", providers)).not.toBe(providerMascotHue("OpenAI", providers));
    expect(providerMascotHue("custom", providers)).not.toBe(providerMascotHue("Anthropic", providers));
  });

  it("keeps a deterministic fallback when the provider catalog is unavailable", () => {
    expect(providerMascotHue("custom-provider")).toBe(providerMascotHue("CUSTOM-PROVIDER"));
  });
});
