import * as React from "react";
import { createPortal } from "react-dom";

import type { ExtensionInventoryRecord } from "../../shared/protocol";
import {
  STATUS_ACTIONS,
  WORKBENCH_LAYOUT_STATE_VERSION,
  type OpenViewOptions,
  type PresentationHost,
  type RendererCategory,
  type StatusSnapshotV1,
  type ViewHostAPI,
  type ViewPane,
  type WorkbenchLayoutState,
  type WorkbenchViewState,
} from "../../shared/workbench";
import {
  canonicalThemeTokenName,
  isPublicSyntaxTokenName,
  isPublicThemeTokenName,
} from "../../shared/themeContract.generated";
import type {
  PluginHost,
  RegisteredRenderer,
  RegisteredViewType,
} from "./PluginHost";
import { PluginPresentation } from "./PluginPresentation";

const LAYOUT_STORAGE_KEY = "wuu.plugin-workbench.layout.v1";
const MAX_STORAGE_VALUE_LENGTH = 1_048_576;
const STORAGE_KEY_PATTERN = /^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/;
const EMPTY_WORKBENCH_SERVICES: WorkbenchServices = Object.freeze({});

interface StorageLike {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
  removeItem(key: string): void;
}

export interface WorkbenchServices {
  getSetting?(pluginId: string, generation: string, key: string): unknown | Promise<unknown>;
  getStorage?(pluginId: string, generation: string, key: string, scope: "user" | "workspace"): Promise<string | null>;
  setStorage?(pluginId: string, generation: string, key: string, value: string, scope: "user" | "workspace"): Promise<void>;
  openSettings?(): void;
  disablePlugin?(pluginId: string): void | Promise<void>;
  reportError?(pluginId: string, generation: string, error: unknown): void;
}

export interface WorkbenchSnapshot extends WorkbenchLayoutState {
  /** Registered definitions are host-owned immutable snapshots. */
  viewTypes: readonly RegisteredViewType[];
  renderers: readonly RegisteredRenderer[];
}

const EMPTY_STATE: WorkbenchLayoutState = Object.freeze({
  version: WORKBENCH_LAYOUT_STATE_VERSION,
  views: Object.freeze([]),
  activeViewByPane: Object.freeze({}),
  dismissedLayoutIds: Object.freeze([]),
});

export class WorkbenchController {
  private readonly listeners = new Set<() => void>();
  private readonly storage: StorageLike;
  private readonly unsubscribeHost: () => void;
  private readonly themeObserver: MutationObserver;
  private state: WorkbenchLayoutState;
  private snapshot: WorkbenchSnapshot;
  private availablePluginIds: ReadonlySet<string> | undefined;
  private nextInstance = 1;
  private appliedThemeTokens = new Set<string>();
  services: WorkbenchServices;

