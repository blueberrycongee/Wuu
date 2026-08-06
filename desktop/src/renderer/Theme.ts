import type {
  ExtensionInventoryRecord,
  ExtensionThemeDescriptor,
  ThemePreference,
} from "../shared/protocol";
import {
  canonicalThemeTokenName,
  isPublicSyntaxTokenName,
  isPublicThemeTokenName,
} from "../shared/themeContract.generated";

/**
 * Renderer-side theme controller.
 *
 * The single source of visual truth is the `data-theme` attribute on
 * <html>: every color in the stylesheets resolves through CSS custom
 * properties, and the theme blocks in base.css key off
 * `:root[data-theme="dark"]`. This module resolves a persisted
 * preference ("system" | "light" | "dark") to a concrete attribute
 * value and keeps it in sync with the OS when the preference is
 * "system".
 *
 * The preload script stamps the attribute once before first paint (so
 * boot doesn't flash light); `applyThemePreference` is the runtime
 * authority from then on.
 */

const DARK_SCHEME_QUERY = "(prefers-color-scheme: dark)";

let systemListenerCleanup: (() => void) | undefined;
const EXTENSION_THEME_KEY = "wuu.extension-theme";
const appliedExtensionTokens = new Set<string>();

export type AvailableExtensionTheme = ExtensionThemeDescriptor & {
  key: string;
  pluginId: string;
  pluginName: string;
};

export type AppliedTheme = "light" | "dark";

export function resolveThemePreference(
  preference: ThemePreference,
): "light" | "dark" {
  if (preference === "system") {
    return typeof window !== "undefined" &&
      window.matchMedia?.(DARK_SCHEME_QUERY).matches
      ? "dark"
      : "light";
  }
  return preference;
}

export function appliedTheme(): string | undefined {
  return document.documentElement.dataset.theme;
}

export function currentAppliedTheme(): AppliedTheme {
  return appliedTheme() === "dark" ? "dark" : "light";
}

export function observeAppliedTheme(
  onChange: (theme: AppliedTheme) => void,
): () => void {
  let current = currentAppliedTheme();
  const observer = new MutationObserver(() => {
    const next = currentAppliedTheme();
    if (next === current) {
      return;
    }
    current = next;
    onChange(next);
  });
  observer.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ["data-theme"],
  });
  return () => observer.disconnect();
}

/**
 * Stamp the resolved theme on <html> and, for "system", follow OS
 * changes live until a different preference is applied.
 */
export function applyThemePreference(preference: ThemePreference): void {
  clearExtensionTheme();
  systemListenerCleanup?.();
  systemListenerCleanup = undefined;

  document.documentElement.dataset.theme = resolveThemePreference(preference);

  if (preference !== "system" || typeof window === "undefined") {
    return;
  }
  const query = window.matchMedia?.(DARK_SCHEME_QUERY);
  if (!query?.addEventListener) {
    return;
  }
  const onChange = (event: MediaQueryListEvent): void => {
    document.documentElement.dataset.theme = event.matches ? "dark" : "light";
  };
  query.addEventListener("change", onChange);
  systemListenerCleanup = () => query.removeEventListener("change", onChange);
}

export function availableExtensionThemes(
  inventory: readonly ExtensionInventoryRecord[] | undefined,
): AvailableExtensionTheme[] {
  return (inventory ?? []).flatMap((plugin) => {
    if (
      plugin.kind !== "plugin" ||
      plugin.enabled === false ||
      (plugin.state !== "granted" && plugin.state !== "active")
    ) {
      return [];
    }
    return (plugin.contributions?.themes ?? []).map((theme) => ({
      ...theme,
      key: `${plugin.id}:${theme.id}`,
      pluginId: plugin.id,
      pluginName: plugin.name,
    }));
  });
}

export function selectedExtensionThemeKey(): string {
  return window.localStorage?.getItem(EXTENSION_THEME_KEY) ?? "";
}

export function applyExtensionTheme(theme: AvailableExtensionTheme): void {
  systemListenerCleanup?.();
  systemListenerCleanup = undefined;
  clearExtensionThemeTokens();
  document.documentElement.dataset.theme = theme.base;
  const explicitTokens = new Set(Object.keys(theme.tokens));
  for (const [token, value] of Object.entries(theme.tokens)) {
    if (!isPublicThemeTokenName(token)) continue;
    document.documentElement.style.setProperty(token, value);
    appliedExtensionTokens.add(token);
    const canonical = canonicalThemeTokenName(token);
    if (canonical !== token && !explicitTokens.has(canonical)) {
      document.documentElement.style.setProperty(canonical, value);
      appliedExtensionTokens.add(canonical);
    }
  }
  for (const [token, value] of Object.entries(theme.syntax ?? {})) {
    if (!isPublicSyntaxTokenName(token)) continue;
    document.documentElement.style.setProperty(token, value);
    appliedExtensionTokens.add(token);
  }
  window.localStorage?.setItem(EXTENSION_THEME_KEY, theme.key);
}

export function clearExtensionTheme(): void {
  clearExtensionThemeTokens();
  window.localStorage?.removeItem(EXTENSION_THEME_KEY);
}

export function syncExtensionTheme(
  inventory: readonly ExtensionInventoryRecord[] | undefined,
): void {
  const selected = selectedExtensionThemeKey();
  if (!selected) {
    clearExtensionThemeTokens();
    return;
  }
  const theme = availableExtensionThemes(inventory).find((candidate) => candidate.key === selected);
  if (theme) {
    applyExtensionTheme(theme);
    return;
  }
  clearExtensionTheme();
}

function clearExtensionThemeTokens(): void {
  for (const token of appliedExtensionTokens) {
    document.documentElement.style.removeProperty(token);
  }
  appliedExtensionTokens.clear();
}

/**
 * Follow preference changes made in OTHER windows. The main process owns
 * the app-global preference and broadcasts every change; each window
 * re-applies it locally (the initiating window already applied it — the
 * re-apply is idempotent). Returns a disposer; the boot subscription is
 * window-lifetime, so it only matters for tests.
 */
export function startThemePreferenceSync(): () => void {
  return (
    window.wuu?.onThemePreferenceChange?.((theme) => {
      applyThemePreference(theme);
    }) ?? (() => {})
  );
}
