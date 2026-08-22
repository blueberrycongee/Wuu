import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { act, createRef } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import type { ChannelRoom, DesktopProject, InitializeResult } from "../shared/protocol";
import { AppSidebar } from "./AppSidebar";
import { desktopPluginHost } from "./plugins/DesktopPluginRuntime";
import type { NavigationSnapshotV1 } from "../shared/workbench";
import {
  initialState,
  SCRATCH_PSEUDO_PROJECT_ID,
  type AppState,
} from "./AppState";

let container: HTMLDivElement;
let root: Root | null = null;
const sidebarCSS = readFileSync(
  resolve(process.cwd(), "src/renderer/styles/sidebar.css"),
  "utf8",
);

beforeEach(() => {
  container = document.createElement("div");
  document.body.appendChild(container);
});

afterEach(() => {
  act(() => root?.unmount());
  desktopPluginHost.unload("test:app-sidebar-navigation");
  root = null;
  container.remove();
});

function initialized(): InitializeResult {
  return {
    protocol_version: "wuu-app-server/v0.1",
    provider: "test",
    model: "test-model",
    workspace_root: "/repo",
  };
}

const sidebarProjects: DesktopProject[] = [
  {
    id: SCRATCH_PSEUDO_PROJECT_ID,
    name: "对话",
    path: "",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  },
  {
    id: "project-1",
    name: "wuu",
    path: "/repo/wuu",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  },
  {
    id: "project-2",
    name: "interview",
    path: "/repo/interview",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  },
];

interface RenderOptions {
  sectionOrder?: string[];
  state?: AppState;
  groupChatEnabled?: boolean;
  channelRooms?: ChannelRoom[];
  pinnedChannelRooms?: ChannelRoom[];
  activeChannelRoomID?: string;
  activeChannelSection?: "rooms" | "agents" | "tasks" | null;
  collapsedSidebarSectionIDs?: Set<string>;
  onSelectChannelRoom?: (roomID: string) => void;
  onToggleChannelRoomPinned?: (room: ChannelRoom) => void;
  onArchiveChannelRoom?: (room: ChannelRoom) => void;
  onOpenChannelAgents?: () => void;
  onOpenChannelTasks?: () => void;
}

function renderSidebar({
  sectionOrder = [SCRATCH_PSEUDO_PROJECT_ID, "project-1", "project-2"],
  state = {
    ...initialState,
    initialized: initialized(),
    activeContext: {
      kind: "project",
      project_id: "project-1",
      cwd: "/repo/wuu",
    },
  },
  groupChatEnabled = false,
  channelRooms = [],
  pinnedChannelRooms = [],
  activeChannelRoomID,
  activeChannelSection = null,
  collapsedSidebarSectionIDs = new Set(),
  onSelectChannelRoom,
  onToggleChannelRoomPinned,
  onArchiveChannelRoom,
  onOpenChannelAgents,
  onOpenChannelTasks,
}: RenderOptions = {}): void {
  act(() => {
    root = createRoot(container);
    root.render(
      <AppSidebar
        state={state}
        sidebarProjects={sidebarProjects}
        pinnedThreads={[]}
        activeThreadID={undefined}
        pendingThreadID={undefined}
        pendingProjectID={undefined}
        collapsedSidebarSectionIDs={collapsedSidebarSectionIDs}
        expandedSidebarSectionIDs={new Set()}
        projectThreadsByProjectID={{}}
        projectMenuOpen={false}
        projectMenuRef={createRef<HTMLDivElement>()}
        searchOpen={false}
        debugFixturesVisible={false}
        sectionOrder={sectionOrder}
        onStartNewThread={() => {}}
        onOpenSkillsTab={() => {}}
        groupChatEnabled={groupChatEnabled}
        channelRooms={channelRooms}
        pinnedChannelRooms={pinnedChannelRooms}
        activeChannelRoomID={activeChannelRoomID}
        activeChannelSection={activeChannelSection}
        onSelectChannelRoom={onSelectChannelRoom}
        onToggleChannelRoomPinned={onToggleChannelRoomPinned}
        onArchiveChannelRoom={onArchiveChannelRoom}
        onOpenChannelAgents={onOpenChannelAgents}
        onOpenChannelTasks={onOpenChannelTasks}
        onOpenChannels={() => {}}
        onToggleConversationSearch={() => {}}
        onSeedConversationFixture={() => {}}
        onOpenChipGallery={() => {}}
        onSelectThread={() => {}}
        onTogglePinned={() => {}}
        onArchiveThread={() => {}}
        onDeleteThread={() => {}}
        onRenameThread={() => {}}
        onToggleProjectMenu={() => {}}
        onCreateProject={() => {}}
        onOpenProjectFolder={() => {}}
        onToggleSidebarSectionCollapsed={() => {}}
        onStartNewThreadForProject={() => {}}
        onSelectProjectThread={() => {}}
        onRemoveProject={() => {}}
        onRelocateProject={() => {}}
        onOpenSettings={() => {}}
      />,
    );
  });
}

