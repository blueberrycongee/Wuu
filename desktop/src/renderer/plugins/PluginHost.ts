import type * as React from "react";

import type {
  CSSSnippet,
  LayoutContribution,
  PresentationMode,
  PresentationTarget,
  PresenterDefinition,
  RendererDefinition,
  StatusItemDefinition,
  ThemeTokens,
  ToolActivityPresenterDefinition,
  ViewPane,
  ViewPlacementContribution,
  ViewTypeDefinition,
} from "../../shared/workbench";
import { VIEW_PLACEMENT_REGIONS } from "../../shared/workbench";
import {
  isPublicSyntaxTokenName,
  isPublicThemeTokenName,
} from "../../shared/themeContract.generated";

export const PLUGIN_SLOT_IDS = [
  "sidebar.primary",
  "sidebar.footer",
  "workspace.header",
  "conversation.header",
  "conversation.message.before",
  "conversation.message.after",
  "composer.above",
  "composer.toolbar",
  "settings.plugin",
] as const;

export type PluginSlotId = (typeof PLUGIN_SLOT_IDS)[number];

export const PLUGIN_SURFACE_IDS = [
  "app.shell",
  "app.sidebar",
  "app.main",
  "app.auxiliary",
  "app.status",
  "view.launch",
  "view.conversation",
  "view.workspace",
  "view.catalog",
  "view.settings",
  "conversation.timeline",
  "conversation.message",
  "conversation.composer",
] as const;

export type PluginSurfaceId = (typeof PLUGIN_SURFACE_IDS)[number];

export interface Disposable {
  dispose(): void;
}

export type PluginSlotRenderContext = Readonly<Record<string, unknown>>;

export interface PluginSlotRegistration {
  id: string;
  order?: number;
  render(context: PluginSlotRenderContext): React.ReactNode;
}

export type PluginSurfaceMode = "replace" | "wrap";

export interface PluginSurfaceRegistration {
  id: string;
  mode: PluginSurfaceMode;
  order?: number;
  render(context: PluginSlotRenderContext, fallback: React.ReactNode): React.ReactNode;
}

export interface PluginContributionDeclarations {
  readonly slots?: readonly Readonly<{ id: string; target: PluginSlotId; order?: number; title?: string }>[];
  readonly surfaces?: readonly Readonly<{ id: string; target: PluginSurfaceId; mode: PluginSurfaceMode; order?: number; title?: string }>[];
  readonly presenters?: readonly Readonly<{ id: string; target: PresentationTarget; mode: PresentationMode; priority?: number; title?: string }>[];
}

export interface PluginCommandRegistration {
  id: string;
  title: string;
  order?: number;
  execute(input?: unknown): unknown | Promise<unknown>;
}

export interface PluginStyleRegistration {
  id: string;
  css: string;
  order?: number;
}

export interface PluginLocaleRegistration {
  id: string;
  locale: string;
  entries: Readonly<Record<string, string>>;
  order?: number;
}

export interface PluginGenerationApi {
  /** The React runtime owned by the Wuu renderer. Plugin bundles should use this object. */
  readonly react: typeof React;
  readonly pluginId: string;
  readonly generation: string;
  invokeRuntime(method: string, input?: unknown): Promise<unknown>;
  onHostEvent(handler: (event: unknown) => void): Disposable;
  registerSlot(slotId: PluginSlotId, contribution: PluginSlotRegistration): Disposable;
  registerSurface(surfaceId: PluginSurfaceId, contribution: PluginSurfaceRegistration): Disposable;
  registerCommand(command: PluginCommandRegistration): Disposable;
  registerStyle(style: PluginStyleRegistration): Disposable;
  registerLocale(locale: PluginLocaleRegistration): Disposable;
  registerCleanup(cleanup: () => void): Disposable;

  // Phase C — Workbench API

  /** Register a view type that the host can open in any pane. */
  registerViewType(definition: ViewTypeDefinition): Disposable;
  /** Request a default View placement in a stable host-owned region. */
  registerViewPlacement(contribution: ViewPlacementContribution): Disposable;
  /** @deprecated Use registerViewPlacement. */
  registerLayoutContribution(contribution: LayoutContribution): Disposable;
  /** Register a custom content renderer (message, tool result, document, file). */
  registerRenderer(definition: RendererDefinition): Disposable;
  /** Apply theme token overrides for a specific theme. */
  registerThemeTokens(tokens: ThemeTokens): Disposable;
  /** Inject a CSS snippet scoped to this plugin. */
  registerCSSSnippet(snippet: CSSSnippet): Disposable;
  /** Register a status bar item. */
  registerStatusItem(item: StatusItemDefinition): Disposable;
  registerPresenter(definition: PresenterDefinition): Disposable;
  registerToolActivityPresenter(definition: ToolActivityPresenterDefinition): Disposable;
}

export interface ActivatePluginGenerationOptions {
  pluginId: string;
  generation: string;
  contributions?: PluginContributionDeclarations;
  register(api: PluginGenerationApi): void | Promise<void>;
}

export interface RegisteredPluginSlotContribution extends PluginSlotRegistration {
  readonly pluginId: string;
  readonly generation: string;
}

export interface RegisteredPluginSurfaceContribution extends PluginSurfaceRegistration {
  readonly pluginId: string;
  readonly generation: string;
}

export interface RegisteredPluginCommand extends PluginCommandRegistration {
  readonly pluginId: string;
  readonly generation: string;
}

export interface RegisteredViewType extends ViewTypeDefinition {
  readonly pluginId: string;
  readonly generation: string;
  readonly order: number;
}

export interface RegisteredViewPlacement {
  readonly pluginId: string;
  readonly generation: string;
  readonly order: number;
  readonly id: string;
  readonly view: string;
  readonly pane: ViewPane;
  readonly legacy: boolean;
}

export interface RegisteredRenderer extends RendererDefinition {
  readonly pluginId: string;
  readonly generation: string;
  readonly order: number;
}

export interface RegisteredThemeTokens extends ThemeTokens {
  readonly pluginId: string;
  readonly generation: string;
  readonly id: string;
  readonly order: number;
}

export interface RegisteredStatusItem extends StatusItemDefinition {
  readonly pluginId: string;
  readonly generation: string;
  readonly order: number;
}

export interface RegisteredToolActivityPresenter extends ToolActivityPresenterDefinition {
  readonly pluginId: string;
  readonly generation: string;
}

export interface RegisteredPresenter extends PresenterDefinition {
  readonly pluginId: string;
  readonly generation: string;
  readonly mode: PresentationMode;
  readonly priority: number;
  readonly order: number;
}

export interface PluginConflictCandidate {
  readonly pluginId: string;
  readonly generation: string;
  readonly contributionId: string;
  readonly title?: string;
}

export interface PluginContributionConflict {
  readonly key: string;
  readonly kind: "surface" | "presenter";
  readonly target: string;
  readonly presentationKey?: string;
  readonly candidates: readonly PluginConflictCandidate[];
  readonly winnerPluginId: string;
}

export type PluginDiagnosticKind = "activation" | "cleanup" | "render";

export interface PluginGenerationDiagnostic {
  readonly pluginId: string;
  readonly generation: string;
  readonly kind: PluginDiagnosticKind;
  readonly message: string;
  readonly cause: unknown;
  readonly slotId?: PluginSlotId;
  readonly surfaceId?: PluginSurfaceId;
  readonly contributionId?: string;
}

