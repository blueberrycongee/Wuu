import type { TranslationKey } from "./zh-CN";

export const enUS = {
  "common.system": "Use system language",
  "common.chinese": "简体中文",
  "common.english": "English",
  "settings.appearance": "Appearance",
  "settings.language": "Language",
  "settings.languageGroup": "Interface language",
  "settings.theme": "Theme",
  "settings.themeGroup": "Appearance theme",
  "settings.themeLight": "Light",
  "settings.themeDark": "Dark",
  "settings.messageFontSize": "Message font size",
  "i18n.missing": "Content unavailable",
} as const satisfies Record<TranslationKey, string>;