describe("AppSidebar layout", () => {
  it("lets a navigation presenter replace the complete production sidebar root", async () => {
    let snapshot: NavigationSnapshotV1 | undefined;
    await desktopPluginHost.activateGeneration({
      pluginId: "test:app-sidebar-navigation",
      generation: "one",
      register(api) {
        api.registerPresenter({
          id: "sidebar",
          target: "navigation.primary",
          render: ({ snapshot: nextSnapshot }) => {
            snapshot = nextSnapshot as NavigationSnapshotV1;
            return <main data-custom-sidebar-root>custom</main>;
          },
        });
      },
    });

    renderSidebar();

    expect(container.querySelector("[data-custom-sidebar-root]")?.textContent).toBe("custom");
    expect(container.querySelector("aside.sidebar")).toBeNull();
    expect(snapshot?.nodes.map(({ id }) => id)).toEqual([
      "command:new-conversation",
      "command:search-conversations",
      "command:skills",
      "section:workspace",
      `project:${SCRATCH_PSEUDO_PROJECT_ID}`,
      "project:project-1",
      "project:project-2",
      "command:settings",
    ]);
  });

  it("hides group chat unless the frontend flag is enabled", () => {
    renderSidebar();

    expect(container.textContent).not.toContain("群聊");
  });

  it("replaces the legacy group chat nav item with the 协作 section", () => {
    renderSidebar({ groupChatEnabled: true });

    const navLabels = Array.from(container.querySelectorAll(".nav-item")).map((item) => item.textContent);
    expect(navLabels).not.toContain("群聊");
    expect(container.querySelector(".channel-mention-badge")).toBeNull();
  });

  it("defines a hover edge drawer for the collapsed sidebar", () => {
    expect(sidebarCSS).toContain(".sidebar-hover-zone");
    expect(sidebarCSS).toMatch(/\.sidebar-hover-zone\s*\{[\s\S]*width:\s*14px;/);
    expect(sidebarCSS).not.toMatch(
      /\.sidebar-collapsed\s+\.sidebar-hover-zone:hover[\s\S]*background:/,
    );
    expect(sidebarCSS).toContain(
      "--sidebar-drawer-bg: var(--wuu-color-surface-muted, #ffffff)",
    );
    expect(sidebarCSS).not.toMatch(/--sidebar-drawer-bg:\s*rgba\(/);
    expect(sidebarCSS).toMatch(
      /\.sidebar-collapsed\.sidebar-drawer-open \.sidebar,[\s\S]*background:\s*var\(--sidebar-drawer-bg\);/,
    );
    expect(sidebarCSS).toMatch(
      /\.sidebar-drawer-docking :is\(\.sidebar, \.settings-sidebar\)\s*\{[\s\S]*background:\s*var\(--sidebar-material-fill\);[\s\S]*background-color var\(--sidebar-motion-duration\)/,
    );
    expect(sidebarCSS).toMatch(
      /\.sidebar-drawer-docking :is\(\.sidebar, \.settings-sidebar\)::before\s*\{[\s\S]*opacity:\s*0\.5;[\s\S]*transition:\s*opacity var\(--sidebar-motion-duration\)/,
    );
    // The ease itself now lives in base.css as a shared motion token;
    // sidebar.css only consumes it.
    expect(sidebarCSS).toContain("var(--sidebar-motion-ease)");
    expect(sidebarCSS).toMatch(
      /\.sidebar-collapsed \.titlebar\s*\{[^}]*padding-left:\s*max\(24px, calc\(var\(--window-controls-inset-left\) \+ 10px\)\);/,
    );
    expect(sidebarCSS).toContain(
      ".sidebar-collapsed.sidebar-drawer-open .sidebar",
    );
    expect(sidebarCSS).toContain(
      ".sidebar-collapsed.sidebar-drawer-closing .sidebar",
    );
    expect(sidebarCSS).toContain(
      ".sidebar-collapsed.sidebar-drawer-open .sidebar .sidebar-content",
    );
    // The collapsed rail carries the drawer's off-canvas start transform
    // (excluded while the dock<->collapse grid animation runs), so the open
    // transition is a real slide-in instead of an instant pop.
    expect(sidebarCSS).toMatch(
      /\.sidebar-collapsed:not\(\.sidebar-animating\) :is\(\.sidebar, \.settings-sidebar\)\s*\{\s*transform:\s*translate3d\(-100%, 0, 0\);/,
    );
    expect(sidebarCSS).toMatch(
      /\.sidebar-collapsed\.sidebar-drawer-open :is\(\.sidebar, \.settings-sidebar\)[\s\S]*?transition:\s*transform\s+var\(--sidebar-drawer-enter-duration\)\s+var\(--sidebar-drawer-enter-easing\);/,
    );
    // The titlebar toggle stays above the drawer (140) as a stationary click
    // target while the panel slides underneath it.
    expect(sidebarCSS).toMatch(
      /\.sidebar-collapsed :is\(\.titlebar, \.settings-titlebar\) \.sidebar-toggle-button\s*\{[^}]*position:\s*relative;[^}]*z-index:\s*150;/,
    );
    expect(sidebarCSS).toMatch(
      /\.sidebar-collapsed\.sidebar-drawer-open :is\(\.sidebar, \.settings-sidebar\),[\s\S]*?z-index:\s*140;/,
    );
    // Closing slides the panel back off-screen with the exit tokens; the old
    // whole-panel opacity fade must not come back.
    expect(sidebarCSS).toMatch(
      /\.sidebar-collapsed\.sidebar-drawer-closing \.sidebar,\s*\.sidebar-collapsed\.sidebar-drawer-closing \.settings-sidebar\s*\{\s*transform:\s*translate3d\(-100%, 0, 0\);\s*transition:\s*transform\s+var\(--sidebar-drawer-exit-duration\)\s+var\(--sidebar-drawer-exit-easing\);/,
    );
    expect(sidebarCSS).not.toMatch(
      /\.sidebar-collapsed\.sidebar-drawer-closing \.settings-sidebar\s*\{[^}]*opacity:\s*0/,
    );
  });

  it("keeps primary actions outside the scrollable sidebar list", () => {
    renderSidebar();

    const content = container.querySelector(".sidebar-content");
    const primaryNav = container.querySelector(".primary-nav");
    const scrollRegion = container.querySelector(".sidebar-main");

    expect(primaryNav?.parentElement).toBe(content);
    expect(scrollRegion?.classList.contains("scrollbar-hidden")).toBe(true);
    expect(scrollRegion?.contains(primaryNav)).toBe(false);
    expect(scrollRegion?.querySelector(".project-section")).not.toBeNull();
  });

  it("keeps the workspace add action visible and collaboration controls out of Harness", () => {
    renderSidebar({ groupChatEnabled: true });

    const workspaceAction = container.querySelector<HTMLButtonElement>(
      '[aria-label="添加工作区"]',
    );
    const collaborationAction = container.querySelector<HTMLButtonElement>(
      '[aria-label="新建频道"]',
    );
    expect(workspaceAction?.classList.contains("sidebar-functional-action")).toBe(true);
    expect(collaborationAction).toBeNull();

    const sharedActionRule =
      sidebarCSS.match(/\.sidebar-functional-action\s*\{[^}]*\}/)?.[0] ?? "";
    expect(sharedActionRule).toMatch(/place-items:\s*center/);
    expect(sharedActionRule).toMatch(/padding:\s*0/);
    expect(sharedActionRule).toMatch(/background:\s*transparent/);

    const sharedActionHoverRule =
      sidebarCSS.match(
        /\.sidebar-functional-action:hover,\s*\.sidebar-functional-action:focus-visible,\s*\.sidebar-functional-action\[aria-expanded="true"\]\s*\{[^}]*\}/,
      )?.[0] ?? "";
    expect(sharedActionHoverRule).toMatch(
      /background:\s*var\(--sidebar-row-icon-hover-bg-default\)/,
    );

    const sectionActionRule =
      sidebarCSS.match(/\.sidebar-section-add-action\s*\{[^}]*\}/)?.[0] ?? "";
    expect(sectionActionRule).toMatch(/top:\s*50%/);
    expect(sectionActionRule).toMatch(/right:\s*var\(--sidebar-row-pad-x, 8px\)/);
    expect(sectionActionRule).toMatch(/transform:\s*translateY\(-50%\)/);
  });

  it("keeps sidebar buttons on a stable horizontal axis across interaction states", () => {
    const rowHoverRule =
      sidebarCSS.match(
        /\.nav-item:hover,\s*\.project-row:hover,\s*\.thread-row:hover\s*\{[^}]*\}/,
      )?.[0] ?? "";
    const rowActiveRule =
      sidebarCSS.match(/\.nav-item:active,\s*\.project-row:active\s*\{[^}]*\}/)?.[0] ?? "";
    const settingsHoverRule =
      sidebarCSS.match(
        /\.sidebar-settings-button:hover,\s*\.sidebar-settings-button\[aria-expanded="true"\]\s*\{[^}]*\}/,
      )?.[0] ?? "";

    expect(rowHoverRule).toMatch(/transform:\s*none/);
    expect(rowActiveRule).toMatch(/transform:\s*scale\(0\.992\)/);
    expect(settingsHoverRule).toMatch(/transform:\s*none/);
    expect([rowHoverRule, rowActiveRule, settingsHoverRule].join("\n")).not.toMatch(
      /translateX|translate3d/,
    );
  });

  it("renders the brand lockup above the primary nav", () => {
    renderSidebar();

    const content = container.querySelector(".sidebar-content");
    const brand = content?.querySelector(".sidebar-brand");
    const primaryNav = content?.querySelector(".primary-nav");

    expect(brand).not.toBeNull();
    expect(brand?.querySelector(".sidebar-brand-wordmark")?.textContent).toBe("wuu");
    expect(brand?.querySelector(".sidebar-brand-descriptor")?.textContent).toBe("harness");
    expect(brand?.querySelector(".sidebar-brand-descriptor")?.getAttribute("aria-pressed")).toBe("true");
    expect(brand?.textContent?.trim()).toBe("wuuharness");
    expect(brand?.nextElementSibling).toBe(primaryNav);
  });

  it("renders only scratch and projects in the workspace order", () => {
    renderSidebar({
      sectionOrder: ["project-2", SCRATCH_PSEUDO_PROJECT_ID, "project-1"],
    });

    const sections = Array.from(
      container.querySelectorAll<HTMLElement>(
        ".sidebar-functional-group-body > section[data-section-id]",
      ),
    );
    expect(sections.map((section) => section.dataset.sectionId)).toEqual([
      "project-2",
      SCRATCH_PSEUDO_PROJECT_ID,
      "project-1",
    ]);
  });
});
