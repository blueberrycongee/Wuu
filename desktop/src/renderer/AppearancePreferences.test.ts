import { afterEach, describe, expect, it, vi } from "vitest";
import { appearanceDefaults, codeEditorTypography, normalizeAppearance, observeAppearance, readAppearance, saveAppearance, startAppearanceSync } from "./AppearancePreferences";
import { applyMessageFlowFontSize } from "./MessageFlowFontSizeSection";

afterEach(() => {
  localStorage.clear();
  document.documentElement.removeAttribute("style");
  vi.restoreAllMocks();
});

describe("appearance preferences", () => {
  it("recovers from corrupt persisted values and bounds independent sizes", () => {
    localStorage.setItem("wuu.appearance.v1", "broken JSON");
    expect(readAppearance()).toEqual(appearanceDefaults);
    expect(normalizeAppearance({ uiSize: 99, uiFont: 42 })).toMatchObject({
      uiSize: 16, uiFont: "",
    });
  });

  it("updates current-window consumers, persists code typography and follows another window", () => {
    const stop = startAppearanceSync();
    const changed = vi.fn();
    const unsubscribe = observeAppearance(changed);
    applyMessageFlowFontSize(19);
    changed.mockClear();
    saveAppearance({ ...appearanceDefaults, codeFont: "Example Mono" });
    expect(changed).toHaveBeenCalledTimes(1);
    expect(readAppearance().codeFont).toBe("Example Mono");
    expect(codeEditorTypography()).toMatchObject({ fontSize: 19, fontFamily: expect.stringContaining('"Example Mono"') });
    localStorage.setItem("wuu.appearance.v1", JSON.stringify({ ...appearanceDefaults, uiSize: 16 }));
    window.dispatchEvent(new StorageEvent("storage", { key: "wuu.appearance.v1" }));
    expect(document.documentElement.style.getPropertyValue("--appearance-ui-size")).toBe("16px");
    expect(changed).toHaveBeenCalledTimes(2);
    stop();
    unsubscribe();
    window.dispatchEvent(new StorageEvent("storage", { key: "wuu.appearance.v1" }));
    expect(changed).toHaveBeenCalledTimes(2);
  });

  it("does not publish or apply an unsaved preference when persistence fails", () => {
    const changed = vi.fn();
    const unsubscribe = observeAppearance(changed);
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("quota"); });
    expect(() => saveAppearance({ ...appearanceDefaults, uiSize: 16 })).toThrow("quota");
    expect(changed).not.toHaveBeenCalled();
    expect(document.documentElement.style.getPropertyValue("--appearance-ui-size")).toBe("");
    unsubscribe();
  });
});
