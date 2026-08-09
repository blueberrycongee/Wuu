import { useEffect, useState } from "react";
import type { ExtensionInventoryRecord, ThemePreference } from "../shared/protocol";
import {
  applyExtensionTheme,
  applyThemePreference,
  availableExtensionThemes,
  selectedExtensionThemeKey,
} from "./Theme";
import { useI18n } from "./i18n";

function ThemePreviewWindow({ theme }: { theme: ThemePreference }): JSX.Element {
  return (
    <span className={`settings-theme-preview-window settings-theme-preview-window-${theme}`}>
      <span className="settings-theme-preview-sidebar" />
      <span className="settings-theme-preview-body">
        <span className="settings-theme-preview-line settings-theme-preview-line-title" />
        <span className="settings-theme-preview-line" />
        <span className="settings-theme-preview-line settings-theme-preview-line-short" />
      </span>
    </span>
  );
}

/**
 * 外观 row body: visual radio cards with a tiny window preview per option
 * (system splits light/dark). Reads and persists through window.wuu
 * directly, and applies the choice to <html data-theme> immediately so
 * the user sees the switch without a save step.
 */
export function ThemePreferenceControl({
  extensionInventory,
}: {
  extensionInventory?: readonly ExtensionInventoryRecord[];
} = {}): JSX.Element {
  const { t } = useI18n();
  const themeOptions: Array<{ value: ThemePreference; label: string }> = [
    { value: "system", label: t("common.system") },
    { value: "light", label: t("settings.themeLight") },
    { value: "dark", label: t("settings.themeDark") },
  ];
  const [preference, setPreference] = useState<ThemePreference>(
    () => window.wuu?.initialThemePreference ?? "system",
  );
  const [extensionThemeKey, setExtensionThemeKey] = useState(selectedExtensionThemeKey);
  const extensionThemes = availableExtensionThemes(extensionInventory);

  useEffect(() => {
    let cancelled = false;
    void window.wuu
      ?.getThemePreference?.()
      .then((stored) => {
        if (!cancelled && stored) {
          setPreference(stored);
        }
      })
      .catch(() => {
        // Keep the preload-provided initial value.
      });
    return () => {
      cancelled = true;
    };
  }, []);

  // Reflect switches made in other windows: the preference is app-global,
  // so the segmented control must follow the main-process broadcast too.
  useEffect(
    () =>
      window.wuu?.onThemePreferenceChange?.((theme) => {
        setPreference(theme);
      }),
    [],
  );

  function choose(next: ThemePreference): void {
    setExtensionThemeKey("");
    setPreference(next);
    applyThemePreference(next);
    void window.wuu?.setThemePreference?.(next).catch(() => {
      // Persistence failure leaves the applied theme for this window;
      // the next launch falls back to the stored preference.
    });
  }

  function chooseExtensionTheme(key: string): void {
    const theme = extensionThemes.find((candidate) => candidate.key === key);
    if (!theme) {
      return;
    }
    setExtensionThemeKey(key);
    applyExtensionTheme(theme);
  }

  return (
    <div className="settings-theme-options" role="radiogroup" aria-label={t("settings.themeGroup")}>
      {themeOptions.map((option) => (
        <button
          key={option.value}
          type="button"
          role="radio"
          aria-checked={!extensionThemeKey && preference === option.value}
          className={`settings-theme-card${!extensionThemeKey && preference === option.value ? " active" : ""}`}
          data-testid={`settings-theme-${option.value}`}
          onClick={() => choose(option.value)}
        >
          <span className={`settings-theme-preview settings-theme-preview-${option.value}`} aria-hidden="true">
            <ThemePreviewWindow theme={option.value} />
          </span>
          <span className="settings-theme-card-label">{option.label}</span>
        </button>
      ))}
      {extensionThemes.map((theme) => (
        <button
          key={theme.key}
          type="button"
          role="radio"
          aria-checked={extensionThemeKey === theme.key}
          className={`settings-theme-card${extensionThemeKey === theme.key ? " active" : ""}`}
          data-testid={`settings-theme-extension-${theme.key}`}
          onClick={() => chooseExtensionTheme(theme.key)}
        >
          <span
            className={`settings-theme-preview settings-theme-preview-${theme.base}`}
            aria-hidden="true"
          >
            <ThemePreviewWindow theme={theme.base} />
          </span>
          <span className="settings-theme-card-label">{theme.name}</span>
          <span className="settings-theme-card-label settings-theme-card-source">{theme.pluginName}</span>
        </button>
      ))}
    </div>
  );
}
