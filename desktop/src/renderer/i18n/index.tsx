import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from "react";
import type { AppLocale, LanguagePreference } from "../../shared/protocol";
import { enUS } from "./resources/en-US";
import { zhCN, type TranslationKey } from "./resources/zh-CN";

const resources: Record<AppLocale, Record<TranslationKey, string>> = {
  "zh-CN": zhCN,
  "en-US": enUS,
};

export function resolveLocale(
  preference: LanguagePreference,
  systemLocale: string = navigator.language,
): AppLocale {
  if (preference !== "system") return preference;
  return systemLocale.toLowerCase().startsWith("zh") ? "zh-CN" : "en-US";
}

export function translate(
  locale: AppLocale,
  key: TranslationKey,
  values: Record<string, string | number> = {},
): string {
  const template = resources[locale][key] || resources["en-US"][key] || resources["zh-CN"][key];
  if (!template) {
    if (import.meta.env.DEV || import.meta.env.MODE === "test") {
      console.warn(`[i18n] Missing translation: ${locale}.${key}`);
    }
    return resources[locale]["i18n.missing"];
  }
  return template.replace(/\{(\w+)\}/g, (_, name: string) => String(values[name] ?? `{${name}}`));
}

type I18nContextValue = {
  locale: AppLocale;
  preference: LanguagePreference;
  setPreference: (preference: LanguagePreference) => void;
  t: (key: TranslationKey, values?: Record<string, string | number>) => string;
  formatNumber: (value: number, options?: Intl.NumberFormatOptions) => string;
  formatDate: (value: Date | number | string, options?: Intl.DateTimeFormatOptions) => string;
  formatRelativeTime: (value: number, unit: Intl.RelativeTimeFormatUnit) => string;
};

function defaultContextValue(): I18nContextValue {
  const preference = window.wuu?.initialLanguagePreference ?? "system";
  const locale = resolveLocale(preference, window.wuu?.initialSystemLocale);
  return {
    locale,
    preference,
    setPreference: () => undefined,
    t: (key, values) => translate(locale, key, values),
    formatNumber: (number, options) => new Intl.NumberFormat(locale, options).format(number),
    formatDate: (input, options) => new Intl.DateTimeFormat(locale, options).format(new Date(input)),
    formatRelativeTime: (input, unit) => new Intl.RelativeTimeFormat(locale, { numeric: "auto" }).format(input, unit),
  };
}

const I18nContext = createContext<I18nContextValue | null>(null);

export function I18nProvider({ children }: { children: ReactNode }): JSX.Element {
  const [preference, setPreferenceState] = useState<LanguagePreference>(
    () => window.wuu?.initialLanguagePreference ?? "system",
  );
  const locale = resolveLocale(preference, window.wuu?.initialSystemLocale);
  const setPreference = useCallback((next: LanguagePreference) => {
    setPreferenceState(next);
    document.documentElement.lang = resolveLocale(next, window.wuu?.initialSystemLocale);
    void window.wuu?.setLanguagePreference(next).catch(() => undefined);
  }, []);
  const value = useMemo<I18nContextValue>(() => ({
    locale,
    preference,
    setPreference,
    t: (key, values) => translate(locale, key, values),
    formatNumber: (number, options) => new Intl.NumberFormat(locale, options).format(number),
    formatDate: (input, options) => new Intl.DateTimeFormat(locale, options).format(new Date(input)),
    formatRelativeTime: (input, unit) => new Intl.RelativeTimeFormat(locale, { numeric: "auto" }).format(input, unit),
  }), [locale, preference, setPreference]);
  document.documentElement.lang = locale;
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18nContextValue {
  const context = useContext(I18nContext);
  return context ?? defaultContextValue();
}
