/**
 * @wuu/plugin-sdk
 *
 * Public SDK for building Wuu plugins. This package provides TypeScript
 * types, interfaces, and helper utilities that plugin developers depend on.
 * Plugins must not import Wuu private source; all public API surfaces
 * through this package.
 *
 * @module
 */

// ---------------------------------------------------------------------------
// Core workbench types
// ---------------------------------------------------------------------------

/** Unique identifier for a view type. */
export type ViewTypeId = string;
/** Where a view instance appears. */
export type ViewPane = "main" | "sidebar" | "auxiliary" | "overlay" | "tab" | "pane";
export type ViewPersistence = "session" | "durable";

export interface ViewTypeDefinition {
  id: ViewTypeId;
  title: string;
  icon?: string;
  defaultPane?: ViewPane;
  persistence?: ViewPersistence;
  render: unknown; // React.ComponentType<ViewRenderProps> — opaque in SDK
}

export interface ViewRenderProps {
  host: ViewHostAPI;
  context: Readonly<Record<string, unknown>>;
}

export interface ViewHostAPI {
  getStorage(key: string): Promise<string | null>;
  setStorage(key: string, value: string): Promise<void>;
  getSetting(key: string): Promise<unknown>;
  executeCommand(commandId: string, input?: unknown): Promise<unknown>;
  openView(viewTypeId: ViewTypeId, options?: OpenViewOptions): Promise<void>;
  closeView(): Promise<void>;
}

export interface OpenViewOptions {
  pane?: ViewPane;
  context?: Readonly<Record<string, unknown>>;
  persistence?: ViewPersistence;
  reveal?: boolean;
}

export interface LayoutContribution {
  id: string;
  parentId: string;
  pane: ViewPane;
  size?: number;
  minSize?: number;
  defaultView?: ViewTypeId;
}

export type RendererCategory = "message" | "tool-result" | "document" | "file-preview";

export interface RendererDefinition {
  id: string;
  category: RendererCategory;
  match: string | RegExp;
  priority?: number;
  render: unknown; // React.ComponentType<RendererProps> — opaque in SDK
}

export interface ThemeTokens {
  theme: string;
  base: string;
  tokens: Record<string, string>;
  syntax?: Record<string, string>;
}

export interface CSSSnippet {
  id: string;
  css: string;
  priority?: number;
}

export interface PluginSettingDefinition {
  type: "boolean" | "string" | "number" | "enum";
  title: string;
  description?: string;
  default: unknown;
  enum?: string[];
  scope: "user" | "workspace";
  apply: "live" | "restart";
}

export interface PluginStorageAPI {
  get(key: string): Promise<string | null>;
  set(key: string, value: string): Promise<void>;
  delete(key: string): Promise<void>;
  keys(): Promise<string[]>;
}

export interface PluginSettingsAPI {
  get(key: string): Promise<unknown>;
  getAll(): Promise<Record<string, unknown>>;
}

export interface CommandDefinition {
  id: string;
  title: string;
  description?: string;
  shortcut?: string;
  contexts?: string[];
  execute(input?: unknown): unknown | Promise<unknown>;
}

export interface StatusItemDefinition {
  id: string;
  label: string;
  icon?: string;
  tooltip?: string;
  command?: string;
  priority?: number;
}

export interface LocaleDefinition {
  locale: string;
  fallback?: string[];
  entries: Record<string, string>;
}

// ---------------------------------------------------------------------------
// Plugin generation API
// ---------------------------------------------------------------------------

export interface PluginGenerationApi {
  /** Host-owned React runtime. Executable desktop bundles must use this instance. */
  readonly react: HostReact;
  readonly pluginId: string;
  readonly generation: string;
  registerSlot(slotId: string, contribution: SlotRegistration): Disposable;
  registerSurface(surfaceId: string, contribution: SurfaceRegistration): Disposable;
  registerCommand(command: CommandRegistration): Disposable;
  registerStyle(style: StyleRegistration): Disposable;
  registerLocale(locale: LocaleRegistration): Disposable;
  registerCleanup(cleanup: () => void): Disposable;
  registerViewType(definition: ViewTypeDefinition): Disposable;
  registerLayoutContribution(contribution: LayoutContribution): Disposable;
  registerRenderer(definition: RendererDefinition): Disposable;
  registerThemeTokens(tokens: ThemeTokens): Disposable;
  registerCSSSnippet(snippet: CSSSnippet): Disposable;
  registerStatusItem(item: StatusItemDefinition): Disposable;
}

/** Minimal type for the host-owned React instance; plugins never bundle React. */
export interface HostReact {
  readonly Fragment: unknown;
  createElement(
    type: string | ((props: Readonly<Record<string, unknown>>) => unknown),
    props: Readonly<Record<string, unknown>> | null,
    ...children: unknown[]
  ): unknown;
}

// ---------------------------------------------------------------------------
// Executable runtime contract
// ---------------------------------------------------------------------------

export interface RuntimeInitializeParams {
  protocol_version: number;
  plugin_id: string;
  plugin_root: string;
  project_root: string;
  wuu_home: string;
}

export interface RuntimeInitializeResult {
  hooks: string[];
  tools?: ReadonlyArray<Record<string, unknown>>;
}

export interface RuntimePlugin {
  initialize(params: RuntimeInitializeParams): RuntimeInitializeResult | Promise<RuntimeInitializeResult>;
}

export interface RuntimeRequest {
  id: string;
  method: string;
  params: RuntimeInitializeParams;
}

export type RuntimeResponse =
  | { id: string; result: RuntimeInitializeResult | null }
  | { id: string; error: { message: string } };

export interface Disposable {
  dispose(): void;
}

export interface SlotRegistration {
  id: string;
  order?: number;
  render(context: Readonly<Record<string, unknown>>): unknown;
}

export type SurfaceMode = "replace" | "wrap";

export interface SurfaceRegistration {
  id: string;
  mode: SurfaceMode;
  order?: number;
  render(context: Readonly<Record<string, unknown>>, fallback: unknown): unknown;
}

export interface CommandRegistration {
  id: string;
  title: string;
  order?: number;
  execute(input?: unknown): unknown | Promise<unknown>;
}

export interface StyleRegistration {
  id: string;
  css: string;
  order?: number;
}

export interface LocaleRegistration {
  id: string;
  locale: string;
  entries: Readonly<Record<string, string>>;
  order?: number;
}

// ---------------------------------------------------------------------------
// Plugin manifest helpers
// ---------------------------------------------------------------------------

export function validateManifest(manifest: Record<string, unknown>): string[] {
  const errors: string[] = [];
  if (!manifest.id || typeof manifest.id !== "string") {
    errors.push("manifest.id is required (string)");
  }
  if (manifest.schema_version !== 1) {
    errors.push("manifest.schema_version must be 1");
  }
  if (!manifest.version || typeof manifest.version !== "string") {
    errors.push("manifest.version is required (string)");
  }
  return errors;
}

export function createManifest(options: {
  id: string;
  name?: string;
  version?: string;
  description?: string;
}): Record<string, unknown> {
  return {
    schema_version: 1,
    id: options.id,
    name: options.name ?? options.id,
    version: options.version ?? "0.1.0",
    description: options.description ?? `A Wuu plugin: ${options.id}`,
  };
}

