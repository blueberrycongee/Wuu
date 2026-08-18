import { readFileSync, readdirSync } from "node:fs";
import { relative, resolve } from "node:path";
import { describe, expect, it } from "vitest";

const RENDERER_ROOT = resolve(process.cwd(), "src/renderer");

const COMMON_LAYER_OWNERS = Object.freeze({
  "App.tsx": { layer: "notice", semanticOwner: "TopNotice.tsx" },
  "ComposerContextMenu.tsx": { layer: "menu" },
  "ComposerFloatingMenu.tsx": { layer: "menu" },
  "JumpToLatestPill.tsx": { layer: "navigation" },
  "SidebarNameDialog.tsx": { layer: "dialog" },
  "ThreadContextMenu.tsx": { layer: "menu" },
  "Toast.tsx": { layer: "notice", semanticOwner: "TopNotice.tsx" },
  "ToolDiffPreview.tsx": { layer: "popover" },
  "Tooltip.tsx": { layer: "tooltip" },
  "WorkspaceFiles.tsx": { layer: "menu" },
} as const);

const FLOATING_MENU_OWNERS = Object.freeze([
  "App.tsx",
  "ChannelComposer.tsx",
  "ComposerContextMeter.tsx",
  "ComposerRuntimeMenus.tsx",
  "ComposerTokenGauge.tsx",
  "ComposerView.tsx",
  "MinuteClockPicker.tsx",
  "QueryHistoryPopover.tsx",
  "SelectMenu.tsx",
]);

const INTENTIONAL_DIRECT_PORTAL_OWNERS = Object.freeze([
  "Modal.tsx",
  "WorkspacePanels.tsx",
  "WorkspacePdfPreview.tsx",
  "WuuMascot.tsx",
  "plugins/Workbench.tsx",
  "ui/layers/UILayerHost.tsx",
]);

function rendererSources(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = resolve(directory, entry.name);
    if (entry.isDirectory()) return rendererSources(path);
    if (!entry.name.endsWith(".tsx") || entry.name.includes(".test.")) return [];
    return [path];
  });
}

describe("production UI layer ownership", () => {
  it("routes common floating UI through the protected layer host", () => {
    for (const [owner, contract] of Object.entries(COMMON_LAYER_OWNERS)) {
      const source = readFileSync(resolve(RENDERER_ROOT, owner), "utf8");
      expect(source, `${owner} protected portal`).toContain(
        `<UILayerPortal layer="${contract.layer}">`,
      );
      expect(source, `${owner} direct portal`).not.toContain("createPortal");
      const semanticOwner = "semanticOwner" in contract
        ? contract.semanticOwner
        : owner;
      const semanticSource = readFileSync(
        resolve(RENDERER_ROOT, semanticOwner),
        "utf8",
      );
      expect(semanticSource, `${semanticOwner} semantic component`).toContain(
        "data-wuu-component=",
      );
      expect(semanticSource, `${semanticOwner} semantic layer`).toContain(
        `data-wuu-layer="${contract.layer}"`,
      );
    }
  });

  it("routes anchored floating menus through the shared floating-menu portal", () => {
    for (const owner of FLOATING_MENU_OWNERS) {
      const source = readFileSync(resolve(RENDERER_ROOT, owner), "utf8");
      expect(source, `${owner} floating menu portal`).toContain(
        "FloatingMenuPortal",
      );
      expect(source, `${owner} direct portal`).not.toContain("createPortal");
    }
  });

  it("limits direct React portals to specialized rendering boundaries", () => {
    const owners = rendererSources(RENDERER_ROOT)
      .filter((path) => readFileSync(path, "utf8").includes("createPortal"))
      .map((path) => relative(RENDERER_ROOT, path))
      .sort();

    expect(owners).toEqual([...INTENTIONAL_DIRECT_PORTAL_OWNERS].sort());
  });

});