  constructor(
    readonly host: PluginHost,
    services: WorkbenchServices = {},
    storage: StorageLike = window.localStorage,
  ) {
    this.services = services;
    this.storage = storage;
    this.state = readLayoutState(storage);
    this.nextInstance = nextViewInstanceSequence(this.state.views);
    this.snapshot = this.createSnapshot();
    this.unsubscribeHost = host.subscribe(() => this.reconcileHost());
    this.themeObserver = new MutationObserver(() => this.applyThemeTokens());
    this.themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });
    this.reconcileHost();
  }

  updateServices(services: WorkbenchServices): void {
    this.services = services;
  }

  dispose(): void {
    this.unsubscribeHost();
    this.themeObserver.disconnect();
    for (const token of this.appliedThemeTokens) {
      document.documentElement.style.removeProperty(token);
    }
    this.appliedThemeTokens.clear();
    this.listeners.clear();
  }

  subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  };

  getSnapshot = (): WorkbenchSnapshot => this.snapshot;

  setAvailablePluginIds(pluginIds: ReadonlySet<string>): void {
    this.availablePluginIds = new Set(pluginIds);
    this.reconcileHost();
  }

  async openView(viewTypeId: string, options: OpenViewOptions = {}): Promise<string> {
    return this.openResolvedView(viewTypeId, options);
  }

  async openPluginView(pluginId: string, viewTypeId: string, options: OpenViewOptions = {}): Promise<string> {
    return this.openResolvedView(viewTypeId, options, pluginId);
  }

  private openResolvedView(
    viewTypeId: string,
    options: OpenViewOptions,
    preferredPluginId?: string,
  ): string {
    const definition = this.findViewType(viewTypeId, preferredPluginId);
    if (!definition) {
      throw new Error(`Plugin view type is not available: ${viewTypeId}`);
    }
    const pane = options.pane ?? definition.defaultPane ?? "main";
    if (options.reveal !== false) {
      const existing = this.state.views.find((view) =>
        view.pluginId === definition.pluginId && view.viewTypeId === definition.id && view.pane === pane);
      if (existing) {
        this.activateView(existing.id);
        return existing.id;
      }
    }
    const instance: WorkbenchViewState = Object.freeze({
      id: `${definition.pluginId}:${definition.id}:${this.nextInstance++}`,
      pluginId: definition.pluginId,
      generation: definition.generation,
      viewTypeId: definition.id,
      pane,
      persistence: options.persistence ?? definition.persistence ?? "session",
      context: freezeContext(options.context),
    });
    this.replaceState({
      ...this.state,
      views: [...this.state.views, instance],
      activeViewByPane: { ...this.state.activeViewByPane, [pane]: instance.id },
    });
    return instance.id;
  }

  async closeView(instanceId: string): Promise<void> {
    const closing = this.state.views.find((view) => view.id === instanceId);
    if (!closing) return;
    const views = this.state.views.filter((view) => view.id !== instanceId);
    const activeViewByPane = { ...this.state.activeViewByPane };
    if (activeViewByPane[closing.pane] === instanceId) {
      activeViewByPane[closing.pane] = [...views].reverse().find((view) => view.pane === closing.pane)?.id;
    }
    this.replaceState({
      ...this.state,
      views,
      activeViewByPane,
      dismissedLayoutIds: closing.sourceLayoutId
        ? [...new Set([...this.state.dismissedLayoutIds, closing.sourceLayoutId])]
        : this.state.dismissedLayoutIds,
    });
  }

  activateView(instanceId: string): void {
    const view = this.state.views.find((candidate) => candidate.id === instanceId);
    if (!view) return;
    this.replaceState({
      ...this.state,
      activeViewByPane: { ...this.state.activeViewByPane, [view.pane]: view.id },
    });
  }

  deactivatePane(pane: ViewPane): void {
    if (this.state.activeViewByPane[pane] === HIDDEN_PANE_VIEW_ID) return;
    this.replaceState({
      ...this.state,
      activeViewByPane: { ...this.state.activeViewByPane, [pane]: HIDDEN_PANE_VIEW_ID },
    });
  }

  getRenderer(category: RendererCategory, contentType: string): RegisteredRenderer | undefined {
    return [...this.host.getRenderers(category)].reverse()
      .find((renderer) => rendererMatches(renderer, contentType));
  }

  createViewHostAPI(view: WorkbenchViewState): ViewHostAPI {
    const requireActive = (): void => {
      if (!this.host.isGenerationActive(view.pluginId, view.generation)) {
        throw new Error("Plugin host context is no longer active");
      }
    };
    return Object.freeze({
      getStorage: async (key: string, scope: "user" | "workspace" = "workspace") => { requireActive(); return this.getPluginStorage(view.pluginId, view.generation, key, scope); },
      setStorage: async (key: string, value: string, scope: "user" | "workspace" = "workspace") => { requireActive(); return this.setPluginStorage(view.pluginId, view.generation, key, value, scope); },
      getSetting: async (key: string) => { requireActive(); return this.getPluginSetting(view.pluginId, view.generation, key); },
      executeCommand: async (commandId: string, input?: unknown) => { requireActive(); return this.executeCommand(view.pluginId, commandId, input); },
      openView: async (viewTypeId: string, options?: OpenViewOptions) => {
        requireActive();
        this.openResolvedView(viewTypeId, options ?? {}, view.pluginId);
      },
      closeView: async () => { requireActive(); this.closeView(view.id); },
    });
  }

  createRendererHostAPI(pluginId: string, generation: string): ViewHostAPI {
    const rendererView: WorkbenchViewState = {
      id: `renderer:${pluginId}`,
      pluginId,
      generation,
      viewTypeId: "renderer",
      pane: "pane",
      persistence: "session",
      context: {},
    };
    return this.createViewHostAPI(rendererView);
  }

  createPresentationHostAPI(
    pluginId: string,
    generation: string,
    actions: readonly string[],
    dispatcher?: (action: string, input?: unknown) => unknown | Promise<unknown>,
  ): PresentationHost {
    const base = this.createRendererHostAPI(pluginId, generation);
    const advertised = Object.freeze([...new Set(actions)]);
    return Object.freeze({
      ...base,
      actions: advertised,
      invoke: async (action: string, input?: unknown) => {
        if (!this.host.isGenerationActive(pluginId, generation)) {
          throw new Error("Plugin host context is no longer active");
        }
        if (!advertised.includes(action)) throw new Error(`Presentation action is not supported: ${action}`);
        if (!dispatcher) throw new Error("Presentation action dispatcher is unavailable");
        return dispatcher(action, input);
      },
    });
  }

  private reconcileHost(): void {
    const definitions = new Map(
      this.host.getViewTypes().map((view) => [viewTypeKey(view.pluginId, view.id), view]),
    );
    let views = this.state.views.flatMap((view): WorkbenchViewState[] => {
      if (this.availablePluginIds && !this.availablePluginIds.has(view.pluginId)) return [];
      const definition = definitions.get(viewTypeKey(view.pluginId, view.viewTypeId));
      if (!definition) return [view];
      return [{
        ...view,
        generation: definition.generation,
        persistence: definition.persistence ?? view.persistence,
      }];
    });

    for (const placement of this.host.getViewPlacements()) {
      const placementId = `${placement.pluginId}:${placement.id}`;
      const definition = definitions.get(viewTypeKey(placement.pluginId, placement.view));
      if (!definition
        || (this.availablePluginIds && !this.availablePluginIds.has(definition.pluginId))
        || this.state.dismissedLayoutIds.includes(placementId)) continue;
      if (views.some((view) => view.sourceLayoutId === placementId)) continue;
      views = [...views, {
        id: `placement:${placementId}`,
        pluginId: definition.pluginId,
        generation: definition.generation,
        viewTypeId: definition.id,
        pane: placement.pane,
        persistence: definition.persistence ?? "session",
        context: Object.freeze({}),
        // Kept under the legacy on-disk field until the persisted workbench
        // schema gets its own versioned migration.
        sourceLayoutId: placementId,
      }];
    }

    const activeViewByPane = { ...this.state.activeViewByPane };
    for (const pane of viewPanes) {
      const active = activeViewByPane[pane];
      if (active === HIDDEN_PANE_VIEW_ID) continue;
      if (!active || !views.some((view) => view.id === active)) {
        activeViewByPane[pane] = [...views].reverse().find((view) => view.pane === pane)?.id;
      }
    }
    this.state = freezeLayoutState({ ...this.state, views, activeViewByPane });
    this.persist();
    this.applyThemeTokens();
    this.publish();
  }

  private applyThemeTokens(): void {
    for (const token of this.appliedThemeTokens) {
      document.documentElement.style.removeProperty(token);
    }
    this.appliedThemeTokens.clear();
    const theme = document.documentElement.dataset.theme ?? "light";
    for (const contribution of this.host.getThemeTokens(theme)) {
      const explicitTokens = new Set(Object.keys(contribution.tokens));
      for (const [token, value] of Object.entries(contribution.tokens)) {
        if (!isPublicThemeTokenName(token)) continue;
        document.documentElement.style.setProperty(token, value);
        this.appliedThemeTokens.add(token);
        const canonical = canonicalThemeTokenName(token);
        if (canonical !== token && !explicitTokens.has(canonical)) {
          document.documentElement.style.setProperty(canonical, value);
          this.appliedThemeTokens.add(canonical);
        }
      }
      for (const [token, value] of Object.entries(contribution.syntax ?? {})) {
        if (!isPublicSyntaxTokenName(token)) continue;
        document.documentElement.style.setProperty(token, value);
        this.appliedThemeTokens.add(token);
      }
    }
  }

  private replaceState(next: WorkbenchLayoutState): void {
    this.state = freezeLayoutState(next);
    this.persist();
    this.publish();
  }

  private publish(): void {
    this.snapshot = this.createSnapshot();
    for (const listener of this.listeners) listener();
  }

  private createSnapshot(): WorkbenchSnapshot {
    const registeredViews = new Set(
      this.host.getViewTypes().map((view) => viewGenerationKey(view.pluginId, view.id, view.generation)),
    );
    return Object.freeze({
      ...this.state,
      views: Object.freeze(this.state.views.filter((view) =>
        registeredViews.has(viewGenerationKey(view.pluginId, view.viewTypeId, view.generation)))),
      viewTypes: this.host.getViewTypes(),
      renderers: this.host.getRenderers(),
    });
  }

  private persist(): void {
    const durableViews = this.state.views.filter((view) => view.persistence === "durable");
    const durableIds = new Set(durableViews.map((view) => view.id));
    const activeViewByPane = Object.fromEntries(
      Object.entries(this.state.activeViewByPane).filter(([, id]) => id && durableIds.has(id)),
    );
    try {
      this.storage.setItem(LAYOUT_STORAGE_KEY, JSON.stringify({
        version: WORKBENCH_LAYOUT_STATE_VERSION,
        views: durableViews,
        activeViewByPane,
        dismissedLayoutIds: this.state.dismissedLayoutIds,
      } satisfies WorkbenchLayoutState));
    } catch {
      // Persistence failures must not take down the host workbench.
    }
  }

  private findViewType(
    viewTypeId: string,
    preferredPluginId?: string,
  ): RegisteredViewType | undefined {
    const matches = this.host.getViewTypes().filter((view) => view.id === viewTypeId);
    const preferred = preferredPluginId
      ? matches.find((view) => view.pluginId === preferredPluginId)
      : undefined;
    if (preferred) return preferred;
    if (matches.length > 1) throw new Error(`Plugin view type is ambiguous: ${viewTypeId}`);
    return matches[0];
  }

  private async executeCommand(pluginId: string, commandId: string, input?: unknown): Promise<unknown> {
    const commands = this.host.getCommands().filter((command) => command.id === commandId);
    const command = commands.find((candidate) => candidate.pluginId === pluginId)
      ?? (commands.length === 1 ? commands[0] : undefined);
    if (!command) throw new Error(`Plugin command is not available: ${commandId}`);
    return command.execute(input);
  }

  private async getPluginSetting(pluginId: string, generation: string, key: string): Promise<unknown> {
    requireStorageKey(key);
    if (!this.services.getSetting) throw new Error("Plugin settings service is unavailable");
    return this.services.getSetting(pluginId, generation, key);
  }

  private async getPluginStorage(pluginId: string, generation: string, key: string, scope: "user" | "workspace"): Promise<string | null> {
    requireStorageKey(key);
    if (!this.services.getStorage) throw new Error("Plugin storage service is unavailable");
    return this.services.getStorage(pluginId, generation, key, scope);
  }

  private async setPluginStorage(pluginId: string, generation: string, key: string, value: string, scope: "user" | "workspace"): Promise<void> {
    requireStorageKey(key);
    if (typeof value !== "string" || value.length > MAX_STORAGE_VALUE_LENGTH) {
      throw new Error("Plugin storage value exceeds the supported limit");
    }
    if (!this.services.setStorage) throw new Error("Plugin storage service is unavailable");
    await this.services.setStorage(pluginId, generation, key, value, scope);
  }
}

