/**
 * Workbench API — Phase C
 *
 * Stable contract for desktop workbench customization. Plugins use these
 * types to contribute views, layouts, renderers, theme tokens, CSS
 * snippets, settings, and namespaced storage without importing Wuu
 * private source.
 *
 * This module defines the public surface. The PluginHost consumes these
 * types to wire contributions into the renderer; plugins receive a
 * PluginGenerationApi that exposes registration methods for each category.
 */

import type * as React from "react";

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

/** Unique identifier for a view type registered by a plugin. */
export type ViewTypeId = string;

/** Where a view instance appears in the workbench. */
export type ViewPane =
  | "main"
  | "sidebar"
  | "auxiliary"
  | "overlay"
  | "tab"
  | "pane";

/** Persistence policy for a view instance. */
export type ViewPersistence = "session" | "durable";

/**
 * A plugin-registered view type. Each view type defines a React component
 * plus metadata. The host opens instances of this type in a pane.
 */
export interface ViewTypeDefinition {
  /** Stable dotted identifier, e.g. "my-plugin.dashboard". */
  id: ViewTypeId;
  /** User-facing title for the view tab or header. */
  title: string;
  /** Icon identifier from the host icon set, or a React node. */
  icon?: string;
  /** Default pane when the host opens this view without a specific target. */
  defaultPane?: ViewPane;
  /** Whether this view's state survives a session restart. */
  persistence?: ViewPersistence;
  /** React component that receives host context and renders the view. */
  render: React.ComponentType<ViewRenderProps>;
}

/** Context the host passes to every view component on render. */
export interface ViewRenderProps {
  /** Opaque host API for controlled interactions. */
  host: ViewHostAPI;
  /** Immutable context snapshot for this view instance. */
  context: Readonly<Record<string, unknown>>;
}

/** Controlled API surface the host exposes to view components. */
export interface ViewHostAPI {
  /** Read plugin-scoped namespaced storage. */
  getStorage(key: string): Promise<string | null>;
  /** Write plugin-scoped namespaced storage. */
  setStorage(key: string, value: string): Promise<void>;
  /** Read a plugin setting value. */
  getSetting(key: string): Promise<unknown>;
  /** Execute a registered command. */
  executeCommand(commandId: string, input?: unknown): Promise<unknown>;
  /** Request the host to open a view instance. */
  openView(viewTypeId: ViewTypeId, options?: OpenViewOptions): Promise<void>;
  /** Request the host to close this view instance. */
  closeView(): Promise<void>;
}

export interface OpenViewOptions {
  pane?: ViewPane;
  context?: Readonly<Record<string, unknown>>;
}

/** Current on-disk workbench state schema. */
export const WORKBENCH_LAYOUT_STATE_VERSION = 1 as const;

/** A host-owned view instance. Plugins never receive the mutable instance. */
export interface WorkbenchViewState {
  id: string;
  pluginId: string;
  generation: string;
  viewTypeId: ViewTypeId;
  pane: ViewPane;
  persistence: ViewPersistence;
  context: Readonly<Record<string, unknown>>;
  sourceLayoutId?: string;
}

/** Versioned, shell-independent state persisted by the desktop workbench. */
export interface WorkbenchLayoutState {
  version: typeof WORKBENCH_LAYOUT_STATE_VERSION;
  views: readonly WorkbenchViewState[];
  activeViewByPane: Readonly<Partial<Record<ViewPane, string>>>;
  dismissedLayoutIds: readonly string[];
}

// ---------------------------------------------------------------------------
// Layout
// ---------------------------------------------------------------------------

/**
 * A plugin's contribution to the workbench layout tree. The host merges
 * layout contributions into the user's persisted layout state. Plugins
 * declare where they want panes or splits; the host resolves conflicts
 * and preserves user overrides.
 */
export interface LayoutContribution {
  /** Stable layout node identifier. */
  id: string;
  /** Parent node id, or "root" for top-level. */
  parentId: string;
  /** Pane type for this node. */
  pane: ViewPane;
  /** Preferred size ratio (0-1) relative to sibling nodes. */
  size?: number;
  /** Minimum size in pixels. */
  minSize?: number;
  /** View type to open in this pane by default. */
  defaultView?: ViewTypeId;
}

// ---------------------------------------------------------------------------
// Renderer
// ---------------------------------------------------------------------------

/** Categories of content that plugins can provide custom renderers for. */
export type RendererCategory =
  | "message"
  | "tool-result"
  | "document"
  | "file-preview";

/** Plugin-registered content renderer. */
export interface RendererDefinition {
  /** Stable identifier. */
  id: string;
  /** Category this renderer handles. */
  category: RendererCategory;
  /** MIME type or content-type pattern this renderer matches. */
  match: string | RegExp;
  /** Priority — higher values override lower-priority renderers. */
  priority?: number;
  /** React component that renders the content. */
  render: React.ComponentType<RendererProps>;
}