export interface PluginHostOptions {
  /** Inject the renderer's existing React runtime; the host never supplies another copy. */
  react: typeof React;
  styleContainer?: HTMLElement | (() => HTMLElement | null);
  invokeRuntime?: (request: Readonly<{
    pluginId: string;
    generation: string;
    method: string;
    input?: unknown;
  }>) => Promise<unknown>;
}

interface OrderedRecord {
  readonly pluginId: string;
  readonly generation: string;
  readonly id: string;
  readonly order: number;
  removed: boolean;
}

interface SlotRecord extends OrderedRecord {
  readonly slotId: PluginSlotId;
  readonly render: PluginSlotRegistration["render"];
}

interface SurfaceRecord extends OrderedRecord {
  readonly surfaceId: PluginSurfaceId;
  readonly mode: PluginSurfaceMode;
  readonly title?: string;
  readonly render: PluginSurfaceRegistration["render"];
}

interface CommandRecord extends OrderedRecord {
  readonly title: string;
  readonly execute: PluginCommandRegistration["execute"];
}

interface StyleRecord extends OrderedRecord {
  readonly css: string;
  element?: HTMLStyleElement;
}

interface LocaleRecord extends OrderedRecord {
  readonly locale: string;
  readonly entries: Readonly<Record<string, string>>;
}

// Phase C — Workbench record types

interface ViewTypeRecord extends OrderedRecord {
  readonly definition: ViewTypeDefinition;
}

interface ViewPlacementRecord extends OrderedRecord {
  readonly view: string;
  readonly pane: ViewPane;
  readonly legacy: boolean;
}

interface RendererRecord extends OrderedRecord {
  readonly definition: RendererDefinition;
}

interface ThemeTokenRecord extends OrderedRecord {
  readonly tokens: ThemeTokens;
}

interface CSSSnippetRecord extends OrderedRecord {
  readonly snippet: CSSSnippet;
  element?: HTMLStyleElement;
}

interface StatusItemRecord extends OrderedRecord {
  readonly item: StatusItemDefinition;
}

interface PresenterRecord extends OrderedRecord {
  readonly target: PresentationTarget;
  readonly key?: string;
  readonly mode: PresentationMode;
  readonly title?: string;
  readonly render: PresenterDefinition["render"];
  readonly compatibilityRender?: ToolActivityPresenterDefinition["render"];
}

interface GenerationState {
  readonly pluginId: string;
  readonly generation: string;
  readonly declaredContributions?: PluginContributionDeclarations;
  readonly slots: SlotRecord[];
  readonly surfaces: SurfaceRecord[];
  readonly commands: CommandRecord[];
  readonly styles: StyleRecord[];
  readonly locales: LocaleRecord[];
  // Phase C — Workbench
  readonly views: ViewTypeRecord[];
  readonly viewPlacements: ViewPlacementRecord[];
  readonly renderers: RendererRecord[];
  readonly themeTokens: ThemeTokenRecord[];
  readonly cssSnippets: CSSSnippetRecord[];
  readonly statusItems: StatusItemRecord[];
  readonly toolActivityPresenters: PresenterRecord[];
  readonly registrationKeys: Set<string>;
  readonly hostEventHandlers: Set<(event: unknown) => void>;
  readonly teardown: Disposable[];
  acceptingRegistrations: boolean;
  active: boolean;
  disposed: boolean;
}

interface PendingActivation {
  readonly state: GenerationState;
  cancelled: boolean;
}

const EMPTY_SLOT_SNAPSHOT: readonly RegisteredPluginSlotContribution[] = Object.freeze([]);
const EMPTY_SURFACE_SNAPSHOT: readonly RegisteredPluginSurfaceContribution[] = Object.freeze([]);
const EMPTY_COMMAND_SNAPSHOT: readonly RegisteredPluginCommand[] = Object.freeze([]);
const EMPTY_VIEW_SNAPSHOT: readonly RegisteredViewType[] = Object.freeze([]);
const EMPTY_VIEW_PLACEMENT_SNAPSHOT: readonly RegisteredViewPlacement[] = Object.freeze([]);
const EMPTY_RENDERER_SNAPSHOT: readonly RegisteredRenderer[] = Object.freeze([]);
const EMPTY_THEME_TOKEN_SNAPSHOT: readonly RegisteredThemeTokens[] = Object.freeze([]);
const EMPTY_STATUS_ITEM_SNAPSHOT: readonly RegisteredStatusItem[] = Object.freeze([]);
const EMPTY_PRESENTER_SNAPSHOT: readonly RegisteredPresenter[] = Object.freeze([]);
const EMPTY_TOOL_ACTIVITY_PRESENTER_SNAPSHOT: readonly RegisteredToolActivityPresenter[] = Object.freeze([]);
const EMPTY_LOCALE_SNAPSHOT: Readonly<Record<string, string>> = Object.freeze({});
const EMPTY_DIAGNOSTIC_SNAPSHOT: readonly PluginGenerationDiagnostic[] = Object.freeze([]);

export class PluginGenerationSupersededError extends Error {
  constructor(pluginId: string, generation: string) {
    super(`Plugin generation ${pluginId}@${generation} was superseded before activation`);
    this.name = "PluginGenerationSupersededError";
  }
}

/**
 * Owns renderer plugin registrations and swaps one active generation per plugin.
 * This lifecycle boundary provides cleanup and startup recovery, not strong isolation.
 */
export class PluginHost {
  readonly react: typeof React;

  private readonly styleContainer?: PluginHostOptions["styleContainer"];
  private readonly runtimeInvoker?: PluginHostOptions["invokeRuntime"];
  private readonly activeGenerations = new Map<string, GenerationState>();
  private readonly pendingActivations = new Map<string, PendingActivation>();
  private readonly slotSnapshots = new Map<PluginSlotId, readonly RegisteredPluginSlotContribution[]>();
  private readonly slotListeners = new Map<PluginSlotId, Set<() => void>>();
  private readonly surfaceSnapshots = new Map<PluginSurfaceId, readonly RegisteredPluginSurfaceContribution[]>();
  private readonly surfaceListeners = new Map<PluginSurfaceId, Set<() => void>>();
  private readonly listeners = new Set<() => void>();
  private readonly localeSnapshots = new Map<string, Readonly<Record<string, string>>>();
  private readonly diagnostics = new Map<string, PluginGenerationDiagnostic[]>();
  private readonly presenterQuerySnapshots = new Map<string, readonly RegisteredPresenter[]>();
  private conflictPreferences: Readonly<Record<string, string>> = Object.freeze({});
  private conflictSnapshot: readonly PluginContributionConflict[] = Object.freeze([]);
  private commandSnapshot: readonly RegisteredPluginCommand[] = EMPTY_COMMAND_SNAPSHOT;
  private viewSnapshot: readonly RegisteredViewType[] = EMPTY_VIEW_SNAPSHOT;
  private viewPlacementSnapshot: readonly RegisteredViewPlacement[] = EMPTY_VIEW_PLACEMENT_SNAPSHOT;
  private rendererSnapshot: readonly RegisteredRenderer[] = EMPTY_RENDERER_SNAPSHOT;
  private themeTokenSnapshot: readonly RegisteredThemeTokens[] = EMPTY_THEME_TOKEN_SNAPSHOT;
  private statusItemSnapshot: readonly RegisteredStatusItem[] = EMPTY_STATUS_ITEM_SNAPSHOT;
  private presenterSnapshot: readonly RegisteredPresenter[] = EMPTY_PRESENTER_SNAPSHOT;
  private toolActivityPresenterSnapshot: readonly RegisteredToolActivityPresenter[] = EMPTY_TOOL_ACTIVITY_PRESENTER_SNAPSHOT;