export interface DesktopWorkbenchProps {
  host: PluginHost;
  controller?: WorkbenchController;
  inventory?: readonly ExtensionInventoryRecord[];
  services?: WorkbenchServices;
}

export function DesktopWorkbench({
  host,
  controller: suppliedController,
  inventory,
  services = EMPTY_WORKBENCH_SERVICES,
}: DesktopWorkbenchProps): JSX.Element {
  const [controller] = React.useState(() => suppliedController ?? new WorkbenchController(host, services));
  const snapshot = React.useSyncExternalStore(controller.subscribe, controller.getSnapshot);

  React.useEffect(() => controller.updateServices(services), [controller, services]);
  React.useEffect(() => () => {
    if (!suppliedController) controller.dispose();
  }, [controller, suppliedController]);
  React.useEffect(() => {
    controller.setAvailablePluginIds(new Set(
      (inventory ?? []).filter(isAvailablePlugin).map((plugin) => plugin.id),
    ));
  }, [controller, inventory]);

  const portals = viewPanes.flatMap((pane) => {
    const views = snapshot.views.filter((view) => view.pane === pane);
    if (views.length === 0) return [];
    const activeId = snapshot.activeViewByPane[pane] ?? views[views.length - 1]?.id;
    if (activeId === HIDDEN_PANE_VIEW_ID) return [];
    const active = views.find((view) => view.id === activeId) ?? views[views.length - 1];
    const target = resolvePaneTarget(pane);
    if (!active || !target) return [];
    const definition = snapshot.viewTypes.find((view) =>
      view.pluginId === active.pluginId
      && view.id === active.viewTypeId
      && view.generation === active.generation);
    if (!definition) return [];
    return [createPortal(
      <WorkbenchView
        key={active.id}
        controller={controller}
        definition={definition}
        view={active}
        siblingViews={views}
      />,
      target,
      active.id,
    )];
  });

  portals.push(createPortal(
    <StatusPresentation host={host} controller={controller} />,
    document.body,
    "plugin-workbench-status",
  ));
  return <>{portals}</>;
}

