import { describe, expect, it } from "vitest";
import { AVATAR_HUES } from "./DefaultAvatar";
import { providerMascotHue } from "./WuuMascot";

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
