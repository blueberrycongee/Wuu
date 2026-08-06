import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const RENDERER_ROOT = resolve(process.cwd(), "src/renderer");

/**
 * Published semantic anchors for plugin CSS snippets and supplemental
 * styles. These `data-wuu-component` names are a compatibility contract:
 * renaming or removing one breaks third-party themes, so this inventory
 * fails the build when an anchor silently disappears from its owner.
 *
 * Overlay anchors (dialog, menu, popover, tooltip, notice, …) are owned
 * by the layer host and covered by ProductionUILayers.test.ts.
 */
const SEMANTIC_ANCHOR_OWNERS = Object.freeze({
  "app-shell": "App.tsx",
  "automations-catalog": "AutomationsCatalog.tsx",
  "composer": "ComposerView.tsx",
  "composer-input": "ComposerView.tsx",
  "composer-send": "ComposerView.tsx",
  "conversation-pane": "App.tsx",
  "launch-view": "LoadingViews.tsx",
  "message": "ThreadItemView.tsx",
  "settings-shell": "SettingsView.tsx",
  "sidebar": "AppSidebar.tsx",
  "skills-catalog": "SkillsCatalog.tsx",
  "turn": "TurnView.tsx",
  "workspace-panel": "WorkspacePanels.tsx",
} as const);

describe("production semantic anchors", () => {
  it("keeps every published data-wuu-component anchor in its owner", () => {
    for (const [anchor, owner] of Object.entries(SEMANTIC_ANCHOR_OWNERS)) {
      const source = readFileSync(resolve(RENDERER_ROOT, owner), "utf8");
      expect(source, `${anchor} anchor in ${owner}`).toContain(
        `data-wuu-component="${anchor}"`,
      );
    }
  });

  it("distinguishes user and agent message variants", () => {
    const source = readFileSync(
      resolve(RENDERER_ROOT, "ThreadItemView.tsx"),
      "utf8",
    );
    expect(source).toContain('data-wuu-variant="user"');
    expect(source).toContain('data-wuu-variant="agent"');
  });
});