interface StatusPresentationProps {
  host: PluginHost;
  controller: WorkbenchController;
}

function StatusPresentation({ host, controller }: StatusPresentationProps): React.ReactNode {
  const statusItems = host.getStatusItems();
  const snapshot: StatusSnapshotV1 = Object.freeze({
    contractVersion: 1,
    items: Object.freeze(statusItems.map((item) => Object.freeze({
      id: statusItemId(item.pluginId, item.id),
      label: item.label,
      icon: item.icon,
      busy: false,
      disabled: false,
      actionId: item.command ? STATUS_ACTIONS.activateItem : undefined,
    }))),
  });
  const fallback = statusItems.length === 0 ? null : (
    <div className="plugin-workbench-status" role="status">
      {statusItems.map((item) => (
        <button
          key={`${item.pluginId}:${item.id}`}
          type="button"
          title={item.tooltip}
          onClick={() => {
            if (!item.command) return;
            const command = host.getCommands().find((candidate) =>
              candidate.pluginId === item.pluginId && candidate.id === item.command);
            void command?.execute();
          }}
        >
          {item.icon ? <span aria-hidden="true">{item.icon}</span> : null}
          {item.label}
        </button>
      ))}
    </div>
  );

  return (
    <PluginPresentation
      host={host}
      controller={controller}
      target="app.status"
      snapshot={snapshot}
      fallback={fallback}
      actions={[STATUS_ACTIONS.activateItem]}
      dispatchAction={async (action, input) => {
        if (action !== STATUS_ACTIONS.activateItem) throw new Error(`Unsupported status action: ${action}`);
        const id = statusActionItemId(input);
        const item = host.getStatusItems().find((candidate) => statusItemId(candidate.pluginId, candidate.id) === id);
        if (!item?.command) throw new Error(`Status item is not available: ${id}`);
        const command = host.getCommands().find((candidate) =>
          candidate.pluginId === item.pluginId
          && candidate.generation === item.generation
          && candidate.id === item.command);
        if (!command) throw new Error(`Status item command is not available: ${id}`);
        return command.execute();
      }}
    />
  );
}