  constructor(options: PluginHostOptions) {
    this.react = options.react;
    this.styleContainer = options.styleContainer;
    this.runtimeInvoker = options.invokeRuntime;
  }

  async activateGeneration(options: ActivatePluginGenerationOptions): Promise<Disposable> {
    const pluginId = requireNonEmpty(options.pluginId, "plugin id");
    const generation = requireNonEmpty(options.generation, "plugin generation");
    const state = createGenerationState(pluginId, generation, options.contributions);
    const pending: PendingActivation = { state, cancelled: false };

    const previousPending = this.pendingActivations.get(pluginId);
    if (previousPending) {
      previousPending.cancelled = true;
    }
    this.pendingActivations.set(pluginId, pending);

    try {
      await options.register(this.createGenerationApi(state));
      state.acceptingRegistrations = false;
      this.assertDeclaredContributionsRegistered(state);
      this.assertViewPlacementTargets(state);
      this.assertToolActivityPresenterOwnership(state);
    } catch (error: unknown) {
      state.acceptingRegistrations = false;
      if (this.pendingActivations.get(pluginId) === pending) {
        this.pendingActivations.delete(pluginId);
      }
      this.addDiagnostic({
        pluginId,
        generation,
        kind: "activation",
        message: `Plugin generation registration failed: ${errorMessage(error)}`,
        cause: error,
      });
      this.disposeGeneration(state);
      throw error;
    }

    if (pending.cancelled || this.pendingActivations.get(pluginId) !== pending) {
      this.disposeGeneration(state);
      throw new PluginGenerationSupersededError(pluginId, generation);
    }

    this.pendingActivations.delete(pluginId);
    const previous = this.activeGenerations.get(pluginId);
    this.activeGenerations.set(pluginId, state);
    state.active = true;
    if (previous) {
      previous.active = false;
      this.disposeGeneration(previous);
    }
    this.refreshPublicState();

    return createDisposable(() => {
      if (this.activeGenerations.get(pluginId) === state) {
        this.unload(pluginId);
      }
    });
  }

  disable(pluginId: string): void {
    this.unload(pluginId);
  }

  unload(pluginId: string): void {
    const normalizedPluginId = requireNonEmpty(pluginId, "plugin id");
    const pending = this.pendingActivations.get(normalizedPluginId);
    if (pending) {
      pending.cancelled = true;
      this.pendingActivations.delete(normalizedPluginId);
    }

    const active = this.activeGenerations.get(normalizedPluginId);
    if (!active) {
      return;
    }
    this.activeGenerations.delete(normalizedPluginId);
    active.active = false;
    this.disposeGeneration(active);
    this.refreshPublicState();
  }

  getSlotSnapshot(slotId: PluginSlotId): readonly RegisteredPluginSlotContribution[] {
    return this.slotSnapshots.get(slotId) ?? EMPTY_SLOT_SNAPSHOT;
  }

  subscribeSlot(slotId: PluginSlotId, listener: () => void): () => void {
    let listeners = this.slotListeners.get(slotId);
    if (!listeners) {
      listeners = new Set();
      this.slotListeners.set(slotId, listeners);
    }
    listeners.add(listener);
    return () => {
      listeners?.delete(listener);
    };
  }

  getSurfaceSnapshot(surfaceId: PluginSurfaceId): readonly RegisteredPluginSurfaceContribution[] {
    return this.surfaceSnapshots.get(surfaceId) ?? EMPTY_SURFACE_SNAPSHOT;
  }

  subscribeSurface(surfaceId: PluginSurfaceId, listener: () => void): () => void {
    let listeners = this.surfaceListeners.get(surfaceId);
    if (!listeners) {
      listeners = new Set();
      this.surfaceListeners.set(surfaceId, listeners);
    }
    listeners.add(listener);
    return () => {
      listeners?.delete(listener);
    };
  }

  getCommands(): readonly RegisteredPluginCommand[] {
    return this.commandSnapshot;
  }

  getViewTypes(): readonly RegisteredViewType[] {
    return this.viewSnapshot;
  }

  getViewPlacements(): readonly RegisteredViewPlacement[] {
    return this.viewPlacementSnapshot;
  }

  getRenderers(category?: RendererDefinition["category"]): readonly RegisteredRenderer[] {
    if (category === undefined) {
      return this.rendererSnapshot;
    }
    return Object.freeze(this.rendererSnapshot.filter((renderer) => renderer.category === category));
  }

  getThemeTokens(theme?: string): readonly RegisteredThemeTokens[] {
    if (theme === undefined) {
      return this.themeTokenSnapshot;
    }
    return Object.freeze(this.themeTokenSnapshot.filter((tokens) => tokens.theme === theme));
  }

  getStatusItems(): readonly RegisteredStatusItem[] {
    return this.statusItemSnapshot;
  }

  getToolActivityPresenters(): readonly RegisteredToolActivityPresenter[] {
    return this.toolActivityPresenterSnapshot;
  }

  getToolActivityPresenter(key: string): RegisteredToolActivityPresenter | undefined {
    return this.getToolActivityPresenters().find((presenter) => presenter.key === key);
  }

  getPresenters(target: PresentationTarget, key?: string): readonly RegisteredPresenter[] {
    const query = `${target}\u0000${key ?? ""}\u0000${key === undefined ? "absent" : "present"}`;
    const cached = this.presenterQuerySnapshots.get(query);
    if (cached) return cached;
    const snapshot = Object.freeze(this.presenterSnapshot.filter((presenter) =>
      presenter.target === target && presenter.key === key));
    this.presenterQuerySnapshots.set(query, snapshot);
    return snapshot;
  }

  isGenerationActive(pluginId: string, generation: string): boolean {
    return this.activeGenerations.get(pluginId)?.generation === generation;
  }

  getLocaleEntries(locale: string): Readonly<Record<string, string>> {
    return this.localeSnapshots.get(locale) ?? EMPTY_LOCALE_SNAPSHOT;
  }

  subscribe(listener: () => void): () => void {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  }

  getGenerationDiagnostics(pluginId: string, generation: string): readonly PluginGenerationDiagnostic[] {
    return this.diagnostics.get(diagnosticKey(pluginId, generation)) ?? EMPTY_DIAGNOSTIC_SNAPSHOT;
  }

  getConflicts(): readonly PluginContributionConflict[] {
    return this.conflictSnapshot;
  }

  setConflictPreferences(preferences: Readonly<Record<string, string>>): void {
    this.conflictPreferences = Object.freeze({ ...preferences });
    this.refreshPublicState();
  }

  setConflictPreference(key: string, pluginId: string): void {
    this.setConflictPreferences({ ...this.conflictPreferences, [key]: pluginId });
  }

