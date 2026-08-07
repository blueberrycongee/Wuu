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
  "channel-view": "ChannelView.tsx",
  "composer": "ComposerView.tsx",
  "composer-frame": "ComposerView.tsx",
  "composer-input": "ComposerView.tsx",
  "composer-pending": "ComposerInputSections.tsx",
  "composer-send": "ComposerView.tsx",
  "composer-toolbar": "ComposerView.tsx",
  "conversation-pane": "App.tsx",
  "empty-session": "LoadingViews.tsx",
  "environment-panel": "EnvironmentPanel.tsx",
  "launch-view": "LoadingViews.tsx",
  "message": "ThreadItemView.tsx",
  "settings-shell": "SettingsView.tsx",
  "session-tab": "SessionTabs.tsx",
  "session-tab-close": "SessionTabs.tsx",
  "session-tab-main": "SessionTabs.tsx",
  "sidebar": "AppSidebar.tsx",
  "sidebar-toggle": "App.tsx",
  "side-thread": "SideThreadPanel.tsx",
  "skills-catalog": "SkillsCatalog.tsx",
  "turn": "TurnView.tsx",
  "workspace-browser": "WorkspaceBrowserPanel.tsx",
  "workspace-browser-address": "WorkspaceBrowserPanel.tsx",
  "workspace-browser-content": "WorkspaceBrowserPanel.tsx",
  "workspace-browser-statusbar": "WorkspaceBrowserPanel.tsx",
  "workspace-browser-toolbar": "WorkspaceBrowserPanel.tsx",
  "workspace-document-turn": "WorkspaceDocumentTurnDock.tsx",
  "workspace-empty-icon": "WorkspaceFiles.tsx",
  "workspace-empty-state": "WorkspaceFiles.tsx",
  "workspace-file-content": "WorkspacePanels.tsx",
  "workspace-file-tree": "WorkspacePanels.tsx",
  "workspace-files": "WorkspacePanels.tsx",
  "workspace-panel": "WorkspacePanels.tsx",
  "workspace-panel-header": "WorkspacePanels.tsx",
  "workspace-pdf-preview": "WorkspacePdfPreview.tsx",
  "workspace-review": "WorkspaceReviewPanels.tsx",
  "workspace-review-content": "WorkspaceReviewPanels.tsx",
  "workspace-review-content-header": "WorkspaceReviewPanels.tsx",
  "workspace-review-item": "WorkspaceReviewPanels.tsx",
  "workspace-review-navigation": "WorkspaceReviewPanels.tsx",
  "workspace-review-search": "WorkspaceReviewPanels.tsx",
  "workspace-terminal": "WorkspaceTerminalPanel.tsx",
  "workspace-terminal-content": "WorkspaceTerminalPanel.tsx",
  "workspace-terminal-item": "WorkspaceTerminalPanel.tsx",
  "workspace-terminal-layout": "WorkspaceTerminalPanel.tsx",
  "workspace-terminal-navigation": "WorkspaceTerminalPanel.tsx",
  "workspace-terminal-screen": "WorkspaceTerminalPanel.tsx",
  "workspace-tool": "WorkspacePanels.tsx",
  "workspace-tool-picker": "WorkspacePanels.tsx",
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

  it("keeps the environment panel anchor on its default and file-preview variants", () => {
    const source = readFileSync(
      resolve(RENDERER_ROOT, "EnvironmentPanel.tsx"),
      "utf8",
    );
    expect(source.match(/data-wuu-component="environment-panel"/g)).toHaveLength(2);
  });

  it("publishes docked, collapsed, and drawer sidebar presentation modes", () => {
    const source = readFileSync(resolve(RENDERER_ROOT, "App.tsx"), "utf8");
    expect(source).toContain("data-wuu-sidebar-mode=");
    expect(source).toContain('sidebarDrawerVisible ? "drawer"');
    expect(source).toContain('sidebarDrawerMode ? "collapsed" : "docked"');
  });

  it("publishes the sidebar toggle anchor in every app and settings titlebar", () => {
    const appSource = readFileSync(resolve(RENDERER_ROOT, "App.tsx"), "utf8");
    const settingsSource = readFileSync(resolve(RENDERER_ROOT, "SettingsView.tsx"), "utf8");
    expect(appSource.match(/data-wuu-component="sidebar-toggle"/g)).toHaveLength(3);
    expect(settingsSource.match(/data-wuu-component="sidebar-toggle"/g)).toHaveLength(1);
  });

  it("publishes the active workspace panel view", () => {
    const source = readFileSync(resolve(RENDERER_ROOT, "WorkspacePanels.tsx"), "utf8");
    expect(source).toContain('data-wuu-view={activeTab?.kind ?? "picker"}');
  });

  it("bridges public workspace file-tree variables into its shadow DOM", () => {
    const source = readFileSync(resolve(RENDERER_ROOT, "WorkspaceFiles.tsx"), "utf8");
    for (const variable of [
      "--wuu-workspace-file-tree-color",
      "--wuu-workspace-file-tree-background",
      "--wuu-workspace-file-tree-selected-background",
      "--wuu-workspace-file-tree-font-family",
      "--wuu-workspace-file-tree-search-background",
      "--wuu-workspace-file-tree-search-border",
      "--wuu-workspace-file-tree-search-radius",
    ]) {
      expect(source, `${variable} shadow DOM bridge`).toContain(variable);
    }
  });

  it("bridges public workspace terminal variables into xterm", () => {
    const source = readFileSync(resolve(RENDERER_ROOT, "WorkspaceTerminalPanel.tsx"), "utf8");
    for (const variable of [
      "--wuu-workspace-terminal-background",
      "--wuu-workspace-terminal-foreground",
      "--wuu-workspace-terminal-cursor",
      "--wuu-workspace-terminal-selection",
      "--wuu-workspace-terminal-font-family",
    ]) {
      expect(source, `${variable} xterm bridge`).toContain(variable);
    }
  });

  it("wraps every plugin contribution in the public coordinate boundary", () => {
    // Multi-plugin coexistence depends on this wrapper: it is what lets a
    // theme plugin's CSS snippets address a capability plugin's UI (by
    // plugin, slot, or surface id) without private class names.
    const slot = readFileSync(resolve(RENDERER_ROOT, "plugins/PluginSlot.tsx"), "utf8");
    expect(slot).toContain('data-wuu-component="plugin-contribution"');
    expect(slot).toContain('data-wuu-plugin={contribution.pluginId}');
    expect(slot).toContain('data-wuu-slot={id}');
    const surface = readFileSync(
      resolve(RENDERER_ROOT, "plugins/PluginSurface.tsx"),
      "utf8",
    );
    expect(surface).toContain('data-wuu-component="plugin-contribution"');
    expect(surface).toContain('data-wuu-surface={surfaceId}');
  });
});