function statusItemId(pluginId: string, itemId: string): string {
  return JSON.stringify([pluginId, itemId]);
}

function statusActionItemId(input: unknown): string {
  if (!isRecord(input) || typeof input.id !== "string" || input.id.length === 0) {
    throw new Error("Status item action requires a valid item id");
  }
  return input.id;
}

interface WorkbenchViewProps {
  controller: WorkbenchController;
  definition: RegisteredViewType;
  view: WorkbenchViewState;
  siblingViews: readonly WorkbenchViewState[];
}

function WorkbenchView({ controller, definition, view, siblingViews }: WorkbenchViewProps): JSX.Element {
  const View = definition.render;
  const host = React.useMemo(() => controller.createViewHostAPI(view), [controller, view]);
  return (
    <section className={`plugin-workbench-view plugin-workbench-view-${view.pane}`} data-plugin-id={view.pluginId}>
      <header className="plugin-workbench-view-header">
        <div role="tablist" aria-label="Plugin views">
          {siblingViews.map((sibling) => (
            <button
              key={sibling.id}
              role="tab"
              type="button"
              aria-selected={sibling.id === view.id}
              onClick={() => controller.activateView(sibling.id)}
            >
              {controller.getSnapshot().viewTypes.find((item) =>
                item.pluginId === sibling.pluginId && item.id === sibling.viewTypeId)?.title
                ?? sibling.viewTypeId}
            </button>
          ))}
        </div>
        <button type="button" aria-label="Close plugin view" onClick={() => void controller.closeView(view.id)}>×</button>
      </header>
      <PluginErrorBoundary
        pluginId={view.pluginId}
        generation={view.generation}
        services={controller.services}
        onUseDefault={() => void controller.closeView(view.id)}
      >
        <View host={host} context={view.context} />
      </PluginErrorBoundary>
    </section>
  );
}

