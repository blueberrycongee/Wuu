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

import type {
  PublicSyntaxTokenName,
  PublicThemeTokenName,
} from "./theme-contract.generated.js";

export {
  LEGACY_THEME_TOKEN_ALIASES,
  PUBLIC_SYNTAX_TOKEN_NAMES,
  PUBLIC_THEME_TOKEN_NAMES,
  canonicalThemeTokenName,
  isPublicSyntaxTokenName,
  isPublicThemeTokenName,
  type PublicSyntaxTokenName,
  type PublicThemeTokenName,
} from "./theme-contract.generated.js";

// ---------------------------------------------------------------------------
// Core workbench types
// ---------------------------------------------------------------------------

/** Unique identifier for a view type. */
export type ViewTypeId = string;
/** Where a view instance appears. */
export type ViewPane = "main" | "sidebar" | "auxiliary" | "overlay" | "tab" | "pane";
/** Stable host-owned regions available for declarative default placement. */
export const VIEW_PLACEMENT_REGIONS = ["main", "sidebar", "auxiliary"] as const;
export type ViewPlacementRegion = (typeof VIEW_PLACEMENT_REGIONS)[number];
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
  getStorage(key: string, scope?: "user" | "workspace"): Promise<string | null>;
  setStorage(key: string, value: string, scope?: "user" | "workspace"): Promise<void>;
  getSetting(key: string): Promise<unknown>;
  executeCommand(commandId: string, input?: unknown): Promise<unknown>;
  openView(viewTypeId: ViewTypeId, options?: OpenViewOptions): Promise<void>;
  closeView(): Promise<void>;
}

export interface PluginUIContainerProps {
  readonly children?: unknown;
  readonly className?: string;
  readonly [property: string]: unknown;
}

export interface PluginUISectionProps extends PluginUIContainerProps {
  readonly title?: unknown;
  readonly description?: unknown;
}

export interface PluginUIStackProps extends PluginUIContainerProps {
  readonly gap?: "small" | "medium" | "large";
}

export interface PluginUIButtonProps extends PluginUIContainerProps {
  readonly variant?: "primary" | "secondary" | "ghost" | "danger";
  readonly type?: "button" | "submit" | "reset";
  readonly disabled?: boolean;
  readonly onClick?: (event: unknown) => void;
}

export interface PluginUITextInputProps extends PluginUIContainerProps {
  readonly label: unknown;
  readonly description?: unknown;
  readonly value?: string | number;
  readonly defaultValue?: string | number;
  readonly placeholder?: string;
  readonly disabled?: boolean;
  readonly onChange?: (event: unknown) => void;
}

export interface PluginUIEmptyStateProps extends PluginUIContainerProps {
  readonly title: unknown;
  readonly description?: unknown;
  readonly actions?: unknown;
}

export type HostUIComponent<Props> = (props: Props) => unknown;

/** Stable host-owned primitives for plugin views. */
export interface PluginUIKit {
  readonly Page: HostUIComponent<PluginUIContainerProps>;
  readonly Panel: HostUIComponent<PluginUIContainerProps>;
  readonly Card: HostUIComponent<PluginUIContainerProps>;
  readonly Section: HostUIComponent<PluginUISectionProps>;
  readonly Stack: HostUIComponent<PluginUIStackProps>;
  readonly Row: HostUIComponent<PluginUIContainerProps>;
  readonly Button: HostUIComponent<PluginUIButtonProps>;
  readonly TextInput: HostUIComponent<PluginUITextInputProps>;
  readonly EmptyState: HostUIComponent<PluginUIEmptyStateProps>;
}

export interface OpenViewOptions {
  pane?: ViewPane;
  context?: Readonly<Record<string, unknown>>;
  persistence?: ViewPersistence;
  reveal?: boolean;
}

export type ToolActivityStatus = "running" | "completed" | "failed";

export interface ToolActivityResultContentPart {
  readonly type: string;
  readonly text?: string;
  readonly data?: string;
  readonly mimeType?: string;
  readonly uri?: string;
  readonly name?: string;
  readonly resource?: unknown;
}

