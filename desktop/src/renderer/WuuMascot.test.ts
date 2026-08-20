import { VERSION as BLOBATAR_VERSION, _layout } from "blobatar";
import { describe, expect, it } from "vitest";
import { AVATAR_HUES } from "./DefaultAvatar";
import { providerMascotHue } from "./WuuMascot";

describe("vendored mascot geometry", () => {
  it("keeps both Wuu eyes readable at compact process-row sizes", () => {
    const layout = _layout("wuu", {
      traits: { shape: 0.2, "body.ratio": 0.5 },
      perspective: { yaw: -32, pitch: 16, strength: 1 },
    });

    expect(BLOBATAR_VERSION).toBe("0.2.0-wuu.6");
    for (const eye of layout.eyes) {
      expect(eye.rx / layout.body.rx).toBeGreaterThan(0.085);
    }
    expect(layout.eyes[0].rx).toBeLessThan(layout.eyes[1].rx);
    expect(Math.abs(layout.eyes[1].surfaceRot ?? 0)).toBeGreaterThan(
      Math.abs(layout.eyes[0].surfaceRot ?? 0),
    );
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