/** Props passed to custom renderer components. */
export interface RendererProps {
  /** The raw content to render. Structure depends on category. */
  content: unknown;
  /** Additional metadata from the host. */
  metadata: Readonly<Record<string, unknown>>;
  /** Host API for controlled interactions. */
  host: ViewHostAPI;
}

// ---------------------------------------------------------------------------
// Theme tokens
// ---------------------------------------------------------------------------

/**
 * Complete design-language token categories. Every token is a CSS custom
 * property that the host defines and theme plugins override.
 *
 * Tokens are organized by category for the Phase C contract. The host
 * owns the CSS variable namespace; plugins contribute overrides per
 * theme (light/dark).
 */
export type ThemeTokenCategory =
  | "color"
  | "typography"
  | "spacing"
  | "density"
  | "radius"
  | "border"
  | "elevation"
  | "motion"
  | "syntax"
  | "content";

/**
 * Theme token contribution from a plugin. Tokens are CSS custom property
 * key-value pairs. The host applies them in priority order on top of the
 * built-in theme.
 */
export interface ThemeTokens {
  /** Theme identifier this contribution targets (e.g. "dark", "light"). */
  theme: string;
  /** Base theme to inherit from ("light" or "dark"). */
  base: string;
  /** CSS custom property overrides: "--wuu-color-bg": "#1a1a2e". */
  tokens: Record<string, string>;
  /** Syntax highlighting overrides. */
  syntax?: Record<string, string>;
}

/**
 * CSS snippet contributed by a plugin. Snippets are scoped to the plugin
 * and injected into the document. They should use CSS custom properties
 * for theming and stable data attributes for targeting host elements.
 *
 * Snippets are a high-trust feature: they run in the same document as the
 * host UI. Plugins must not depend on internal React state or private
 * class names, which are not part of the compatibility contract.
 */
export interface CSSSnippet {
  /** Stable snippet identifier. */
  id: string;
  /** Raw CSS to inject. Scoped via a data-plugin attribute automatically. */
  css: string;
  /** Priority — higher values are injected later (higher specificity). */
  priority?: number;
}

// ---------------------------------------------------------------------------
// Settings & storage
// ---------------------------------------------------------------------------

/**
 * Plugin setting definition. Mirrors the Go-side SettingDefinition in the
 * plugin manifest but typed for the TypeScript workbench API.
 */
export interface PluginSettingDefinition {
  type: "boolean" | "string" | "number" | "enum";
  title: string;
  description?: string;
  default: unknown;
  enum?: string[];
  scope: "user" | "workspace";
  apply: "live" | "restart";
}

/**
 * Namespaced storage API. Plugins read and write key-value pairs scoped
 * to their plugin ID. Storage is durable and survives restarts.
 */
export interface PluginStorageAPI {
  get(key: string): Promise<string | null>;
  set(key: string, value: string): Promise<void>;
  delete(key: string): Promise<void>;
  keys(): Promise<string[]>;
}

/**
 * Plugin settings API. Plugins read their own setting values as defined
 * in the manifest settings section.
 */
export interface PluginSettingsAPI {
  get(key: string): Promise<unknown>;
  getAll(): Promise<Record<string, unknown>>;
}

// ---------------------------------------------------------------------------
// Commands (extended)
// ---------------------------------------------------------------------------

/**
 * Extended command registration with keyboard shortcut and status item
 * support. Complements the existing PluginCommandRegistration.
 */
export interface CommandDefinition {
  id: string;
  title: string;
  description?: string;
  /** Keyboard shortcut, e.g. "Ctrl+Shift+P". */
  shortcut?: string;
  /** Target context where this command is available. */
  contexts?: string[];
  /** Execute the command. */
  execute(input?: unknown): unknown | Promise<unknown>;
}

/**
 * Status bar item contributed by a plugin.
 */
export interface StatusItemDefinition {
  id: string;
  /** Text label shown in the status bar. */
  label: string;
  /** Optional icon. */
  icon?: string;
  /** Tooltip text. */
  tooltip?: string;
  /** Command to execute on click. */
  command?: string;
  /** Priority — higher values appear further right. */
  priority?: number;
}

// ---------------------------------------------------------------------------
// Locale (extended)
// ---------------------------------------------------------------------------

/**
 * Locale contribution with fallback chain support.
 */
export interface LocaleDefinition {
  /** BCP 47 locale tag, e.g. "en", "zh-CN". */
  locale: string;
  /** Fallback locale chain, e.g. ["en"]. */
  fallback?: string[];
  /** Translation entries: key → translated string. */
  entries: Record<string, string>;
}
