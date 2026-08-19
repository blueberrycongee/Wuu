import {
  Archive,
  Clock,
  FileText,
  Folder,
  FolderOpen,
  FolderPlus,
  LayoutGrid,
  List as ListIcon,
  MessageSquare,
  MessageSquarePlus,
  MessagesSquare,
  Pin,
  Plus,
  Search,
  Settings,
} from "lucide-react";
import {
  type PointerEvent as ReactPointerEvent,
  type RefObject,
  useCallback,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore,
} from "react";
import {
  closestCenter,
  DndContext,
  DragOverlay,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragStartEvent,
} from "@dnd-kit/core";
import { restrictToVerticalAxis } from "@dnd-kit/modifiers";
import {
  SortableContext,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import type { ChannelRoom, DesktopProject } from "../shared/protocol";
import {
  isThreadRunning,
  isThreadUnread,
  type AppState,
  type ThreadSummary,
} from "./AppState";
import type { ConversationFixtureKind } from "./ConversationFixtures";
import { SCRATCH_PSEUDO_PROJECT_ID } from "./AppState";
import { PinnedThreadList, ProjectGroup } from "./ThreadSidebar";
import { SidebarSection } from "./SidebarSection";
import {
  SidebarSectionDragPreview,
  SortableSidebarSection,
  reorderSidebarSections,
  type SidebarSectionHeaderInfo,
} from "./SortableSidebarSection";
export { reorderSidebarSections } from "./SortableSidebarSection";
import { PluginBlocksIcon } from "./PluginBlocksIcon";
import { PluginIcon } from "./PublicIcon";
import { AppModeSwitch } from "./AppModeSwitch";
import { useI18n } from "./i18n";
import {
  NavigationPresentation,
  type NavigationSourceNode,
} from "./plugins/NavigationPresentation";
import {
  desktopPluginHost,
  desktopWorkbenchController,
} from "./plugins/DesktopPluginRuntime";
import type { PluginHost } from "./plugins/PluginHost";
import type { WorkbenchController } from "./plugins/Workbench";
import { PluginSlot } from "./plugins/PluginSlot";

/**
 * Stable section identity keys for the new sidebar tree.
 *
 * - `SIDEBAR_SECTION_PINNED` is FIXED-position (always first, above the
 *   reorderable list). It is intentionally NOT in `sectionOrder`.
 * - `SCRATCH_PSEUDO_PROJECT_ID` ("__wuu_scratch__") is the 对话
 *   pseudo-project entry. It belongs to the workspace functional group and
 *   can be reordered with real projects.
 */
export const SIDEBAR_SECTION_PINNED = "__wuu_pinned__";

/**
 * Fixed-position 协作 (group chat) section. Like the pinned section it is
 * intentionally NOT part of the reorderable `sectionOrder` — it always sits
 * between 置顶 and the workspace group so the sidebar anatomy stays
 * predictable regardless of how the user reorders projects.
 */
export const SIDEBAR_SECTION_COLLAB = "__wuu_collab__";

/**
 * Reconcile the persisted sidebar section order against the current
 * project list. Pure function so it is directly testable.
 *
 * Rules:
 *   1. Drop any stored key that is neither a real project id nor
 *      `SCRATCH_PSEUDO_PROJECT_ID` (including stale fixed or legacy section
 *      ids) once `projectIDs` is known. When `projectIDs` is empty the real
 *      project list has not been loaded yet, so stored keys are preserved —
 *      pruning them against an empty list would destroy the user's persisted
 *      workspace order on every launch.
 *   2. Append newly-seen project ids in `projectIDs` order.
 *   3. Ensure the scratch entry is present while preserving workspace order.
 */
export function reconcileSidebarSectionOrder(
  stored: string[] | undefined,
  projectIDs: string[],
): string[] {
  const projectIDsKnown = projectIDs.length > 0;
  const knownIDs = new Set<string>([
    SCRATCH_PSEUDO_PROJECT_ID,
    ...projectIDs,
  ]);
  const out: string[] = [];
  if (Array.isArray(stored)) {
    for (const key of stored) {
      if (typeof key !== "string" || key.length === 0) continue;
      if (key === SIDEBAR_SECTION_PINNED) continue;
      if (projectIDsKnown && !knownIDs.has(key)) continue;
      if (out.includes(key)) continue;
      out.push(key);
    }
  }
  for (const id of projectIDs) {
    if (!out.includes(id)) {
      out.push(id);
    }
  }
  if (!out.includes(SCRATCH_PSEUDO_PROJECT_ID)) {
    out.unshift(SCRATCH_PSEUDO_PROJECT_ID);
  }
  if (
    stored &&
    stored.length === out.length &&
    stored.every((key, index) => key === out[index])
  ) {
    return stored;
  }
  return out;
}

/**
 * Pure reorder helper: returns the order with `activeId` moved to the
 * position of `overId`. Pure so the drag-end logic is testable without
 * mounting the DOM or stubbing dnd-kit. Behavior contract:
 *   - overId is null / empty / equal to activeId → returns the input
 *     unchanged. dnd-kit occasionally emits "no over" when the pointer
 *     releases outside any droppable; we treat that as a no-op.
 *   - Either id absent from the order → returns the input unchanged.
 *     This preserves the persisted order when something exotic happens
 *     (e.g. the active section was removed from the reconcile result
 *     while the drag was in flight).
 */
// Context that lets SidebarSection — a shared component used by both
// the non-sortable pinned section and the sortable Agents / 对话 / 项目
// sections — pick up the dnd-kit activator listeners when it's inside a
// SortableSection. Default value is null so non-sortable callsites
// (the pinned section) fall through to the no-drag-handle path and the
// header still works as a plain toggle.
//
// The context itself lives in SidebarSection so the consumer side
// (SidebarSection) and provider side (SortableSection, below) share a
// single import without a circular type dependency. SidebarSection
// also exports the hook SidebarSection uses to read the handle.

export function AppSidebar({
  state,
  sidebarProjects,
  activeProjectID,
  pinnedThreads,
  activeThreadID,
  pendingThreadID,
  pendingProjectID,
  collapsedSidebarSectionIDs,
  expandedSidebarSectionIDs,
  loadingProjectThreadIDs,
  projectThreadsByProjectID,
  projectMenuOpen,
  projectMenuRef,
  searchOpen,
  debugFixturesVisible,
  sectionOrder,
  onStartNewThread,
  onOpenSkillsTab,
  groupChatEnabled = false,
  onToggleConversationSearch,
  onSeedConversationFixture,
  onOpenChipGallery,
  onSelectThread,
  onTogglePinned,
  onArchiveThread,
  onDeleteThread,
  onRenameThread,
  onToggleProjectMenu,
  onCreateProject,
  onOpenProjectFolder,
  onToggleSidebarSectionCollapsed,
  onSelectProjectWorkspace,
  onStartNewThreadForProject,
  onSelectProjectThread,
  onRemoveProject,
  onRelocateProject,
  onReorderSections,
  onPointerEnter,
  onPointerLeave,
  onOpenSettings,
  onSwitchToCollaboration,
  pluginHost = desktopPluginHost,
  workbenchController = desktopWorkbenchController,
}: {
  state: AppState;
  // The sidebar renders scratch conversations through the same ProjectList
  // path as real projects, so App.tsx prepends a synthetic DesktopProject
  // (id = SCRATCH_PSEUDO_PROJECT_ID) into this array. The original
  // state.projects list is unchanged; sidebarProjects is what the sidebar
  // actually shows.
  sidebarProjects: DesktopProject[];
  activeProjectID?: string;
  pinnedThreads: ThreadSummary[];
  activeThreadID?: string;
  pendingThreadID?: string;
  pendingProjectID?: string;
  collapsedSidebarSectionIDs: Set<string>;
  expandedSidebarSectionIDs: Set<string>;
  loadingProjectThreadIDs?: ReadonlySet<string>;
  projectThreadsByProjectID: Record<string, ThreadSummary[]>;
  projectMenuOpen: boolean;
  projectMenuRef: RefObject<HTMLDivElement | null>;
  searchOpen: boolean;
  debugFixturesVisible: boolean;
  // Order of reorderable sections. The pinned section is rendered first
  // (fixed position) and is NOT included. Each key maps to either
  // SCRATCH_PSEUDO_PROJECT_ID or a real project id.
  sectionOrder: string[];
  onStartNewThread: () => void;
  onOpenSkillsTab: () => void;
  groupChatEnabled?: boolean;
  // Unified 协作 section: the room list (with per-room unread counts) is
  // polled at the App level and passed down so the sidebar and the channel
  // canvas never disagree about what needs attention.
  channelRooms?: ChannelRoom[];
  pinnedChannelRooms?: ChannelRoom[];
  activeChannelRoomID?: string;
  // Which channel canvas is on screen, or null when channels are closed —
  // drives the Agents / 任务 entry row highlights.
  activeChannelSection?: "rooms" | "agents" | "tasks" | null;
  onSelectChannelRoom?: (roomID: string) => void;
  onToggleChannelRoomPinned?: (room: ChannelRoom) => void;
  onArchiveChannelRoom?: (room: ChannelRoom) => void;
  onOpenChannelAgents?: () => void;
  onOpenChannelTasks?: () => void;
  onOpenChannels?: () => void;
  onCreateChannelRoom?: () => void;
  onToggleConversationSearch: () => void;
  onSeedConversationFixture: (kind: ConversationFixtureKind) => void;
  onOpenChipGallery: () => void;
  onSelectThread: (id: string) => void;
  onTogglePinned: (thread: ThreadSummary) => void;
  onArchiveThread: (thread: ThreadSummary) => void;
  onDeleteThread: (thread: ThreadSummary) => void;
  onRenameThread: (thread: ThreadSummary, title: string) => void;
  onToggleProjectMenu: () => void;
  onCreateProject: () => void;
  onOpenProjectFolder: () => void;
  onToggleSidebarSectionCollapsed: (id: string) => void;
  onSelectProjectWorkspace?: (id: string) => void;
  onStartNewThreadForProject: (id: string) => void;
  onSelectProjectThread: (projectID: string, threadID: string) => void;
  onRemoveProject: (id: string) => void;
  onRelocateProject: (id: string) => void;
  // Fires when the user drops a reorderable sidebar section in a new
  // position. The next array is the FULL sectionOrder with the moved
  // entry swapped into place. App.tsx persists this via the same
  // sidebarSectionOrder effect that already exists. Optional so test
  // harnesses and other consumers can render the sidebar without
  // wiring persistence — production callers (App.tsx) always provide it.
  onReorderSections?: (nextOrder: string[]) => void;
  onPointerEnter?: () => void;
  onPointerLeave?: (event: ReactPointerEvent<HTMLElement>) => void;
  onOpenSettings: () => void;
  onSwitchToCollaboration?: () => void;
  pluginHost?: PluginHost;
  workbenchController?: WorkbenchController;
}): JSX.Element {
  const { t } = useI18n();
  const hasRuntimeContext = Boolean(state.activeContext);
  const fixturesEnabled = hasRuntimeContext && Boolean(state.initialized);
  // The scratch pseudo project is "active" when the runtime context is in
  // no-project mode (i.e. the user is viewing a scratch conversation).
  // Active state is passed into ProjectList so the row highlights even though
  // it has no DesktopProject entry in state.projects.
  const sidebarScratchPseudoActive = state.activeContext?.kind === "no_project";
  const pluginNavigationEntries = useSyncExternalStore(
    (listener) => pluginHost.subscribe(listener),
    () => pluginHost.getNavigationEntries(),
    () => pluginHost.getNavigationEntries(),
  );
  const workbenchSnapshot = useSyncExternalStore(
    workbenchController.subscribe,
    workbenchController.getSnapshot,
    workbenchController.getSnapshot,
  );
  const activePluginMainView = workbenchSnapshot.views.find(
    (view) => view.id === workbenchSnapshot.activeViewByRegion.primary,
  );
  const activateNative = useCallback((action: () => void): void => {
    workbenchController.deactivateRegion("primary");
    action();
  }, [workbenchController]);
  const openPluginNavigation = useCallback((pluginId: string, viewTypeId: string): void => {
    void workbenchController.openPluginView(pluginId, viewTypeId, {
      region: "primary",
      persistence: "durable",
      reveal: true,
    }).catch(() => workbenchController.deactivateRegion("primary"));
  }, [workbenchController]);

  // Drag-and-drop reorder wiring for the reorderable sections. The 6px
  // activation distance lets plain clicks on the header (collapse toggle)
  // and on threads inside the section pass through without triggering a
  // drag — matches SessionTabs so the two surfaces share a feel.
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 6 } }),
  );
  const [draggingSectionID, setDraggingSectionID] = useState<string | undefined>();
  function handleDragStart(event: DragStartEvent): void {
    setDraggingSectionID(String(event.active.id));
  }
  function handleDragEnd(event: DragEndEvent): void {
    const activeID = String(event.active.id);
    const overID = event.over ? String(event.over.id) : undefined;
    const next = reorderSidebarSections(sectionOrder, activeID, overID);
    if (next !== sectionOrder) {
      onReorderSections?.(next);
    }
    setDraggingSectionID(undefined);
  }
  function handleDragCancel(): void {
    setDraggingSectionID(undefined);
  }
  // Each SortableSection calls back into this registry on every render
  // with its id → header info. The DragOverlay reads from this map to
  // render the right header for the currently dragged section. Using a
  // ref (not state) avoids an extra render when a child mounts and
  // keeps the lookup cheap during high-frequency drag pointer moves.
  const sectionHeaderInfoByIDRef = useRef<Map<string, SidebarSectionHeaderInfo>>(new Map());
  const registerSectionHeaderInfo = useCallback(
    (id: string, info: SidebarSectionHeaderInfo | null): void => {
      if (info === null) {
        sectionHeaderInfoByIDRef.current.delete(id);
      } else {
        sectionHeaderInfoByIDRef.current.set(id, info);
      }
    },
    [],
  );
  const draggingSectionInfo = draggingSectionID
    ? sectionHeaderInfoByIDRef.current.get(draggingSectionID)
    : undefined;
  const pinnedRows = pinnedThreads;
  const hasPinnedRows = pinnedRows.length > 0;
  const pinnedCollapsed = collapsedSidebarSectionIDs.has(
    SIDEBAR_SECTION_PINNED,
  );
  const functionalGroups = ([{
    id: "workspace" as const,
    label: t("sidebar.workspace"),
    sectionIDs: sectionOrder,
  }] satisfies Array<{
    id: "workspace";
    label: string;
    sectionIDs: string[];
  }>).filter((group) => group.sectionIDs.length > 0);

  const navigationNodes = useMemo<readonly NavigationSourceNode[]>(() => {
    const nodes: NavigationSourceNode[] = [
      {
        id: "command:new-conversation",
        kind: "command",
        label: t("sidebar.newConversation"),
        icon: "message-square-plus",
        disabled: !hasRuntimeContext,
        onActivate: () => activateNative(onStartNewThread),
      },
      {
        id: "command:search-conversations",
        kind: "command",
        label: t("sidebar.searchConversations"),
        icon: "search",
        active: searchOpen,
        disabled: !hasRuntimeContext,
        onActivate: () => activateNative(onToggleConversationSearch),
      },
    ];
    nodes.push(
      {
        id: "command:skills",
        kind: "command",
        label: t("skills.sectionSkills"),
        icon: "plugin-blocks",
        disabled: !hasRuntimeContext,
        onActivate: () => activateNative(onOpenSkillsTab),
      },
    );
    if (debugFixturesVisible) {
      const fixtureCommands: Array<[string, string, () => void, boolean]> = [
        ["fixture-long", t("sidebar.devFixtures.longConversation"), () => onSeedConversationFixture("long"), !fixturesEnabled],
        ["fixture-rich", t("sidebar.devFixtures.richContent"), () => onSeedConversationFixture("rich"), !fixturesEnabled],
        ["fixture-running", t("sidebar.devFixtures.running"), () => onSeedConversationFixture("running"), !fixturesEnabled],
        ["fixture-compact", t("sidebar.devFixtures.compaction"), () => onSeedConversationFixture("compact"), !fixturesEnabled],
        ["fixture-chips", t("sidebar.devFixtures.chipGallery"), onOpenChipGallery, false],
      ];
      for (const [id, label, onActivate, disabled] of fixtureCommands) {
        nodes.push({ id: `command:${id}`, kind: "command", label, disabled, onActivate });
      }
    }

    if (hasPinnedRows) {
      nodes.push({
        id: "section:pinned",
        kind: "section",
        label: t("sidebar.pinned"),
        icon: "pin",
        depth: 0,
      });
      for (const thread of pinnedRows) {
        nodes.push(threadNavigationNode(
          thread,
          "section:pinned",
          activeThreadID,
          state.lastViewedTurnByThreadID,
          () => onSelectThread(thread.id),
          () => onTogglePinned(thread),
        ));
      }
    }

    if (sectionOrder.length > 0) {
      nodes.push({
        id: "section:workspace",
        kind: "section",
        label: t("sidebar.workspace"),
        depth: 0,
      });
      for (const projectID of sectionOrder) {
        const project = sidebarProjects.find((candidate) => candidate.id === projectID);
        if (!project) continue;
        const threads = (projectThreadsByProjectID[projectID] ?? []).filter(
          (thread) => !thread.pinned,
        );
        const isScratch = projectID === SCRATCH_PSEUDO_PROJECT_ID;
        const projectActive = (isScratch
          ? sidebarScratchPseudoActive
          : projectID === (activeProjectID ?? state.activeProjectId)) &&
          !threads.some((thread) => thread.id === activeThreadID);
        nodes.push({
          id: `project:${projectID}`,
          kind: "project",
          label: isScratch ? t("sidebar.conversations") : project.name,
          parentId: "section:workspace",
          depth: 1,
          icon: isScratch ? "messages" : "folder",
          active: projectActive,
          unread: threads.some((thread) => isThreadUnread(
            thread,
            state.lastViewedTurnByThreadID[thread.id],
          )),
          running: threads.some((thread) => isThreadRunning(thread)),
          disabled: onSelectProjectWorkspace === undefined,
          onActivate: onSelectProjectWorkspace === undefined
            ? undefined
            : () => onSelectProjectWorkspace(projectID),
        });
        for (const thread of threads) {
          nodes.push(threadNavigationNode(
            thread,
            `project:${projectID}`,
            activeThreadID,
            state.lastViewedTurnByThreadID,
            () => onSelectProjectThread(projectID, thread.id),
            () => onTogglePinned(thread),
          ));
        }
      }
    }
    if (pluginNavigationEntries.length > 0) {
      nodes.push({
        id: "section:plugins",
        kind: "section",
        label: t("skills.sectionPlugins"),
        icon: "plugin-blocks",
        depth: 0,
      });
      for (const entry of pluginNavigationEntries) {
        nodes.push({
          id: `plugin:${entry.pluginId}:${entry.id}`,
          kind: "command",
          parentId: "section:plugins",
          depth: 1,
          label: entry.title,
          icon: entry.icon && "name" in entry.icon ? entry.icon.name : "plugin-blocks",
          active: activePluginMainView?.pluginId === entry.pluginId
            && activePluginMainView.viewTypeId === entry.view,
          onActivate: () => openPluginNavigation(entry.pluginId, entry.view),
        });
      }
    }
    nodes.push({
      id: "command:settings",
      kind: "command",
      label: t("sidebar.settings"),
      icon: "settings",
      disabled: !state.initialized,
      onActivate: () => activateNative(onOpenSettings),
    });
    return Object.freeze(nodes);
  }, [
    activateNative, activeProjectID, activeThreadID,
    debugFixturesVisible, fixturesEnabled, hasPinnedRows, hasRuntimeContext,
    onOpenChipGallery, onOpenSettings, onOpenSkillsTab,
    onSeedConversationFixture,
    onSelectProjectThread, onSelectProjectWorkspace, onSelectThread,
    onStartNewThread, onToggleConversationSearch,
    onTogglePinned, pinnedRows, projectThreadsByProjectID,
    searchOpen, sectionOrder, sidebarProjects, sidebarScratchPseudoActive,
    activePluginMainView, openPluginNavigation, pluginNavigationEntries,
    state.activeProjectId, state.initialized, state.lastViewedTurnByThreadID, t,
  ]);

  const nativeSidebar = (
    <aside
      className="sidebar"
      data-wuu-component="sidebar"
      onPointerEnter={onPointerEnter}
      onPointerLeave={onPointerLeave}
    >
      <div className="sidebar-content">
        <div className="traffic-spacer" />
        <AppModeSwitch
          mode="harness"
          collaborationEnabled={groupChatEnabled}
          onChange={(mode) => { if (mode === "collaboration") onSwitchToCollaboration?.(); }}
        />
        <nav className="primary-nav" aria-label={t("sidebar.mainNavigation")}>
          <button
            className="nav-item"
            onClick={() => activateNative(onStartNewThread)}
            disabled={!hasRuntimeContext}
          >
            <MessageSquarePlus className="icon-lg" />
            <span>{t("sidebar.newConversation")}</span>
          </button>
          <button
            className="nav-item conversation-search-trigger"
            type="button"
            aria-haspopup="dialog"
            aria-expanded={searchOpen}
            onClick={() => activateNative(onToggleConversationSearch)}
            disabled={!hasRuntimeContext}
          >
            <Search className="icon-lg" />
            <span>{t("sidebar.searchConversations")}</span>
          </button>
          <button
            className="nav-item"
            onClick={() => activateNative(onOpenSkillsTab)}
            disabled={!hasRuntimeContext}
          >
            <PluginBlocksIcon className="icon-lg" />
            <span>{t("skills.sectionSkills")}</span>
          </button>
          {debugFixturesVisible ? (
            <div className="dev-fixture-nav" aria-label={t("sidebar.devFixtures.label")}>
              <div className="dev-fixture-label">{t("sidebar.devFixtures.title")}</div>
              <button
                className="nav-item dev-fixture-button"
                onClick={() => onSeedConversationFixture("long")}
                disabled={!fixturesEnabled}
              >
                <FileText className="icon" />
                <span>{t("sidebar.devFixtures.longConversation")}</span>
              </button>
              <button
                className="nav-item dev-fixture-button"
                onClick={() => onSeedConversationFixture("rich")}
                disabled={!fixturesEnabled}
              >
                <ListIcon className="icon" />
                <span>{t("sidebar.devFixtures.richContent")}</span>
              </button>
              <button
                className="nav-item dev-fixture-button"
                onClick={() => onSeedConversationFixture("running")}
                disabled={!fixturesEnabled}
              >
                <Clock className="icon" />
                <span>{t("sidebar.devFixtures.running")}</span>
              </button>
              <button
                className="nav-item dev-fixture-button"
                onClick={() => onSeedConversationFixture("compact")}
                disabled={!fixturesEnabled}
              >
                <Archive className="icon" />
                <span>{t("sidebar.devFixtures.compaction")}</span>
              </button>
              <button
                className="nav-item dev-fixture-button"
                onClick={onOpenChipGallery}
              >
                <LayoutGrid className="icon" />
                <span>{t("sidebar.devFixtures.chipGallery")}</span>
              </button>
            </div>
          ) : null}
        </nav>

        <div className="sidebar-main scrollbar-hidden">
          {pluginNavigationEntries.length > 0 ? (
            <section
              className="sidebar-functional-group plugin-navigation-group"
              aria-label={t("skills.sectionPlugins")}
              data-wuu-component="plugin-navigation"
            >
              <div className="sidebar-functional-heading">
                <span className="sidebar-functional-heading-label">{t("skills.sectionPlugins")}</span>
              </div>
              <div className="sidebar-functional-group-body">
                {pluginNavigationEntries.map((entry) => {
                  const active = activePluginMainView?.pluginId === entry.pluginId
                    && activePluginMainView.viewTypeId === entry.view;
                  return (
                    <button
                      key={`${entry.pluginId}:${entry.id}`}
                      type="button"
                      className={`nav-item plugin-navigation-item${active ? " active" : ""}`}
                      data-wuu-component="plugin-navigation-item"
                      data-wuu-plugin={entry.pluginId}
                      aria-current={active ? "page" : undefined}
                      title={entry.description || entry.title}
                      onClick={() => openPluginNavigation(entry.pluginId, entry.view)}
                    >
                      <PluginIcon icon={entry.icon} pluginId={entry.pluginId} fingerprint={entry.generation} className="icon-lg" />
                      <span>{entry.title}</span>
                    </button>
                  );
                })}
              </div>
            </section>
          ) : null}
          {hasPinnedRows ? (
            <section
              className="sidebar-functional-group pinned-functional-group"
              aria-label={t("sidebar.pinned")}
            >
              <div className="sidebar-functional-group-body">
                <div className="pinned-thread-section">
                  <SidebarSection
                    expanded={!pinnedCollapsed}
                    iconKind="pinned"
                    CollapsedIcon={Pin}
                    ExpandedIcon={Pin}
                    label={t("sidebar.pinned")}
                    ariaLabel={t(pinnedCollapsed ? "sidebar.expandSection" : "sidebar.collapseSection", { section: t("sidebar.pinned") })}
                    title={t(pinnedCollapsed ? "sidebar.expandSection" : "sidebar.collapseSection", { section: t("sidebar.pinned") })}
                    onToggle={() =>
                      onToggleSidebarSectionCollapsed(SIDEBAR_SECTION_PINNED)
                    }
                  >
                    <PinnedThreadList
                      threads={pinnedRows}
                      activeID={activeThreadID}
                      pendingThreadID={pendingThreadID}
                      
                      lastViewedTurnByThreadID={state.lastViewedTurnByThreadID}
                      onSelect={onSelectThread}
                      onTogglePinned={onTogglePinned}
                      onArchive={onArchiveThread}
                      onDelete={onDeleteThread}
                      onRename={onRenameThread}
                      
                    />
                  </SidebarSection>
                </div>
              </div>
            </section>
          ) : null}
          {sectionOrder.length > 0 ? (
            <DndContext
              sensors={sensors}
              collisionDetection={closestCenter}
              modifiers={[restrictToVerticalAxis]}
              onDragStart={handleDragStart}
              onDragEnd={handleDragEnd}
              onDragCancel={handleDragCancel}
            >
              <SortableContext
                items={sectionOrder}
                strategy={verticalListSortingStrategy}
              >
                {functionalGroups.map((group) => (
                  <section
                    key={group.id}
                    className="sidebar-functional-group"
                    aria-label={group.label}
                  >
                    <div className="sidebar-functional-heading">
                      <span className="sidebar-functional-heading-label">
                        {group.label}
                      </span>
                      {group.id === "workspace" ? (
                        <div
                          className="sidebar-add-workspace"
                          ref={projectMenuRef}
                        >
                          <button
                            className="sidebar-functional-action"
                            type="button"
                            aria-label={t("sidebar.addWorkspace")}
                            title={t("sidebar.addWorkspace")}
                            aria-haspopup="menu"
                            aria-expanded={projectMenuOpen}
                            onClick={onToggleProjectMenu}
                          >
                            <Plus aria-hidden="true" />
                          </button>
                          {projectMenuOpen ? (
                            <div className="project-add-menu" role="menu">
                              <button role="menuitem" onClick={onCreateProject}>
                                <FolderPlus className="icon-xl" />
                                <span>{t("sidebar.newBlankProject")}</span>
                              </button>
                              <button role="menuitem" onClick={onOpenProjectFolder}>
                                <FolderOpen className="icon-xl" />
                                <span>{t("sidebar.useExistingFolder")}</span>
                              </button>
                            </div>
                          ) : null}
                        </div>
                      ) : null}
                    </div>
                    <div className="sidebar-functional-group-body">
                      {group.sectionIDs.map((key) => {
            // For SCRATCH_PSEUDO_PROJECT_ID or any real project id, look up
            // the synthetic DesktopProject (App.tsx prepends the scratch
            // pseudo so `sidebarProjects` contains every key).
            const project = sidebarProjects.find((p) => p.id === key);
            if (!project) {
              return null;
            }
            // The 对话 scratch pseudo is surfaced with aria-label="项目" so
            // legacy single-wrapper selectors (and screen-reader users
            // scanning for "项目") still find it. Real projects expose the
            // more specific aria-label="项目 {name}" so per-project
            // automation can target them by id.
            const isScratchPseudo = project.id === SCRATCH_PSEUDO_PROJECT_ID;
            const sectionAriaLabel = isScratchPseudo
              ? t("sidebar.project")
              : t("sidebar.projectNamed", { name: project.name });
            return (
              <SortableSidebarSection
                key={key}
                id={key}
                className="project-section"
                ariaLabel={sectionAriaLabel}
                headerInfo={{
                  label: isScratchPseudo ? t("sidebar.conversations") : project.name,
                  iconKind: isScratchPseudo ? "conversation" : "project",
                  CollapsedIcon: isScratchPseudo ? MessageSquare : Folder,
                  ExpandedIcon: isScratchPseudo ? MessagesSquare : FolderOpen,
                }}
                registerHeaderInfo={registerSectionHeaderInfo}
              >
                <ProjectGroup
                  project={project}
                  activeID={activeProjectID ?? state.activeProjectId}
                  pendingProjectID={pendingProjectID}
                  expandedSidebarSectionIDs={expandedSidebarSectionIDs}
                  loadingProjectThreadIDs={loadingProjectThreadIDs}
                  threadsByProjectID={projectThreadsByProjectID}
                  activeThreadID={activeThreadID}
                  pendingThreadID={pendingThreadID}
                  
                  lastViewedTurnByThreadID={state.lastViewedTurnByThreadID}
                  scratchPseudoProjectID={SCRATCH_PSEUDO_PROJECT_ID}
                  scratchPseudoActive={sidebarScratchPseudoActive}
                  onToggleSidebarSectionCollapsed={
                    onToggleSidebarSectionCollapsed
                  }
                  onSelectProjectWorkspace={onSelectProjectWorkspace}
                  onStartNewThread={onStartNewThreadForProject}
                  onSelectThread={onSelectProjectThread}
                  onToggleThreadPinned={onTogglePinned}
                  onArchiveThread={onArchiveThread}
                  onDeleteThread={onDeleteThread}
                  onRenameThread={onRenameThread}
                  
                  onRemoveProject={onRemoveProject}
                  onRelocateProject={onRelocateProject}
                />
              </SortableSidebarSection>
            );
                      })}
                    </div>
                  </section>
                ))}
              </SortableContext>
              <DragOverlay
                dropAnimation={{
                  duration: 150,
                  easing: "cubic-bezier(0.16, 1, 0.3, 1)",
                }}
              >
                {draggingSectionInfo ? (
                  <SidebarSectionDragPreview info={draggingSectionInfo} />
                ) : null}
              </DragOverlay>
            </DndContext>
          ) : null}
        </div>
        <PluginSlot
          host={pluginHost}
          id="sidebar.primary"
          context={Object.freeze({
            initialized: Boolean(state.initialized),
            hasActiveThread: activeThreadID !== undefined,
            projectCount: sidebarProjects.length,
          })}
        />
        <div className="sidebar-settings">
          <PluginSlot
            host={pluginHost}
            id="sidebar.footer"
            context={Object.freeze({ initialized: Boolean(state.initialized) })}
          />
          <button
            className="sidebar-settings-button"
            type="button"
            disabled={!state.initialized}
            onClick={() => activateNative(onOpenSettings)}
          >
            <Settings className="icon-lg" />
            <span>{t("sidebar.settings")}</span>
          </button>
        </div>
      </div>
    </aside>
  );
  return (
    <NavigationPresentation nodes={navigationNodes} fallback={nativeSidebar} />
  );
}

function threadNavigationNode(
  thread: ThreadSummary,
  parentId: string,
  activeThreadID: string | undefined,
  lastViewedTurnByThreadID: Readonly<Record<string, string>>,
  onActivate: () => void,
  onTogglePinned: () => void,
): NavigationSourceNode {
  return {
    id: `thread:${thread.id}`,
    kind: "thread",
    label: thread.title?.trim() || thread.preview?.trim() || thread.id,
    parentId,
    depth: 2,
    active: thread.id === activeThreadID,
    pinned: Boolean(thread.pinned),
    unread: isThreadUnread(thread, lastViewedTurnByThreadID[thread.id]),
    running: isThreadRunning(thread),
    onActivate,
    onTogglePinned,
  };
}