export function PluginViewContent({
  controller,
  pluginId,
  viewTypeId,
  context = Object.freeze({}),
}: {
  controller: WorkbenchController;
  pluginId: string;
  viewTypeId: string;
  context?: Readonly<Record<string, unknown>>;
}): JSX.Element {
  const snapshot = React.useSyncExternalStore(controller.subscribe, controller.getSnapshot);
  const definition = snapshot.viewTypes.find((view) =>
    view.pluginId === pluginId && view.id === viewTypeId);
  const view = React.useMemo<WorkbenchViewState | undefined>(() => definition ? Object.freeze({
    id: `embedded:${pluginId}:${viewTypeId}`,
    pluginId,
    generation: definition.generation,
    viewTypeId,
    pane: "pane",
    persistence: "session",
    context: freezeContext(context),
  }) : undefined, [context, definition, pluginId, viewTypeId]);
  const host = React.useMemo(() => view ? controller.createViewHostAPI(view) : undefined, [controller, view]);
  if (!definition || !host || !view) {
    return <div className="plugin-workbench-error" role="status">Plugin view is unavailable.</div>;
  }
  const View = definition.render;
  return (
    <div
      className="plugin-view-content"
      data-wuu-component="plugin-view-content"
      data-wuu-plugin={pluginId}
      data-wuu-view={viewTypeId}
    >
      <PluginErrorBoundary
        pluginId={pluginId}
        generation={definition.generation}
        services={controller.services}
        onUseDefault={() => undefined}
      >
        <View host={host} context={view.context} />
      </PluginErrorBoundary>
    </div>
  );
}

export interface WorkbenchContentRendererProps {
  controller: WorkbenchController;
  category: RendererCategory;
  contentType: string;
  content: unknown;
  metadata?: Readonly<Record<string, unknown>>;
  fallback: React.ReactNode;
}

export function WorkbenchContentRenderer(props: WorkbenchContentRendererProps): JSX.Element {
  const renderer = props.controller.getRenderer(props.category, props.contentType);
  if (!renderer) return <>{props.fallback}</>;
  const Renderer = renderer.render;
  return (
    <PluginErrorBoundary
      pluginId={renderer.pluginId}
      generation={renderer.generation}
      services={props.controller.services}
      onUseDefault={() => undefined}
      fallback={props.fallback}
    >
      <Renderer
        content={props.content}
        metadata={props.metadata ?? {}}
        host={props.controller.createRendererHostAPI(renderer.pluginId, renderer.generation)}
      />
    </PluginErrorBoundary>
  );
}

interface PluginErrorBoundaryProps {
  pluginId: string;
  generation: string;
  services: WorkbenchServices;
  onUseDefault(): void;
  fallback?: React.ReactNode;
  children: React.ReactNode;
}

interface PluginErrorBoundaryState { error?: unknown }

export class PluginErrorBoundary extends React.Component<PluginErrorBoundaryProps, PluginErrorBoundaryState> {
  state: PluginErrorBoundaryState = {};

  static getDerivedStateFromError(error: unknown): PluginErrorBoundaryState {
    return { error };
  }

  componentDidCatch(error: unknown): void {
    this.props.services.reportError?.(this.props.pluginId, this.props.generation, error);
  }

  render(): React.ReactNode {
    if (this.state.error === undefined) return this.props.children;
    if (this.props.fallback !== undefined) return this.props.fallback;
    return (
      <div className="plugin-workbench-error" role="alert">
        <strong>This plugin view could not be displayed.</strong>
        <div>
          <button type="button" onClick={this.props.onUseDefault}>Use default UI</button>
          <button type="button" onClick={() => this.props.services.openSettings?.()}>Open settings</button>
          <button type="button" onClick={() => void this.props.services.disablePlugin?.(this.props.pluginId)}>
            Disable plugin
          </button>
        </div>
      </div>
    );
  }
}

const viewPanes: readonly ViewPane[] = ["main", "sidebar", "auxiliary", "tab", "pane", "overlay"];
const HIDDEN_PANE_VIEW_ID = "__wuu_hidden_pane__";