  recordRenderFailure(
    contribution: RegisteredPluginSlotContribution | RegisteredPluginSurfaceContribution,
    location: { slotId: PluginSlotId } | { surfaceId: PluginSurfaceId },
    error: unknown,
  ): void {
    const kind = "slotId" in location ? "slot" : "surface";
    this.addDiagnostic({
      pluginId: contribution.pluginId,
      generation: contribution.generation,
      kind: "render",
      message: `Plugin ${kind} contribution ${contribution.id} failed to render: ${errorMessage(error)}`,
      cause: error,
      ...location,
      contributionId: contribution.id,
    });
  }

  recordPresenterFailure(contribution: RegisteredPresenter, error: unknown): void {
    this.addDiagnostic({
      pluginId: contribution.pluginId,
      generation: contribution.generation,
      kind: "render",
      message: `Plugin presenter contribution ${contribution.id} failed to render: ${errorMessage(error)}`,
      cause: error,
      contributionId: contribution.id,
    });
  }

  publishHostEvent(event: unknown): void {
    for (const state of this.activeGenerations.values()) {
      for (const handler of state.hostEventHandlers) {
        try {
          handler(event);
        } catch (error: unknown) {
          this.addDiagnostic({
            pluginId: state.pluginId,
            generation: state.generation,
            kind: "render",
            message: `Plugin host event handler failed: ${errorMessage(error)}`,
            cause: error,
          });
        }
      }
    }
  }

  private createGenerationApi(state: GenerationState): PluginGenerationApi {
    const registerViewPlacement = (
      contribution: {
        id: string;
        view: string;
        pane: ViewPane;
        priority?: number;
        legacy: boolean;
      },
    ): Disposable => {
      this.assertAccepting(state);
      const id = this.claimRegistrationId(state, "view-placement", contribution.id);
      const record: ViewPlacementRecord = {
        pluginId: state.pluginId,
        generation: state.generation,
        id,
        order: normalizeOrder(contribution.priority),
        removed: false,
        view: contribution.legacy
          ? contribution.view.trim()
          : requireNonEmpty(contribution.view, "view placement view id"),
        pane: contribution.pane,
        legacy: contribution.legacy,
      };
      state.viewPlacements.push(record);
      return this.ownRecord(state, record);
    };

    return Object.freeze({
      react: this.react,
      pluginId: state.pluginId,
      generation: state.generation,
      invokeRuntime: async (method: string, input?: unknown) => {
        if (!this.runtimeInvoker) {
          throw new Error("Plugin runtime requests are unavailable");
        }
        if (this.activeGenerations.get(state.pluginId) !== state || state.disposed) {
          throw new Error("Plugin generation is no longer active");
        }
        return this.runtimeInvoker({
          pluginId: state.pluginId,
          generation: state.generation,
          method: requireNonEmpty(method, "plugin runtime method"),
          input,
        });
      },
      onHostEvent: (handler: (event: unknown) => void) => {
        this.assertAccepting(state);
        state.hostEventHandlers.add(handler);
        const disposable = createDisposable(() => state.hostEventHandlers.delete(handler));
        state.teardown.push(disposable);
        return disposable;
      },
      registerSlot: (slotId: PluginSlotId, contribution: PluginSlotRegistration) => {
        this.assertAccepting(state);
        const id = this.claimRegistrationId(state, `slot:${slotId}`, contribution.id);
        const order = normalizeOrder(contribution.order);
        this.assertDeclaredContribution(state, "slot", { id, target: slotId, order });
        const record: SlotRecord = {
          pluginId: state.pluginId,
          generation: state.generation,
          id,
          order,
          slotId,
          render: contribution.render,
          removed: false,
        };
        state.slots.push(record);
        return this.ownRecord(state, record);
      },
      registerSurface: (surfaceId: PluginSurfaceId, contribution: PluginSurfaceRegistration) => {
        this.assertAccepting(state);
        const id = this.claimRegistrationId(state, `surface:${surfaceId}`, contribution.id);
        const order = normalizeOrder(contribution.order);
        const mode = normalizeSurfaceMode(contribution.mode);
        this.assertDeclaredContribution(state, "surface", { id, target: surfaceId, mode, order });
        const record: SurfaceRecord = {
          pluginId: state.pluginId,
          generation: state.generation,
          id,
          order,
          surfaceId,
          mode,
          title: declaredContributionTitle(state, "surface", id),
          render: contribution.render,
          removed: false,
        };
        state.surfaces.push(record);
        return this.ownRecord(state, record);
      },
      registerCommand: (command: PluginCommandRegistration) => {
        this.assertAccepting(state);
        const id = this.claimRegistrationId(state, "command", command.id);
        const record: CommandRecord = {
          pluginId: state.pluginId,
          generation: state.generation,
          id,
          order: normalizeOrder(command.order),
          title: requireNonEmpty(command.title, "command title"),
          execute: command.execute,
          removed: false,
        };
        state.commands.push(record);
        return this.ownRecord(state, record);
      },
      registerStyle: (style: PluginStyleRegistration) => {
        this.assertAccepting(state);
        const id = this.claimRegistrationId(state, "style", style.id);
        const record: StyleRecord = {
          pluginId: state.pluginId,
          generation: state.generation,
          id,
          order: normalizeOrder(style.order),
          css: style.css,
          removed: false,
        };
        state.styles.push(record);
        return this.ownRecord(state, record, () => record.element?.remove());
      },
      registerLocale: (locale: PluginLocaleRegistration) => {
        this.assertAccepting(state);
        const normalizedLocale = requireNonEmpty(locale.locale, "locale");
        const id = this.claimRegistrationId(state, `locale:${normalizedLocale}`, locale.id);
        const record: LocaleRecord = {
          pluginId: state.pluginId,
          generation: state.generation,
          id,
          order: normalizeOrder(locale.order),
          locale: normalizedLocale,
          entries: Object.freeze({ ...locale.entries }),
          removed: false,
        };
        state.locales.push(record);
        return this.ownRecord(state, record);
      },
      registerCleanup: (cleanup: () => void) => {
        this.assertAccepting(state);
        const disposable = createDisposable(() => {
          try {
            cleanup();
          } catch (error: unknown) {
            this.addDiagnostic({
              pluginId: state.pluginId,
              generation: state.generation,
              kind: "cleanup",
              message: `Plugin cleanup failed: ${errorMessage(error)}`,
              cause: error,
            });
          }
        });
        state.teardown.push(disposable);
        return disposable;
      },

      registerViewType: (definition: ViewTypeDefinition) => {
        this.assertAccepting(state);
        const id = this.claimRegistrationId(state, "view", definition.id);
        const record: ViewTypeRecord = {
          pluginId: state.pluginId, generation: state.generation, id,
          order: 0, removed: false, definition: { ...definition, id },
        };
        state.views.push(record);
        return this.ownRecord(state, record);
      },

      registerViewPlacement: (contribution: ViewPlacementContribution) => {
        const region = normalizeViewPlacementRegion(contribution.region);
        return registerViewPlacement({
          id: contribution.id,
          view: contribution.view,
          pane: region,
          priority: contribution.priority,
          legacy: false,
        });
      },

      registerLayoutContribution: (contribution: LayoutContribution) => {
        return registerViewPlacement({
          id: contribution.id,
          view: typeof contribution.defaultView === "string"
            ? contribution.defaultView
            : "",
          pane: normalizeViewPane(contribution.pane),
          legacy: true,
        });
      },

      registerRenderer: (definition: RendererDefinition) => {
        this.assertAccepting(state);
        const id = this.claimRegistrationId(state, "renderer", definition.id);
        const record: RendererRecord = {
          pluginId: state.pluginId, generation: state.generation, id,
          order: definition.priority ?? 0, removed: false,
          definition: { ...definition, priority: definition.priority ?? 0 },
        };
        state.renderers.push(record);
        return this.ownRecord(state, record);
      },

      registerThemeTokens: (tokens: ThemeTokens) => {
        this.assertAccepting(state);
        validateThemeTokens(tokens);
        const id = this.claimRegistrationId(state, "theme", `theme:${tokens.theme}`);
        const record: ThemeTokenRecord = {
          pluginId: state.pluginId, generation: state.generation, id,
          order: 0, removed: false, tokens,
        };
        state.themeTokens.push(record);
        return this.ownRecord(state, record);
      },

      registerCSSSnippet: (snippet: CSSSnippet) => {
        this.assertAccepting(state);
        const id = this.claimRegistrationId(state, "css", snippet.id);
        const record: CSSSnippetRecord = {
          pluginId: state.pluginId, generation: state.generation, id,
          order: snippet.priority ?? 0, removed: false, snippet,
        };
        state.cssSnippets.push(record);
        return this.ownRecord(state, record, () => record.element?.remove());
      },

      registerStatusItem: (item: StatusItemDefinition) => {
        this.assertAccepting(state);
        const id = this.claimRegistrationId(state, "status", item.id);
        const record: StatusItemRecord = {
          pluginId: state.pluginId, generation: state.generation, id,
          order: item.priority ?? 0, removed: false, item,
        };
        state.statusItems.push(record);
        return this.ownRecord(state, record);
      },

      registerPresenter: (definition: PresenterDefinition) => {
        this.assertAccepting(state);
        const id = this.claimExactRegistrationId(state, "presenter", definition.id);
        const target = requireExactNonEmpty(definition.target, "presenter target");
        const key = definition.key === undefined ? undefined : requireExactNonEmpty(definition.key, "presenter key");
        if (typeof definition.render !== "function") throw new Error("Plugin presenter render must be a function");
        const mode = normalizePresentationMode(definition.mode);
        const order = normalizeOrder(definition.priority);
        this.assertDeclaredContribution(state, "presenter", { id, target, mode, priority: order });
        const record: PresenterRecord = {
          pluginId: state.pluginId, generation: state.generation, id, target, key,
          mode, order, title: declaredContributionTitle(state, "presenter", id),
          removed: false, render: definition.render,
        };
        state.toolActivityPresenters.push(record);
        return this.ownRecord(state, record);
      },

      registerToolActivityPresenter: (definition: ToolActivityPresenterDefinition) => {
        this.assertAccepting(state);
        const id = this.claimExactRegistrationId(state, "tool activity presenter", definition.id);
        const key = requireExactNonEmpty(definition.key, "tool activity presenter key");
        if (typeof definition.render !== "function") {
          throw new Error("Plugin tool activity presenter render must be a function");
        }
        if (state.toolActivityPresenters.some((record) => !record.removed && record.key === key)) {
          throw new Error(`Duplicate plugin tool activity presenter key: ${key}`);
        }
        this.assertToolActivityPresenterKeyAvailable(state.pluginId, key);
        const record: PresenterRecord = {
          pluginId: state.pluginId, generation: state.generation, id, key,
          target: "conversation.tool-activity", mode: "replace",
          order: 0, removed: false,
          render: (props) => this.react.createElement(definition.render, {
            activity: props.snapshot as never, host: props.host, fallback: props.fallback,
          }),
          compatibilityRender: definition.render,
        };
        state.toolActivityPresenters.push(record);
        return this.ownRecord(state, record);
      },
    });
  }

