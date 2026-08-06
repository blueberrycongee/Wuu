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
  registerToolActivityPresenter(definition: ToolActivityPresenterDefinition): Disposable;
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

export const RUNTIME_PROTOCOL_V1 = 1 as const;
export const CAPABILITY_PROTOCOL_V2 = 2 as const;
export const REQUEST_TRANSFORM_CAPABILITY = "agent.request.transform" as const;
export const SYSTEM_PROMPT_SECTION_CAPABILITY = "agent.system_prompt.section" as const;
export const COMPACTION_CAPABILITY = "agent.compaction" as const;

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

export interface CapabilityDescriptor {
  id: string;
  kind: CapabilityKind;
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
  "host.subagent.spawn",
  "host.subagent.status",
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
  "host.subagent.spawn": {
    params: { name: string; description: string; prompt: string; model?: string };
    result: { agent_id: string };
  };
  "host.subagent.status": {
    params: { agent_id: string };
    result: { agent_id: string; status: "running" | "completed" | "failed" | "cancelled"; result?: string };
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

