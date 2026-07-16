export const zhCN = {
  "common.system": "跟随系统",
  "common.chinese": "简体中文",
  "common.english": "English",
  "settings.appearance": "外观",
  "settings.language": "语言",
  "settings.languageGroup": "界面语言",
  "settings.theme": "主题",
  "settings.themeGroup": "外观主题",
  "settings.themeLight": "亮色",
  "settings.themeDark": "暗色",
  "settings.messageFontSize": "消息流字号",
  "i18n.missing": "内容暂不可用",
} as const;

export type TranslationKey = keyof typeof zhCN;