export interface ToolActivityStructuredResult {
  readonly content?: readonly ToolActivityResultContentPart[];
  readonly structuredContent?: unknown;
  readonly metadata?: unknown;
  readonly isError?: boolean;
  readonly activity?: Readonly<{
    id: string;
    kind: string;
    state?: string;
    threadId?: string;
    previewUri?: string;
  }>;
}

/** Host-owned immutable view of a tool call. It intentionally excludes thread internals. */
export interface ToolActivitySnapshot {
  readonly id: string;
  readonly toolName: string;
  readonly capability?: string;
  readonly kind?: string;
  readonly status: ToolActivityStatus;
  readonly argumentsText?: string;
  readonly resultText?: string;
  readonly structuredResult?: ToolActivityStructuredResult;
  readonly error?: string;
}

export interface ToolActivityPresenterProps {
  readonly activity: ToolActivitySnapshot;
  readonly host: ViewHostAPI;
  readonly fallback: unknown;
}

export interface ToolActivityPresenterDefinition {
  readonly id: string;
  /** Exact stable dispatch identity, such as a tool display capability. */
  readonly key: string;
  readonly render: (props: ToolActivityPresenterProps) => unknown;
}

/**
 * Requests that one registered View be opened in a stable host-owned region.
 * The host retains its shell, dimensions, protected chrome, and recovery UI.
 */
export interface ViewPlacementContribution {
  id: string;
  view: ViewTypeId;
  region: ViewPlacementRegion;
  priority?: number;
}

/**
 * @deprecated Use ViewPlacementContribution. Only id, pane, and defaultView
 * are consumed by the compatibility adapter.
 */
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
  tokens: Readonly<Partial<Record<PublicThemeTokenName, string>>>;
  syntax?: Readonly<Partial<Record<PublicSyntaxTokenName, string>>>;
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
  /** Host-owned primitives that inherit the active Wuu theme and spacing rhythm. */
  readonly ui: PluginUIKit;
  readonly pluginId: string;
  readonly generation: string;
  /** Invoke one method owned by this plugin's active runtime generation. */
  invokeRuntime(method: string, input?: unknown): Promise<unknown>;
  /** Observe host lifecycle events; disposal is bound to this generation. */
  onHostEvent(handler: (event: unknown) => void): Disposable;
  registerSlot(slotId: string, contribution: SlotRegistration): Disposable;
  registerSurface(surfaceId: string, contribution: SurfaceRegistration): Disposable;
  registerCommand(command: CommandRegistration): Disposable;
  registerStyle(style: StyleRegistration): Disposable;
  registerLocale(locale: LocaleRegistration): Disposable;
  registerCleanup(cleanup: () => void): Disposable;
  registerViewType(definition: ViewTypeDefinition): Disposable;
  registerViewPlacement(contribution: ViewPlacementContribution): Disposable;
  /** @deprecated Use registerViewPlacement. */
  registerLayoutContribution(contribution: LayoutContribution): Disposable;
  registerRenderer(definition: RendererDefinition): Disposable;
  registerThemeTokens(tokens: ThemeTokens): Disposable;
  registerCSSSnippet(snippet: CSSSnippet): Disposable;
  registerStatusItem(item: StatusItemDefinition): Disposable;
  registerPresenter(definition: PresenterDefinition): Disposable;
  registerToolActivityPresenter(definition: ToolActivityPresenterDefinition): Disposable;
}

/** Minimal type for the host-owned React instance; plugins never bundle React. */
export interface HostReact {
  readonly Fragment: unknown;
  useState<T>(initial: T | (() => T)): [T, (next: T | ((current: T) => T)) => void];
  useEffect(effect: () => void | (() => void), dependencies?: readonly unknown[]): void;
  useMemo<T>(factory: () => T, dependencies: readonly unknown[]): T;
  useCallback<T>(callback: T, dependencies: readonly unknown[]): T;
  useRef<T>(initial: T): { current: T };
  useSyncExternalStore<T>(
    subscribe: (notify: () => void) => () => void,
    getSnapshot: () => T,
    getServerSnapshot?: () => T,
  ): T;
  createElement(
    type: string | ((props: Readonly<Record<string, unknown>>) => unknown),
    props: Readonly<Record<string, unknown>> | null,
    ...children: unknown[]
  ): unknown;
}

