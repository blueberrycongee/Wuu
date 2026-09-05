import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { ONBOARDING_PLUGIN_ORDER, OnboardingMascotStage } from "./OnboardingMascotStage";

describe("onboarding companion", () => {
  let container: HTMLDivElement;
  let root: Root;
  beforeEach(() => {
    container = document.createElement("div");
    document.body.append(container);
    root = createRoot(container);
  });
  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
  });

  async function render(ids: readonly string[], preview = ""): Promise<void> {
    await act(async () => root.render(<OnboardingMascotStage pluginIDs={ids} previewedPluginID={preview} />));
  }

  it("keeps the same character while arbitrary capability combinations are equipped and removed", async () => {
    await render([]);
    const face = container.querySelector("[data-wuu-mascot-activity]");
    expect(face).not.toBeNull();
    // Exhausting seven binary choices exercises shared equipment ownership:
    // removing memory, for example, must not also remove dream's arrangement.
    for (let mask = 0; mask < 1 << ONBOARDING_PLUGIN_ORDER.length; mask++) {
      const ids = ONBOARDING_PLUGIN_ORDER.filter((_, index) => mask & (1 << index));
      await render(ids);
      const rendered = [...container.querySelectorAll("[data-onboarding-capability]")]
        .map((node) => node.getAttribute("data-onboarding-capability"));
      expect(rendered.sort()).toEqual(ids.filter((id) => id !== "subagent").sort());
      expect(container.querySelectorAll("[data-onboarding-companion]:not([hidden])"))
        .toHaveLength(ids.includes("subagent") ? 3 : 1);
      expect(container.querySelector("[data-wuu-mascot-activity]")).toBe(face);
    }
  });

  it("does not duplicate equipment when previewing an enabled plugin or react to an unavailable preview", async () => {
    await render(["memory", "memory", "unknown-plugin"], "memory");
    expect(container.querySelectorAll("[data-onboarding-capability]")).toHaveLength(1);
    const notebook = container.querySelector("[data-onboarding-capability=memory]");
    await render(["memory"], "ask-user");
    expect(container.querySelector("[data-onboarding-capability=memory]")).toBe(notebook);
    expect(container.querySelector("[data-onboarding-preview]")).toBeNull();
    expect(container.querySelectorAll("button, input, [tabindex='0']")).toHaveLength(0);
    expect(container.querySelector("[data-testid=onboarding-mascot-stage]")?.getAttribute("aria-hidden")).toBe("true");
  });
});