  private ownRecord(
    state: GenerationState,
    record: OrderedRecord,
    remove?: () => void,
  ): Disposable {
    const disposable = createDisposable(() => {
      record.removed = true;
      remove?.();
      if (state.active) {
        this.refreshPublicState();
      }
    });
    state.teardown.push(disposable);
    return disposable;
  }

  private assertAccepting(state: GenerationState): void {
    if (!state.acceptingRegistrations) {
      throw new Error(`Plugin generation ${state.pluginId}@${state.generation} is no longer registering`);
    }
  }

  private claimRegistrationId(state: GenerationState, kind: string, value: string): string {
    const id = requireNonEmpty(value, `${kind} registration id`);
    const key = `${kind}:${id}`;
    if (state.registrationKeys.has(key)) {
      throw new Error(`Duplicate plugin ${kind} registration id: ${id}`);
    }
    state.registrationKeys.add(key);
    return id;
  }

  private claimExactRegistrationId(state: GenerationState, kind: string, value: string): string {
    const id = requireExactNonEmpty(value, `${kind} registration id`);
    const key = `${kind}:${id}`;
    if (state.registrationKeys.has(key)) {
      throw new Error(`Duplicate plugin ${kind} registration id: ${id}`);
    }
    state.registrationKeys.add(key);
    return id;
  }

  private assertToolActivityPresenterKeyAvailable(pluginId: string, key: string): void {
    for (const state of this.activeGenerations.values()) {
      if (state.pluginId !== pluginId && state.toolActivityPresenters.some((record) =>
        !record.removed && record.compatibilityRender !== undefined && record.key === key)) {
        throw new Error(`Tool activity presenter key is already owned by another plugin: ${key}`);
      }
    }
    for (const pending of this.pendingActivations.values()) {
      const state = pending.state;
      if (!pending.cancelled && state.pluginId !== pluginId
        && state.toolActivityPresenters.some((record) =>
          !record.removed && record.compatibilityRender !== undefined && record.key === key)) {
        throw new Error(`Tool activity presenter key is already owned by another plugin: ${key}`);
      }
    }
  }

  private assertToolActivityPresenterOwnership(state: GenerationState): void {
    for (const presenter of state.toolActivityPresenters) {
      if (!presenter.removed && presenter.compatibilityRender !== undefined && presenter.key !== undefined) {
        this.assertToolActivityPresenterKeyAvailable(state.pluginId, presenter.key);
      }
    }
  }

  private assertDeclaredContribution(
    state: GenerationState,
    kind: "slot" | "surface" | "presenter",
    actual: Readonly<{ id: string; target: string; mode?: string; order?: number; priority?: number }>,
  ): void {
    const declarations = state.declaredContributions;
    if (!declarations) return;
    const declared = kind === "slot" ? declarations.slots
      : kind === "surface" ? declarations.surfaces
      : declarations.presenters;
    const match = declared?.find((candidate) => candidate.id === actual.id);
    if (!match) {
      throw new Error(`Plugin ${kind} registration ${actual.id} is not declared in the manifest`);
    }
    const declaredOrder = "priority" in match
      ? normalizeOrder(match.priority)
      : normalizeOrder("order" in match ? match.order : undefined);
    const actualOrder = actual.priority ?? actual.order ?? 0;
    const declaredMode = "mode" in match ? match.mode : undefined;
    if (match.target !== actual.target || declaredMode !== actual.mode || declaredOrder !== actualOrder) {
      throw new Error(`Plugin ${kind} registration ${actual.id} does not match its manifest declaration`);
    }
  }