// ---------------------------------------------------------------------------
// Executable runtime contract
// ---------------------------------------------------------------------------

export const RUNTIME_PROTOCOL_V1 = 1 as const;
export const CAPABILITY_PROTOCOL_V2 = 2 as const;
export const REQUEST_TRANSFORM_CAPABILITY = "agent.request.transform" as const;
export const SYSTEM_PROMPT_SECTION_CAPABILITY = "agent.system_prompt.section" as const;
export const COMPACTION_CAPABILITY = "agent.compaction" as const;
export const PLUGIN_CLIENT_REQUEST_CAPABILITY = "plugin.client.request" as const;

export type RuntimeHook =
  | "session.start"
  | "session.stop"
  | "chat.message"
  | "chat.request"
  | "tool.definition"
  | "tool.execute.before"
  | "tool.execute.after"
  | "shell.env";

export type CapabilityKind = "observe" | "transform" | "guard" | "around" | "decision";
export type CapabilityErrorPolicy = "propagate" | "isolate" | "ignore";

export interface CapabilityDescriptor {
  id: string;
  kind: CapabilityKind;
  error_policy?: CapabilityErrorPolicy;
  version: number;
  priority?: number;
  depends_on?: string[];
  conflicts?: string[];
}

export const HOST_SERVICE_METHODS = [
  "host.storage.get",
  "host.storage.set",
  "host.storage.delete",
  "host.storage.keys",
  "host.settings.get",
  "host.settings.list",
  "host.child_session.request",
  "host.session.create",
  "host.session.send",
  "host.session.info",
  "host.workspace.root",
  "host.workspace.list",
  "host.diagnostics.log",
] as const;

export type HostServiceMethod = (typeof HOST_SERVICE_METHODS)[number];

export interface HostServiceDescriptor {
  id: HostServiceMethod | (string & {});
  required?: boolean;
}

export interface RuntimeInitializeParams {
  protocol_version: number;
  plugin_id: string;
  plugin_root: string;
  project_root: string;
  wuu_home: string;
  capability_protocol_version?: number;
  supported_host_services?: HostServiceMethod[];
}

export interface RuntimeInitializeResult {
  hooks: RuntimeHook[];
  tools?: ToolRegistration[];
  protocol_version?: 1 | 2;
  capabilities?: CapabilityDescriptor[];
  required_host_services?: HostServiceDescriptor[];
}

export interface JSONSchemaObject {
  type: "object";
  properties?: Readonly<Record<string, unknown>>;
  required?: string[];
  additionalProperties?: boolean;
  [key: string]: unknown;
}

export interface ToolActivityMetadata {
  read_only?: boolean;
  concurrency_safe?: boolean;
  destructive?: boolean;
  risk?: string;
  reason?: string;
}

export interface ToolDisplayMetadata {
  kind?: string;
  text?: string;
  capability?: string;
}

export interface ToolRegistration {
  id: string;
  description: string;
  input_schema: JSONSchemaObject;
  execution_scopes?: Array<"root" | "child">;
  activity?: ToolActivityMetadata;
  display?: ToolDisplayMetadata;
}

export interface CapabilityInvokeParams<TInput = unknown, TOutput = unknown> {
  capability: string;
  input: TInput;
  output: TOutput;
}

export interface CapabilityInvokeResult<TOutput = unknown> {
  output: TOutput;
}

export interface HookInvokeParams<TInput = unknown, TOutput = unknown> {
  hook: RuntimeHook;
  input: TInput;
  output: TOutput;
}

export interface HookInvokeResult<TOutput = unknown> {
  output: TOutput;
}

export interface RequestTransformInput {
  session_id?: string;
  thread_id?: string;
  cwd?: string;
  provider?: string;
  step_index: number;
}

export interface RequestTransformOutput<TRequest = Readonly<Record<string, unknown>>> {
  request: TRequest;
}

