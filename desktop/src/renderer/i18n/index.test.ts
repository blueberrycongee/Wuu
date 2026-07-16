import { describe, expect, it, vi } from "vitest";
import { resolveLocale, translate } from ".";
import type { TranslationKey } from "./resources/zh-CN";

describe("i18n", () => {
  it("resolves system Chinese locales to zh-CN and all others to en-US", () => {
    expect(resolveLocale("system", "zh-Hans-CN")).toBe("zh-CN");
    expect(resolveLocale("system", "en-GB")).toBe("en-US");
    expect(resolveLocale("system", "ja-JP")).toBe("en-US");
  });

  it("honors an explicit language preference", () => {
    expect(resolveLocale("zh-CN", "en-US")).toBe("zh-CN");
    expect(resolveLocale("en-US", "zh-CN")).toBe("en-US");
  });

  it("interpolates values and exposes missing keys in tests", () => {
    expect(translate("en-US", "settings.language")).toBe("Language");
    const warning = vi.spyOn(console, "warn").mockImplementation(() => undefined);
    expect(translate("en-US", "missing" as TranslationKey)).toBe("Content unavailable");
    expect(warning).toHaveBeenCalledWith("[i18n] Missing translation: en-US.missing");
    warning.mockRestore();
  });
});