  private assertDeclaredContributionsRegistered(state: GenerationState): void {
    const declarations = state.declaredContributions;
    if (!declarations) return;
    for (const declaration of declarations.slots ?? []) {
      if (!state.slots.some((record) => !record.removed && record.id === declaration.id)) {
        throw new Error(`Manifest slot contribution ${declaration.id} was not registered during activation`);
      }
    }
    for (const declaration of declarations.surfaces ?? []) {
      if (!state.surfaces.some((record) => !record.removed && record.id === declaration.id)) {
        throw new Error(`Manifest surface contribution ${declaration.id} was not registered during activation`);
      }
    }
    for (const declaration of declarations.presenters ?? []) {
      if (!state.toolActivityPresenters.some((record) => !record.removed && record.id === declaration.id)) {
        throw new Error(`Manifest presenter contribution ${declaration.id} was not registered during activation`);
      }
    }
  }

  private assertViewPlacementTargets(state: GenerationState): void {
    for (const placement of state.viewPlacements) {
      if (placement.removed || placement.legacy) continue;
      if (!state.views.some((view) => !view.removed && view.id === placement.view)) {
        throw new Error(
          `Plugin View placement ${placement.id} references an unregistered View: ${placement.view}`,
        );
      }
    }
  }

  private disposeGeneration(state: GenerationState): void {
    if (state.disposed) {
      return;
    }
    state.disposed = true;
    state.acceptingRegistrations = false;
    for (const disposable of [...state.teardown].reverse()) {
      disposable.dispose();
    }
  }

  private refreshPublicState(): void {
    let changed = false;
    const conflicts: PluginContributionConflict[] = [];
    const slotRecords = new Map<PluginSlotId, SlotRecord[]>();
    const surfaceRecords = new Map<PluginSurfaceId, SurfaceRecord[]>();
    const commands: CommandRecord[] = [];
    const locales: LocaleRecord[] = [];
    const styles: StyleRecord[] = [];
    const views: ViewTypeRecord[] = [];
    const viewPlacements: ViewPlacementRecord[] = [];
    const renderers: RendererRecord[] = [];
    const themeTokens: ThemeTokenRecord[] = [];
    const cssSnippets: CSSSnippetRecord[] = [];
    const statusItems: StatusItemRecord[] = [];
    const toolActivityPresenters: PresenterRecord[] = [];

    for (const state of this.activeGenerations.values()) {
      for (const record of state.slots) {
        if (!record.removed) {
          const records = slotRecords.get(record.slotId) ?? [];
          records.push(record);
          slotRecords.set(record.slotId, records);
        }
      }
      for (const record of state.surfaces) {
        if (!record.removed) {
          const records = surfaceRecords.get(record.surfaceId) ?? [];
          records.push(record);
          surfaceRecords.set(record.surfaceId, records);
        }
      }
      commands.push(...state.commands.filter((record) => !record.removed));
      locales.push(...state.locales.filter((record) => !record.removed));
      styles.push(...state.styles.filter((record) => !record.removed));
      views.push(...state.views.filter((record) => !record.removed));
      viewPlacements.push(...state.viewPlacements.filter((record) => !record.removed));
      renderers.push(...state.renderers.filter((record) => !record.removed));
      themeTokens.push(...state.themeTokens.filter((record) => !record.removed));
      cssSnippets.push(...state.cssSnippets.filter((record) => !record.removed));
      statusItems.push(...state.statusItems.filter((record) => !record.removed));
      toolActivityPresenters.push(...state.toolActivityPresenters.filter((record) => !record.removed));
    }

    for (const slotId of PLUGIN_SLOT_IDS) {
      const next = Object.freeze((slotRecords.get(slotId) ?? [])
        .sort(compareOrdered)
        .map(toPublicSlotContribution));
      const previous = this.slotSnapshots.get(slotId) ?? EMPTY_SLOT_SNAPSHOT;
      if (!sameContributions(previous, next)) {
        this.slotSnapshots.set(slotId, next);
        changed = true;
        for (const listener of this.slotListeners.get(slotId) ?? []) {
          listener();
        }
      }
    }

    for (const surfaceId of PLUGIN_SURFACE_IDS) {
      const resolved = resolveReplaceConflict(
        "surface",
        surfaceId,
        undefined,
        (surfaceRecords.get(surfaceId) ?? []).sort(compareOrdered),
        this.conflictPreferences,
      );
      if (resolved.conflict) conflicts.push(resolved.conflict);
      const next = Object.freeze(resolved.records.map(toPublicSurfaceContribution));
      const previous = this.surfaceSnapshots.get(surfaceId) ?? EMPTY_SURFACE_SNAPSHOT;
      if (!sameContributions(previous, next)) {
        this.surfaceSnapshots.set(surfaceId, next);
        changed = true;
        for (const listener of this.surfaceListeners.get(surfaceId) ?? []) {
          listener();
        }
      }
    }

    const nextCommands = Object.freeze(commands.sort(compareOrdered).map(toPublicCommand));
    if (!sameContributions(this.commandSnapshot, nextCommands)) {
      this.commandSnapshot = nextCommands;
      changed = true;
    }

    const nextViews = Object.freeze(views.sort(compareOrdered).map(toPublicViewType));
    if (!sameContributions(this.viewSnapshot, nextViews)) {
      this.viewSnapshot = nextViews;
      changed = true;
    }

    const nextViewPlacements = Object.freeze(
      viewPlacements.sort(compareOrdered).map(toPublicViewPlacement),
    );
    if (!sameContributions(this.viewPlacementSnapshot, nextViewPlacements)) {
      this.viewPlacementSnapshot = nextViewPlacements;
      changed = true;
    }

    const nextRenderers = Object.freeze(renderers.sort(compareOrdered).map(toPublicRenderer));
    if (!sameContributions(this.rendererSnapshot, nextRenderers)) {
      this.rendererSnapshot = nextRenderers;
      changed = true;
    }

    const nextThemeTokens = Object.freeze(themeTokens.sort(compareOrdered).map(toPublicThemeTokens));
    if (!sameContributions(this.themeTokenSnapshot, nextThemeTokens)) {
      this.themeTokenSnapshot = nextThemeTokens;
      changed = true;
    }

    const nextStatusItems = Object.freeze(statusItems.sort(compareOrdered).map(toPublicStatusItem));
    if (!sameContributions(this.statusItemSnapshot, nextStatusItems)) {
      this.statusItemSnapshot = nextStatusItems;
      changed = true;
    }

    const resolvedPresenters = resolvePresenterConflicts(
      toolActivityPresenters.sort(compareOrdered),
      this.conflictPreferences,
    );
    conflicts.push(...resolvedPresenters.conflicts);
    const nextPresenters = Object.freeze(resolvedPresenters.records.map(toPublicPresenter));
    if (!sameContributions(this.presenterSnapshot, nextPresenters)) {
      this.presenterSnapshot = nextPresenters;
      this.presenterQuerySnapshots.clear();
      changed = true;
    }

    const nextToolActivityPresenters = Object.freeze(toolActivityPresenters
      .filter((record) => record.compatibilityRender !== undefined && record.key !== undefined)
      .map((record) => Object.freeze({
        pluginId: record.pluginId,
        generation: record.generation,
        id: record.id,
        key: record.key!,
        render: record.compatibilityRender!,
      })));
    if (!sameContributions(this.toolActivityPresenterSnapshot, nextToolActivityPresenters)) {
      this.toolActivityPresenterSnapshot = nextToolActivityPresenters;
      changed = true;
    }

    const nextConflicts = Object.freeze(conflicts.map((conflict) => Object.freeze(conflict)));
    if (JSON.stringify(this.conflictSnapshot) !== JSON.stringify(nextConflicts)) {
      this.conflictSnapshot = nextConflicts;
      changed = true;
    }

    if (this.refreshLocales(locales)) {
      changed = true;
    }
    this.refreshStyles(styles);
    this.refreshCSSSnippets(cssSnippets);

    if (changed) {
      this.notifyListeners();
    }
  }