export interface SystemPromptSectionInput {
  cwd: string;
  provider: string;
  model: string;
}

export interface SystemPromptSectionOutput {
  text: string;
}

/** Provider-neutral message payload. Preserve unknown fields when compacting. */
export type CompactionMessage = Readonly<Record<string, unknown>>;

export interface CompactionInput<TMessage extends CompactionMessage = CompactionMessage> {
  model: string;
  messages: readonly TMessage[];
}

export interface CompactionOutput<TMessage extends CompactionMessage = CompactionMessage> {
  messages: readonly TMessage[];
}

export interface ToolExecuteParams<TArguments = unknown> {
  tool_id: string;
  session_id?: string;
  thread_id?: string;
  actor_id?: string;
  actor_path?: string;
  cwd: string;
  step_index?: number;
  call_id: string;
  tool: string;
  arguments: TArguments;
}

export type ToolContentType = "text" | "image" | "audio" | "file" | "resource" | "resource_link";

export interface ToolContentPart {
  type: ToolContentType;
  text?: string;
  data?: string;
  mime_type?: string;
  uri?: string;
  name?: string;
  resource?: unknown;
}

export interface ToolResult {
  content?: ToolContentPart[];
  structured_content?: unknown;
  meta?: unknown;
  is_error?: boolean;
  activity?: {
    id: string;
    kind: string;
    state?: string;
    thread_id?: string;
    preview_uri?: string;
  };
}

export interface ToolExecuteResult {
  result: ToolResult;
}

export interface HostServiceContracts {
  "host.storage.get": { params: { key: string }; result: { value: string | null } };
  "host.storage.set": { params: { key: string; value: string }; result: null };
  "host.storage.delete": { params: { key: string }; result: null };
  "host.storage.keys": { params: Record<string, never>; result: { keys: string[] } };
  "host.settings.get": { params: { key: string }; result: { value: unknown } };
  "host.settings.list": { params: Record<string, never>; result: { entries: Record<string, unknown> } };
  "host.child_session.request": {
    params: { action: string; actor_id?: string; actor_path?: string; input?: unknown };
    result: unknown;
  };
  "host.session.create": {
    params: {
      request_id: string;
      visibility: "user" | "plugin";
      parent_session_id?: string;
      context_source: "fresh" | "fork";
    };
    result: { session_id: string; created: boolean };
  };
  "host.session.send": {
    params: {
      request_id: string;
      session_id: string;
      input: {
        prompt: string;
        context_blocks?: Array<{ kind?: string; title?: string; source?: string; content: string }>;
      };
      presentation?: { kind: "query_bubble"; text: string; name?: string };
      cause?: string;
    };
    result: { state: string; session_id: string; turn_id?: string; queue_id?: string };
  };
  "host.session.info": { params: Record<string, never>; result: { session_id: string; thread_id?: string; cwd: string; model: string } };
  "host.workspace.root": { params: Record<string, never>; result: { root: string } };
  "host.workspace.list": {
    params: Record<string, never>;
    result: { workspaces: Array<{ id: string; root: string; name?: string }> };
  };
  "host.diagnostics.log": { params: Readonly<Record<string, unknown>>; result: unknown };
}

export type HostServiceCall<M extends HostServiceMethod = HostServiceMethod> = M extends HostServiceMethod
  ? { id: string; method: M; params?: HostServiceContracts[M]["params"] }
  : never;

export interface HostServiceError {
  code: string;
  message: string;
}

export type HostServiceResponse<M extends HostServiceMethod = HostServiceMethod> =
  | { id: string; result?: HostServiceContracts[M]["result"]; error?: never }
  | { id: string; result?: never; error: HostServiceError };

export type HostServiceRequest<M extends HostServiceMethod = HostServiceMethod> = HostServiceCall<M>;
export type HostServiceResult<M extends HostServiceMethod = HostServiceMethod> = HostServiceResponse<M>;

