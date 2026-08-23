import { afterEach, describe, expect, it, vi } from "vitest";

afterEach(() => {
  vi.unstubAllEnvs();
  vi.resetModules();
});

describe("frontend feature flags", () => {
  it("supports explicit remote control opt-in builds", async () => {
    vi.stubEnv("VITE_ENABLE_REMOTE_CONTROL", "true");
    const { ENABLE_REMOTE_CONTROL } = await import("./FeatureFlags");

    expect(ENABLE_REMOTE_CONTROL).toBe(true);
  });

  it("enables collaboration by default and supports an emergency build opt-out", async () => {
    vi.stubEnv("VITE_ENABLE_GROUP_CHAT", "");
    let featureFlags = await import("./FeatureFlags");
    expect(featureFlags.ENABLE_GROUP_CHAT).toBe(true);

    vi.resetModules();
    vi.stubEnv("VITE_ENABLE_GROUP_CHAT", "false");
    featureFlags = await import("./FeatureFlags");
    expect(featureFlags.ENABLE_GROUP_CHAT).toBe(false);

    vi.resetModules();
    vi.stubEnv("VITE_ENABLE_GROUP_CHAT", "true");
    featureFlags = await import("./FeatureFlags");
    expect(featureFlags.ENABLE_GROUP_CHAT).toBe(true);
  });

  it("keeps voice input and BYOK polish disabled unless explicitly enabled", async () => {
    vi.stubEnv("VITE_ENABLE_VOICE_INPUT", "false");
    let featureFlags = await import("./FeatureFlags");
    expect(featureFlags.ENABLE_VOICE_INPUT).toBe(false);

    vi.resetModules();
    vi.stubEnv("VITE_ENABLE_VOICE_INPUT", "true");
    featureFlags = await import("./FeatureFlags");
    expect(featureFlags.ENABLE_VOICE_INPUT).toBe(true);
  });

  it("keeps the embedded browser disabled unless explicitly enabled in development", async () => {
    vi.stubEnv("VITE_ENABLE_BROWSER", "false");
    let featureFlags = await import("./FeatureFlags");
    expect(featureFlags.ENABLE_EMBEDDED_BROWSER).toBe(false);

    vi.resetModules();
    vi.stubEnv("VITE_ENABLE_BROWSER", "true");
    featureFlags = await import("./FeatureFlags");
    expect(featureFlags.ENABLE_EMBEDDED_BROWSER).toBe(true);
  });

  it("keeps management assistants disabled unless explicitly enabled", async () => {
    vi.stubEnv("VITE_ENABLE_MANAGEMENT_ASSISTANT", "false");
    let featureFlags = await import("./FeatureFlags");
    expect(featureFlags.ENABLE_MANAGEMENT_ASSISTANT).toBe(false);

    vi.resetModules();
    vi.stubEnv("VITE_ENABLE_MANAGEMENT_ASSISTANT", "true");
    featureFlags = await import("./FeatureFlags");
    expect(featureFlags.ENABLE_MANAGEMENT_ASSISTANT).toBe(true);
  });
});
