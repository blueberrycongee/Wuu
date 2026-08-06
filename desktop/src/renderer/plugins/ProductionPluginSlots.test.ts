import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

import { PLUGIN_SLOT_IDS } from "./PluginHost";

const PRODUCTION_SLOT_OWNERS = Object.freeze({
  "sidebar.primary": "../AppSidebar.tsx",
  "sidebar.footer": "../AppSidebar.tsx",
  "workspace.header": "../WorkspacePanels.tsx",
  "conversation.header": "../ConversationShellRenderers.tsx",
  "conversation.message.before": "../ThreadItemView.tsx",
  "conversation.message.after": "../ThreadItemView.tsx",
  "composer.above": "../ComposerView.tsx",
  "composer.toolbar": "../ComposerView.tsx",
  "settings.plugin": "../SettingsView.tsx",
} as const);

describe("production plugin slot mounts", () => {
  it("mounts every fixed slot in its real owner without using App or legacy surfaces", () => {
    expect(Object.keys(PRODUCTION_SLOT_OWNERS)).toEqual([...PLUGIN_SLOT_IDS]);

    for (const [slotId, owner] of Object.entries(PRODUCTION_SLOT_OWNERS)) {
      const source = readFileSync(new URL(owner, import.meta.url), "utf8");
      expect(source, `${slotId} production owner`).toContain(`id="${slotId}"`);
      expect(owner).not.toBe("../App.tsx");
    }
  });

  it("freezes sanitized context at every production owner", () => {
    for (const owner of new Set(Object.values(PRODUCTION_SLOT_OWNERS))) {
      const source = readFileSync(new URL(owner, import.meta.url), "utf8");
      expect(source, owner).toContain("Object.freeze({");
    }
  });
});