export interface RuntimePlugin {
  initialize(params: RuntimeInitializeParams): RuntimeInitializeResult | Promise<RuntimeInitializeResult>;
  invokeCapability?(params: CapabilityInvokeParams): CapabilityInvokeResult | Promise<CapabilityInvokeResult>;
  invokeHook?(params: HookInvokeParams): HookInvokeResult | Promise<HookInvokeResult>;
  executeTool?(params: ToolExecuteParams): ToolExecuteResult | Promise<ToolExecuteResult>;
  shutdown?(): void | Promise<void>;
}

export interface RuntimeInitializeRequest {
  id: string;
  method: "initialize";
  params: RuntimeInitializeParams;
}

export interface RuntimeCapabilityRequest {
  id: string;
  method: "capability.invoke";
  params: CapabilityInvokeParams;
}

export interface RuntimeHookRequest {
  id: string;
  method: "hook.invoke";
  params: HookInvokeParams;
}

export interface RuntimeToolRequest {
  id: string;
  method: "tool.execute";
  params: ToolExecuteParams;
}

export interface RuntimeShutdownRequest {
  id: string;
  method: "shutdown";
  params?: undefined;
}

export type RuntimeRequest =
  | RuntimeInitializeRequest
  | RuntimeCapabilityRequest
  | RuntimeHookRequest
  | RuntimeToolRequest
  | RuntimeShutdownRequest;

export type RuntimeResponse =
  | { id: string; result: RuntimeInitializeResult | CapabilityInvokeResult | HookInvokeResult | ToolExecuteResult | null }
  | { id: string; error: { message: string } };

export async function handleRuntimeRequest(plugin: RuntimePlugin, request: RuntimeRequest): Promise<RuntimeResponse> {
  try {
    switch (request.method) {
      case "initialize":
        return { id: request.id, result: await plugin.initialize(request.params) };
      case "capability.invoke":
        if (!plugin.invokeCapability) throw new Error("capability.invoke is not implemented");
        return { id: request.id, result: await plugin.invokeCapability(request.params) };
      case "hook.invoke":
        if (!plugin.invokeHook) throw new Error("hook.invoke is not implemented");
        return { id: request.id, result: await plugin.invokeHook(request.params) };
      case "tool.execute":
        if (!plugin.executeTool) throw new Error("tool.execute is not implemented");
        return { id: request.id, result: await plugin.executeTool(request.params) };
      case "shutdown":
        await plugin.shutdown?.();
        return { id: request.id, result: null };
    }
  } catch (error) {
    return { id: request.id, error: { message: error instanceof Error ? error.message : String(error) } };
  }
}

export interface JSONLInput extends AsyncIterable<Uint8Array | string> {}
export interface JSONLOutput { write(chunk: string): unknown }

/** Runs a plugin over Wuu's one-request/one-response JSON-lines transport. */
export async function runJSONLRuntime(
  plugin: RuntimePlugin,
  streams: { input: JSONLInput; output: JSONLOutput },
): Promise<void> {
  const decoder = new TextDecoder();
  let buffered = "";
  for await (const chunk of streams.input) {
    buffered += typeof chunk === "string" ? chunk : decoder.decode(chunk, { stream: true });
    const lines = buffered.split("\n");
    buffered = lines.pop() ?? "";
    for (const line of lines) {
      await writeRuntimeLine(plugin, line, streams.output);
    }
  }
  buffered += decoder.decode();
  if (buffered.trim() !== "") await writeRuntimeLine(plugin, buffered, streams.output);
}

async function writeRuntimeLine(plugin: RuntimePlugin, line: string, output: JSONLOutput): Promise<void> {
  if (line.trim() === "") return;
  let response: RuntimeResponse;
  try {
    const parsed: unknown = JSON.parse(line);
    if (!isRuntimeRequest(parsed)) throw new Error("invalid runtime request");
    response = await handleRuntimeRequest(plugin, parsed);
  } catch (error) {
    response = { id: "invalid", error: { message: error instanceof Error ? error.message : String(error) } };
  }
  output.write(`${JSON.stringify(response)}\n`);
}

function isRuntimeRequest(value: unknown): value is RuntimeRequest {
  if (typeof value !== "object" || value === null) return false;
  const request = value as { id?: unknown; method?: unknown; params?: unknown };
  if (typeof request.id !== "string" || typeof request.method !== "string") return false;
  if (request.method === "shutdown") return true;
  return ["initialize", "capability.invoke", "hook.invoke", "tool.execute"].includes(request.method)
    && typeof request.params === "object" && request.params !== null;
}