  private refreshLocales(records: LocaleRecord[]): boolean {
    const grouped = new Map<string, LocaleRecord[]>();
    for (const record of records) {
      const localeRecords = grouped.get(record.locale) ?? [];
      localeRecords.push(record);
      grouped.set(record.locale, localeRecords);
    }

    let changed = false;
    const allLocales = new Set([...this.localeSnapshots.keys(), ...grouped.keys()]);
    for (const locale of allLocales) {
      const merged: Record<string, string> = {};
      for (const record of (grouped.get(locale) ?? []).sort(compareOrdered)) {
        for (const key of Object.keys(record.entries).sort(compareText)) {
          merged[key] = record.entries[key];
        }
      }
      const next = Object.freeze(merged);
      const previous = this.localeSnapshots.get(locale) ?? EMPTY_LOCALE_SNAPSHOT;
      if (!sameStringRecord(previous, next)) {
        changed = true;
        if (Object.keys(next).length === 0) {
          this.localeSnapshots.delete(locale);
        } else {
          this.localeSnapshots.set(locale, next);
        }
      }
    }
    return changed;
  }

  private refreshStyles(records: StyleRecord[]): void {
    const container = this.resolveStyleContainer();
    if (!container) {
      return;
    }
    for (const record of records.sort(compareOrdered)) {
      let element = record.element;
      if (!element) {
        element = container.ownerDocument.createElement("style");
        element.dataset.wuuPluginId = record.pluginId;
        element.dataset.wuuPluginGeneration = record.generation;
        element.dataset.wuuPluginStyle = record.id;
        element.textContent = record.css;
        record.element = element;
      }
      container.appendChild(element);
    }
  }

  private refreshCSSSnippets(records: CSSSnippetRecord[]): void {
    const container = this.resolveStyleContainer();
    if (!container) {
      return;
    }
    for (const record of records.sort(compareOrdered)) {
      let element = record.element;
      if (!element) {
        element = container.ownerDocument.createElement("style");
        element.dataset.wuuPluginId = record.pluginId;
        element.dataset.wuuPluginGeneration = record.generation;
        element.dataset.wuuPluginCssSnippet = record.id;
        element.textContent = record.snippet.css;
        record.element = element;
      }
      container.appendChild(element);
    }
  }

  private resolveStyleContainer(): HTMLElement | null {
    if (typeof this.styleContainer === "function") {
      return this.styleContainer();
    }
    if (this.styleContainer) {
      return this.styleContainer;
    }
    return typeof document === "undefined" ? null : document.head;
  }

  private addDiagnostic(diagnostic: PluginGenerationDiagnostic): void {
    const key = diagnosticKey(diagnostic.pluginId, diagnostic.generation);
    const diagnostics = this.diagnostics.get(key) ?? [];
    diagnostics.push(Object.freeze(diagnostic));
    this.diagnostics.set(key, diagnostics);
    this.notifyListeners();
  }

  private notifyListeners(): void {
    for (const listener of this.listeners) {
      listener();
    }
  }
}

function createGenerationState(
  pluginId: string,
  generation: string,
  declaredContributions?: PluginContributionDeclarations,
): GenerationState {
  return {
    pluginId,
    generation,
    declaredContributions,
    slots: [],
    surfaces: [],
    commands: [],
    styles: [],
    locales: [],
    views: [],
    viewPlacements: [],
    renderers: [],
    themeTokens: [],
    cssSnippets: [],
    statusItems: [],
    toolActivityPresenters: [],
    registrationKeys: new Set(),
    hostEventHandlers: new Set(),
    teardown: [],
    acceptingRegistrations: true,
    active: false,
    disposed: false,
  };
}

function toPublicSlotContribution(record: SlotRecord): RegisteredPluginSlotContribution {
  return Object.freeze({
    pluginId: record.pluginId,
    generation: record.generation,
    id: record.id,
    order: record.order,
    render: record.render,
  });
}

function toPublicSurfaceContribution(record: SurfaceRecord): RegisteredPluginSurfaceContribution {
  return Object.freeze({
    pluginId: record.pluginId,
    generation: record.generation,
    id: record.id,
    order: record.order,
    mode: record.mode,
    render: record.render,
  });
}

function toPublicCommand(record: CommandRecord): RegisteredPluginCommand {
  return Object.freeze({
    pluginId: record.pluginId,
    generation: record.generation,
    id: record.id,
    title: record.title,
    order: record.order,
    execute: record.execute,
  });
}

function toPublicViewType(record: ViewTypeRecord): RegisteredViewType {
  return Object.freeze({
    ...record.definition,
    pluginId: record.pluginId,
    generation: record.generation,
    id: record.id,
    order: record.order,
  });
}

function toPublicViewPlacement(record: ViewPlacementRecord): RegisteredViewPlacement {
  return Object.freeze({
    pluginId: record.pluginId,
    generation: record.generation,
    id: record.id,
    order: record.order,
    view: record.view,
    pane: record.pane,
    legacy: record.legacy,
  });
}

function toPublicRenderer(record: RendererRecord): RegisteredRenderer {
  return Object.freeze({
    ...record.definition,
    pluginId: record.pluginId,
    generation: record.generation,
    id: record.id,
    order: record.order,
  });
}

function toPublicThemeTokens(record: ThemeTokenRecord): RegisteredThemeTokens {
  return Object.freeze({
    ...record.tokens,
    tokens: Object.freeze({ ...record.tokens.tokens }),
    syntax: record.tokens.syntax === undefined
      ? undefined
      : Object.freeze({ ...record.tokens.syntax }),
    pluginId: record.pluginId,
    generation: record.generation,
    id: record.id,
    order: record.order,
  });
}

function toPublicStatusItem(record: StatusItemRecord): RegisteredStatusItem {
  return Object.freeze({
    ...record.item,
    pluginId: record.pluginId,
    generation: record.generation,
    id: record.id,
    order: record.order,
  });
}

function toPublicPresenter(record: PresenterRecord): RegisteredPresenter {
  return Object.freeze({
    pluginId: record.pluginId,
    generation: record.generation,
    id: record.id,
    target: record.target,
    key: record.key,
    mode: record.mode,
    priority: record.order,
    order: record.order,
    render: record.render,
  });
}