function resolvePaneTarget(pane: ViewPane): Element | null {
  if (pane === "overlay") return document.body;
  if (pane === "sidebar") return document.querySelector(".sidebar");
  if (pane === "auxiliary") {
    return document.querySelector(".workspace-right-panel") ?? document.querySelector(".conversation-pane");
  }
  return document.querySelector(".conversation-pane");
}

function rendererMatches(renderer: RegisteredRenderer, contentType: string): boolean {
  if (typeof renderer.match === "string") {
    if (renderer.match.endsWith("/*")) return contentType.startsWith(renderer.match.slice(0, -1));
    return renderer.match === contentType;
  }
  renderer.match.lastIndex = 0;
  return renderer.match.test(contentType);
}

function isAvailablePlugin(plugin: ExtensionInventoryRecord): boolean {
  return plugin.kind === "plugin"
    && plugin.enabled !== false
    && (plugin.state === "active" || plugin.state === "granted")
    && (plugin.approval_state === "official" || plugin.approval_state === "granted");
}

function readLayoutState(storage: StorageLike): WorkbenchLayoutState {
  try {
    const parsed: unknown = JSON.parse(storage.getItem(LAYOUT_STORAGE_KEY) ?? "null");
    if (!isRecord(parsed) || parsed.version !== WORKBENCH_LAYOUT_STATE_VERSION || !Array.isArray(parsed.views)) {
      return EMPTY_STATE;
    }
    const views = parsed.views.flatMap(parseViewState);
    const activeViewByPane = isRecord(parsed.activeViewByPane)
      ? Object.fromEntries(Object.entries(parsed.activeViewByPane).filter(([, value]) => typeof value === "string"))
      : {};
    const dismissedLayoutIds = Array.isArray(parsed.dismissedLayoutIds)
      ? parsed.dismissedLayoutIds.filter((value): value is string => typeof value === "string")
      : [];
    return freezeLayoutState({ version: WORKBENCH_LAYOUT_STATE_VERSION, views, activeViewByPane, dismissedLayoutIds });
  } catch {
    try {
      storage.removeItem(LAYOUT_STORAGE_KEY);
    } catch {
      // An unavailable storage backend behaves like an empty layout.
    }
    return EMPTY_STATE;
  }
}

function nextViewInstanceSequence(views: readonly WorkbenchViewState[]): number {
  let next = 1;
  for (const view of views) {
    const match = /:(\d+)$/.exec(view.id);
    if (match) next = Math.max(next, Number(match[1]) + 1);
  }
  return next;
}

function parseViewState(value: unknown): WorkbenchViewState[] {
  if (!isRecord(value)
    || typeof value.id !== "string"
    || typeof value.pluginId !== "string"
    || typeof value.generation !== "string"
    || typeof value.viewTypeId !== "string"
    || !isViewPane(value.pane)
    || (value.persistence !== "session" && value.persistence !== "durable")) return [];
  return [{
    id: value.id,
    pluginId: value.pluginId,
    generation: value.generation,
    viewTypeId: value.viewTypeId,
    pane: value.pane,
    persistence: value.persistence,
    context: freezeContext(isRecord(value.context) ? value.context : {}),
    sourceLayoutId: typeof value.sourceLayoutId === "string" ? value.sourceLayoutId : undefined,
  }];
}

function freezeLayoutState(state: WorkbenchLayoutState): WorkbenchLayoutState {
  return Object.freeze({
    version: WORKBENCH_LAYOUT_STATE_VERSION,
    views: Object.freeze([...state.views]),
    activeViewByPane: Object.freeze({ ...state.activeViewByPane }),
    dismissedLayoutIds: Object.freeze([...state.dismissedLayoutIds]),
  });
}

function freezeContext(context: Readonly<Record<string, unknown>> | undefined): Readonly<Record<string, unknown>> {
  return Object.freeze({ ...(context ?? {}) });
}

function viewTypeKey(pluginId: string, viewTypeId: string): string {
  return `${pluginId}\u0000${viewTypeId}`;
}

function viewGenerationKey(pluginId: string, viewTypeId: string, generation: string): string {
  return `${viewTypeKey(pluginId, viewTypeId)}\u0000${generation}`;
}

function requireStorageKey(key: string): void {
  if (!STORAGE_KEY_PATTERN.test(key)) throw new Error("Plugin storage key is invalid");
}

function isViewPane(value: unknown): value is ViewPane {
  return typeof value === "string" && (viewPanes as readonly string[]).includes(value);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