export interface Disposable {
  dispose(): void;
}

export const PRESENTATION_TARGETS = [
  "conversation.item", "conversation.process", "conversation.tool-activity",
  "conversation.composer", "header.conversation", "header.workspace",
  "navigation.primary", "app.status", "content.preview", "settings",
] as const;
export type BuiltInPresentationTarget = (typeof PRESENTATION_TARGETS)[number];
/** Built-in targets plus future dotted, namespaced targets. */
export type PresentationTarget = BuiltInPresentationTarget | (string & {});
export type PresentationMode = "replace" | "wrap";

/** Sanitized attachment metadata. The opaque id is resolved only through host actions. */
export interface AttachmentDescriptorV1 {
  readonly id: string;
  readonly name: string;
  readonly mimeType?: string;
  readonly sizeBytes?: number;
  readonly width?: number;
  readonly height?: number;
}

export interface ConversationContentV1 {
  readonly type: "plain-text" | "markdown" | "code";
  readonly text: string;
  readonly language?: string;
}

export interface ConversationToolReferenceV1 {
  readonly id: string;
  readonly name?: string;
  readonly capability?: string;
  readonly status?: "pending" | ToolActivityStatus | "cancelled";
}

export interface ConversationProcessReferenceV1 {
  readonly id: string;
  readonly label?: string;
  readonly phase?: string;
  readonly status?: "pending" | "running" | "completed" | "failed" | "cancelled";
}

/** Public conversation item data, independent of renderer and thread storage types. */
export interface ConversationItemSnapshotV1 {
  readonly contractVersion: 1;
  readonly id: string;
  readonly kind: "user-message" | "assistant-message" | "reasoning" | "notice" | "attachment" | "tool-reference" | "process-reference";
  readonly status?: "pending" | "streaming" | "completed" | "failed" | "cancelled";
  readonly phase?: string;
  readonly text?: string;
  readonly contentType?: "plain-text" | "markdown" | "code";
  readonly content?: readonly ConversationContentV1[];
  readonly attachments?: readonly AttachmentDescriptorV1[];
  readonly toolReferences?: readonly ConversationToolReferenceV1[];
  readonly processReferences?: readonly ConversationProcessReferenceV1[];
  readonly createdAt?: string;
}

export type ConversationProcessKindV1 = "reasoning" | "tool-group" | "mixed";
export type ConversationProcessStatusV1 = "running" | "completed" | "failed";

/** One ordered, sanitized entry in the process surface. */
export interface ConversationProcessItemV1 {
  readonly id: string;
  readonly kind: "reasoning" | "tool-activity";
  readonly status: ConversationProcessStatusV1;
  /** Reasoning text only. Tool arguments and results are intentionally excluded. */
  readonly text?: string;
  readonly toolName?: string;
  readonly capability?: string;
  readonly toolKind?: string;
  readonly error?: string;
}

/** Public data for the complete reasoning/tool process region of one turn. */
export interface ConversationProcessSnapshotV1 {
  readonly contractVersion: 1;
  readonly kind: ConversationProcessKindV1;
  readonly status: ConversationProcessStatusV1;
  readonly streaming: boolean;
  readonly active: boolean;
  /** Items preserve host display order and never contain private thread records. */
  readonly items: readonly ConversationProcessItemV1[];
}

export interface ComposerQueueSummaryV1 {
  readonly id: string;
  readonly text?: string;
  readonly attachmentCount?: number;
  readonly status?: "queued" | "pending";
}

export interface ModelSummaryV1 {
  readonly id: string;
  readonly label: string;
  readonly providerId?: string;
  readonly contextWindowTokens?: number;
}

export interface RuntimeSummaryV1 {
  readonly id: string;
  readonly label: string;
  readonly status?: "available" | "unavailable" | "starting" | "running" | "error";
}

export interface PermissionSummaryV1 {
  readonly id: string;
  readonly label: string;
  readonly description?: string;
}