function declaredContributionTitle(
  state: GenerationState,
  kind: "surface" | "presenter",
  id: string,
): string | undefined {
  const declarations = kind === "surface"
    ? state.declaredContributions?.surfaces
    : state.declaredContributions?.presenters;
  return declarations?.find((declaration) => declaration.id === id)?.title;
}

function conflictPreferenceKey(
  kind: "surface" | "presenter",
  target: string,
  presentationKey?: string,
): string {
  return kind === "surface"
    ? `surface:${target}`
    : `presenter:${target}:${presentationKey ?? ""}`;
}

function resolveReplaceConflict<T extends OrderedRecord & { mode: "replace" | "wrap"; title?: string }>(
  kind: "surface" | "presenter",
  target: string,
  presentationKey: string | undefined,
  records: T[],
  preferences: Readonly<Record<string, string>>,
): { records: T[]; conflict?: PluginContributionConflict } {
  const replacements = records.filter((record) => record.mode === "replace");
  if (replacements.length < 2) return { records };
  const key = conflictPreferenceKey(kind, target, presentationKey);
  const preferredPluginId = preferences[key];
  let winner = replacements.at(-1)!;
  if (preferredPluginId) {
    for (let index = replacements.length - 1; index >= 0; index--) {
      if (replacements[index].pluginId === preferredPluginId) {
        winner = replacements[index];
        break;
      }
    }
  }
  const resolved = records.filter((record) => record !== winner);
  resolved.push(winner);
  return {
    records: resolved,
    conflict: {
      key,
      kind,
      target,
      ...(presentationKey === undefined ? {} : { presentationKey }),
      candidates: Object.freeze(replacements.map((record) => Object.freeze({
        pluginId: record.pluginId,
        generation: record.generation,
        contributionId: record.id,
        ...(record.title ? { title: record.title } : {}),
      }))),
      winnerPluginId: winner.pluginId,
    },
  };
}

function resolvePresenterConflicts(
  records: PresenterRecord[],
  preferences: Readonly<Record<string, string>>,
): { records: PresenterRecord[]; conflicts: PluginContributionConflict[] } {
  const groups = new Map<string, PresenterRecord[]>();
  for (const record of records) {
    const key = `${record.target}\u0000${record.key ?? ""}\u0000${record.key === undefined ? "absent" : "present"}`;
    const group = groups.get(key) ?? [];
    group.push(record);
    groups.set(key, group);
  }
  let resolved = [...records];
  const conflicts: PluginContributionConflict[] = [];
  for (const group of groups.values()) {
    const first = group[0];
    const result = resolveReplaceConflict(
      "presenter",
      first.target,
      first.key,
      group,
      preferences,
    );
    if (!result.conflict) continue;
    conflicts.push(result.conflict);
    const winner = result.records.at(-1)!;
    resolved = resolved.filter((record) => record !== winner);
    resolved.push(winner);
  }
  return { records: resolved, conflicts };
}

function compareOrdered(left: OrderedRecord, right: OrderedRecord): number {
  return left.order - right.order
    || compareText(left.pluginId, right.pluginId)
    || compareText(left.id, right.id);
}

function compareText(left: string, right: string): number {
  return left < right ? -1 : left > right ? 1 : 0;
}

function normalizeOrder(order: number | undefined): number {
  if (order === undefined) {
    return 0;
  }
  if (!Number.isFinite(order)) {
    throw new Error("Plugin registration order must be a finite number");
  }
  return order;
}

function normalizeViewPlacementRegion(region: unknown): ViewPlacementContribution["region"] {
  if (typeof region !== "string"
    || !(VIEW_PLACEMENT_REGIONS as readonly string[]).includes(region)) {
    throw new Error(`Unsupported plugin View placement region: ${String(region)}`);
  }
  return region as ViewPlacementContribution["region"];
}

function normalizeViewPane(pane: unknown): ViewPane {
  if (pane !== "main" && pane !== "sidebar" && pane !== "auxiliary"
    && pane !== "overlay" && pane !== "tab" && pane !== "pane") {
    throw new Error(`Unsupported legacy plugin View pane: ${String(pane)}`);
  }
  return pane;
}

function normalizeSurfaceMode(mode: PluginSurfaceMode): PluginSurfaceMode {
  if (mode !== "replace" && mode !== "wrap") {
    throw new Error(`Unsupported plugin surface mode: ${String(mode)}`);
  }
  return mode;
}

function normalizePresentationMode(mode: PresentationMode | undefined): PresentationMode {
  if (mode === undefined) return "replace";
  if (mode !== "replace" && mode !== "wrap") {
    throw new Error(`Unsupported presenter mode: ${String(mode)}`);
  }
  return mode;
}

function requireNonEmpty(value: string, label: string): string {
  const normalized = value.trim();
  if (!normalized) {
    throw new Error(`Plugin ${label} must not be empty`);
  }
  return normalized;
}

function requireExactNonEmpty(value: string, label: string): string {
  if (typeof value !== "string" || value.trim().length === 0) {
    throw new Error(`Plugin ${label} must not be empty`);
  }
  return value;
}

function validateThemeTokens(contribution: ThemeTokens): void {
  requireNonEmpty(contribution.theme, "theme id");
  if (contribution.base !== "light" && contribution.base !== "dark") {
    throw new Error(`Plugin theme base must be light or dark: ${String(contribution.base)}`);
  }
  const tokens = Object.entries(contribution.tokens);
  const syntax = Object.entries(contribution.syntax ?? {});
  if (tokens.length === 0 && syntax.length === 0) {
    throw new Error("Plugin theme token contribution must not be empty");
  }
  for (const [name, value] of tokens) {
    if (!isPublicThemeTokenName(name)) {
      throw new Error(`Unsupported plugin theme token: ${name}`);
    }
    requireExactNonEmpty(value, `theme token ${name}`);
  }
  for (const [name, value] of syntax) {
    if (!isPublicSyntaxTokenName(name)) {
      throw new Error(`Unsupported plugin syntax token: ${name}`);
    }
    requireExactNonEmpty(value, `syntax token ${name}`);
  }
}

function sameContributions(
  previous: readonly { pluginId: string; generation: string; id: string; order?: number }[],
  next: readonly { pluginId: string; generation: string; id: string; order?: number }[],
): boolean {
  return previous.length === next.length && previous.every((item, index) => {
    const candidate = next[index];
    return candidate !== undefined
      && item.pluginId === candidate.pluginId
      && item.generation === candidate.generation
      && item.id === candidate.id
      && item.order === candidate.order;
  });
}

function sameStringRecord(
  previous: Readonly<Record<string, string>>,
  next: Readonly<Record<string, string>>,
): boolean {
  const previousKeys = Object.keys(previous);
  const nextKeys = Object.keys(next);
  return previousKeys.length === nextKeys.length
    && previousKeys.every((key) => previous[key] === next[key]);
}

function diagnosticKey(pluginId: string, generation: string): string {
  return `${pluginId}\u0000${generation}`;
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

function createDisposable(dispose: () => void): Disposable {
  let disposed = false;
  return Object.freeze({
    dispose: () => {
      if (disposed) {
        return;
      }
      disposed = true;
      dispose();
    },
  });
}
