import { act, createRef } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import type { DesktopProject, InitializeResult } from "../shared/protocol";
import { AppSidebar } from "./AppSidebar";
import {
  initialState,
  SCRATCH_PSEUDO_PROJECT_ID,
  type AppState,
} from "./AppState";

let container: HTMLDivElement;
let root: Root | null = null;

beforeEach(() => {
  container = document.createElement("div");
  document.body.appendChild(container);
});

afterEach(() => {
  act(() => root?.unmount());
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
  collaborationActive?: boolean;
  onOpenCollaboration?: () => void;
  sectionOrder?: string[];
  state?: AppState;
}

function renderSidebar({
  collaborationActive = false,
  onOpenCollaboration = () => {},
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
        collapsedSidebarSectionIDs={new Set()}
        expandedSidebarSectionIDs={new Set()}
        projectThreadsByProjectID={{}}
        projectMenuOpen={false}
        projectMenuRef={createRef<HTMLDivElement>()}
        searchOpen={false}
        debugFixturesVisible={false}
        sectionOrder={sectionOrder}
        onStartNewThread={() => {}}
        onOpenSkillsTab={() => {}}
        collaborationActive={collaborationActive}
        onOpenCollaboration={onOpenCollaboration}
        onToggleConversationSearch={() => {}}
        onSeedConversationFixture={() => {}}
        onSeedAgentTreeDemo={() => {}}
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

describe("AppSidebar", () => {
  it("keeps collaboration in the fixed primary navigation and opens it", () => {
    let opened = 0;
    renderSidebar({
      collaborationActive: true,
      onOpenCollaboration: () => {
        opened += 1;
      },
    });

    const labels = Array.from(
      container.querySelectorAll<HTMLButtonElement>(".primary-nav > .nav-item"),
    ).map((button) => button.textContent?.trim());
    expect(labels).toEqual(["新对话", "搜索会话", "技能", "协作"]);

    const collaborationButton = Array.from(
      container.querySelectorAll<HTMLButtonElement>(".primary-nav > .nav-item"),
    ).find((button) => button.textContent?.trim() === "协作");
    expect(collaborationButton?.classList.contains("active")).toBe(true);
    act(() => collaborationButton?.click());
    expect(opened).toBe(1);
  });

  it("shows collaboration even before the runtime initializes", () => {
    renderSidebar({ state: initialState });

    const collaborationButton = Array.from(
      container.querySelectorAll<HTMLButtonElement>(".primary-nav > .nav-item"),
    ).find((button) => button.textContent?.trim() === "协作");
    expect(collaborationButton).toBeDefined();
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