export interface ContextUsageSummaryV1 {
  readonly usedTokens?: number;
  readonly limitTokens?: number;
  readonly percent?: number;
}

export type ComposerSubmissionModeV1 = "send" | "queue" | "steer";

export interface ComposerSnapshotV1 {
  readonly contractVersion: 1;
  readonly draftText?: string;
  readonly attachments?: readonly AttachmentDescriptorV1[];
  readonly queued?: readonly ComposerQueueSummaryV1[];
  readonly pending?: readonly ComposerQueueSummaryV1[];
  readonly running?: boolean;
  readonly readOnly?: boolean;
  readonly threadId?: string;
  readonly variant?: string;
  readonly availableSubmissionModes?: readonly ComposerSubmissionModeV1[];
  readonly activeSubmissionMode?: ComposerSubmissionModeV1;
  readonly model?: ModelSummaryV1;
  readonly runtime?: RuntimeSummaryV1;
  readonly permission?: PermissionSummaryV1;
  readonly contextUsage?: ContextUsageSummaryV1;
  readonly disabledReason?: string;
}

export interface HeaderTabDescriptorV1 {
  readonly id: string;
  readonly title: string;
  readonly subtitle?: string;
  readonly kind?: string;
  readonly busy?: boolean;
  readonly dirty?: boolean;
  readonly disabled?: boolean;
}

export interface HeaderSnapshotV1 {
  readonly contractVersion: 1;
  readonly scope: "conversation" | "workspace";
  readonly title?: string;
  readonly subtitle?: string;
  readonly tabs?: readonly HeaderTabDescriptorV1[];
  readonly activeTabId?: string;
  readonly busy?: boolean;
  readonly dirty?: boolean;
  readonly canNavigateBack?: boolean;
  readonly canNavigateForward?: boolean;
}

export interface NavigationNodeV1 {
  readonly id: string;
  readonly kind: "section" | "project" | "thread" | "room" | "command";
  readonly label: string;
  readonly parentId?: string;
  readonly depth?: number;
  readonly description?: string;
  readonly icon?: string;
  readonly active?: boolean;
  readonly pinned?: boolean;
  readonly unread?: boolean;
  readonly running?: boolean;
  readonly disabled?: boolean;
}

export interface NavigationSnapshotV1 {
  readonly contractVersion: 1;
  /** Nodes are in host display order; parentId and depth describe hierarchy. */
  readonly nodes: readonly NavigationNodeV1[];
  readonly activeNodeId?: string;
}

export interface StatusItemV1 {
  readonly id: string;
  readonly label: string;
  readonly kind?: "info" | "progress" | "success" | "warning" | "error";
  readonly detail?: string;
  readonly icon?: string;
  readonly progress?: number;
  readonly busy?: boolean;
  readonly disabled?: boolean;
  readonly actionId?: string;
}

export interface StatusSnapshotV1 {
  readonly contractVersion: 1;
  readonly items: readonly StatusItemV1[];
}

export interface FileSelectionDescriptorV1 {
  readonly startOffset?: number;
  readonly endOffset?: number;
  readonly startLine?: number;
  readonly startColumn?: number;
  readonly endLine?: number;
  readonly endColumn?: number;
}

/** File data safe for presentation; it never contains filesystem or window handles. */
export interface FilePreviewSnapshotV1 {
  readonly contractVersion: 1;
  readonly resourceId: string;
  readonly workspaceRelativePath: string;
  readonly contentType?: string;
  readonly text?: string;
  readonly safeHostUrl?: string;
  readonly sizeBytes?: number;
  readonly binary?: boolean;
  readonly readOnly?: boolean;
  readonly dirty?: boolean;
  readonly loading?: boolean;
  readonly error?: string;
  readonly selection?: FileSelectionDescriptorV1;
}

export interface SettingsPageSummaryV1 {
  readonly id: string;
  readonly label: string;
  readonly description?: string;
  readonly disabled?: boolean;
}

export interface SettingsPluginSummaryV1 {
  readonly id: string;
  readonly name: string;
  readonly version?: string;
  readonly enabled?: boolean;
  readonly status?: string;
}

export interface SettingsRuntimeSummaryV1 {
  readonly id: string;
  readonly label: string;
  readonly version?: string;
  readonly status?: string;
}

export interface SettingsProviderSummaryV1 {
  readonly id: string;
  readonly label: string;
  readonly configured?: boolean;
  readonly status?: string;
}

/** Sanitized settings navigation and service summaries; provider credentials are excluded. */
export interface SettingsSnapshotV1 {
  readonly contractVersion: 1;
  readonly activePageId?: string;
  readonly availablePages?: readonly SettingsPageSummaryV1[];
  readonly plugins?: readonly SettingsPluginSummaryV1[];
  readonly runtimes?: readonly SettingsRuntimeSummaryV1[];
  readonly providers?: readonly SettingsProviderSummaryV1[];
  readonly busy?: boolean;
  readonly error?: string;
}

export const CONVERSATION_ITEM_ACTIONS = {
  copy: "conversation.item.copy",
  edit: "conversation.item.edit",
  retry: "conversation.item.retry",
  openAttachment: "conversation.item.open-attachment",
  openTool: "conversation.item.open-tool",
  cancelProcess: "conversation.item.cancel-process",
} as const;
export type ConversationItemActionId = (typeof CONVERSATION_ITEM_ACTIONS)[keyof typeof CONVERSATION_ITEM_ACTIONS];

export const COMPOSER_ACTIONS = {
  setDraft: "conversation.composer.set-draft",
  addAttachment: "conversation.composer.add-attachment",
  removeAttachment: "conversation.composer.remove-attachment",
  setSubmissionMode: "conversation.composer.set-submission-mode",
  submit: "conversation.composer.submit",
  stop: "conversation.composer.stop",
} as const;
export type ComposerActionId = (typeof COMPOSER_ACTIONS)[keyof typeof COMPOSER_ACTIONS];

export const HEADER_ACTIONS = {
  selectTab: "header.select-tab",
  closeTab: "header.close-tab",
  navigateBack: "header.navigate-back",
  navigateForward: "header.navigate-forward",
} as const;
export type HeaderActionId = (typeof HEADER_ACTIONS)[keyof typeof HEADER_ACTIONS];

export const NAVIGATION_ACTIONS = {
  activateNode: "navigation.activate-node",
  pinNode: "navigation.pin-node",
  unpinNode: "navigation.unpin-node",
} as const;
export type NavigationActionId = (typeof NAVIGATION_ACTIONS)[keyof typeof NAVIGATION_ACTIONS];

export const STATUS_ACTIONS = { activateItem: "status.activate-item" } as const;
export type StatusActionId = (typeof STATUS_ACTIONS)[keyof typeof STATUS_ACTIONS];

export const FILE_PREVIEW_ACTIONS = {
  open: "file-preview.open",
  reveal: "file-preview.reveal",
  select: "file-preview.select",
  save: "file-preview.save",
  reload: "file-preview.reload",
} as const;
export type FilePreviewActionId = (typeof FILE_PREVIEW_ACTIONS)[keyof typeof FILE_PREVIEW_ACTIONS];

export const SETTINGS_ACTIONS = {
  openPage: "settings.open-page",
  updateValue: "settings.update-value",
  refresh: "settings.refresh",
} as const;
export type SettingsActionId = (typeof SETTINGS_ACTIONS)[keyof typeof SETTINGS_ACTIONS];

export interface PresentationHost extends ViewHostAPI {
  readonly actions: readonly string[];
  invoke(action: string, input?: unknown): Promise<unknown>;
}

export interface PresenterProps {
  readonly contractVersion: 1;
  readonly target: PresentationTarget;
  readonly key?: string;
  readonly snapshot: unknown;
  readonly host: PresentationHost;
  readonly fallback: unknown;
}

export interface PresenterDefinition {
  readonly id: string;
  readonly target: PresentationTarget;
  readonly key?: string;
  readonly mode?: PresentationMode;
  readonly priority?: number;
  readonly render: (props: PresenterProps) => unknown;
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
