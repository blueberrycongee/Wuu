import {
  CAPABILITY_PROTOCOL_V2,
  COMPOSER_ACTIONS,
  CONVERSATION_ITEM_ACTIONS,
  FILE_PREVIEW_ACTIONS,
  HEADER_ACTIONS,
  NAVIGATION_ACTIONS,
  PRESENTATION_TARGETS,
  PUBLIC_SYNTAX_TOKEN_NAMES,
  PUBLIC_THEME_TOKEN_NAMES,
  REQUEST_TRANSFORM_CAPABILITY,
  SETTINGS_ACTIONS,
  STATUS_ACTIONS,
  handleRuntimeRequest,
  runJSONLRuntime,
  type ComposerSnapshotV1,
  type ConversationItemSnapshotV1,
  type ConversationProcessSnapshotV1,
  type FilePreviewSnapshotV1,
  type HeaderSnapshotV1,
  type NavigationSnapshotV1,
  type RuntimePlugin,
  type PresenterDefinition,
  type SettingsSnapshotV1,
  type StatusSnapshotV1,
  type ToolActivityPresenterDefinition,
  type ToolActivitySnapshot,
} from "./index.js";

if (!PRESENTATION_TARGETS.includes("conversation.tool-activity") || !PRESENTATION_TARGETS.includes("settings")) {
  throw new Error("presentation target contract failed");
}
if (!PUBLIC_THEME_TOKEN_NAMES.includes("--wuu-color-canvas") || !PUBLIC_SYNTAX_TOKEN_NAMES.includes("--wuu-syntax-keyword")) {
  throw new Error("theme token contract failed");
}
const genericPresenter: PresenterDefinition = {
  id: "preview",
  target: "content.preview",
  mode: "wrap",
  priority: 10,
  render: ({ contractVersion, fallback }) => contractVersion === 1 ? fallback : null,
};
if (genericPresenter.target !== "content.preview") throw new Error("presenter definition contract failed");

const conversationItem: ConversationItemSnapshotV1 = Object.freeze({
  contractVersion: 1,
  id: "message-1",
  kind: "assistant-message",
  status: "completed",
  content: Object.freeze([{ type: "markdown", text: "Done" }] as const),
  attachments: Object.freeze([{ id: "attachment-1", name: "result.png", mimeType: "image/png", width: 640, height: 480 }]),
  toolReferences: Object.freeze([{ id: "call-1", name: "read_file", capability: "workspace.read", status: "completed" }] as const),
});
const conversationProcess: ConversationProcessSnapshotV1 = Object.freeze({
  contractVersion: 1,
  kind: "mixed",
  status: "running",
  streaming: true,
  active: true,
  items: Object.freeze([
    Object.freeze({ id: "reason-1", kind: "reasoning", status: "running", text: "Checking" }),
    Object.freeze({ id: "tool-1", kind: "tool-activity", status: "completed", toolName: "read_file", capability: "workspace.read" }),
  ]),
});
const composer: ComposerSnapshotV1 = Object.freeze({
  contractVersion: 1,
  draftText: "Continue",
  threadId: "thread-1",
  availableSubmissionModes: Object.freeze(["send", "queue"] as const),
  model: Object.freeze({ id: "model-1", label: "Model", providerId: "provider-1" }),
  contextUsage: Object.freeze({ usedTokens: 100, limitTokens: 1_000, percent: 10 }),
});
const header: HeaderSnapshotV1 = Object.freeze({
  contractVersion: 1,
  scope: "conversation",
  title: "Conversation",
  tabs: Object.freeze([{ id: "thread-1", title: "Conversation" }] as const),
  activeTabId: "thread-1",
});
const navigation: NavigationSnapshotV1 = Object.freeze({
  contractVersion: 1,
  nodes: Object.freeze([
    { id: "workspace", kind: "section", label: "Workspace", depth: 0 },
    { id: "thread-1", kind: "thread", label: "Conversation", parentId: "workspace", depth: 1, active: true },
  ] as const),
  activeNodeId: "thread-1",
});
const status: StatusSnapshotV1 = Object.freeze({
  contractVersion: 1,
  items: Object.freeze([{ id: "runtime", label: "Running", kind: "progress", busy: true, actionId: STATUS_ACTIONS.activateItem }] as const),
});
const preview: FilePreviewSnapshotV1 = Object.freeze({
  contractVersion: 1,
  resourceId: "resource-1",
  workspaceRelativePath: "src/index.ts",
  contentType: "text/typescript",
  text: "export {};",
  selection: Object.freeze({ startLine: 1, startColumn: 1 }),
});
const settings: SettingsSnapshotV1 = Object.freeze({
  contractVersion: 1,
  activePageId: "providers",
  availablePages: Object.freeze([{ id: "providers", label: "Providers" }]),
  providers: Object.freeze([{ id: "provider-1", label: "Provider", configured: true }]),
});
if (
  conversationItem.contractVersion !== 1 || conversationProcess.kind !== "mixed" || composer.contractVersion !== 1 || header.contractVersion !== 1
  || navigation.nodes.length !== 2 || status.items.length !== 1 || preview.resourceId !== "resource-1"
  || settings.providers?.[0]?.configured !== true
) throw new Error("presentation snapshot V1 contract failed");

const actionIds = [
  CONVERSATION_ITEM_ACTIONS.copy,
  COMPOSER_ACTIONS.submit,
  HEADER_ACTIONS.selectTab,
  NAVIGATION_ACTIONS.activateNode,
  STATUS_ACTIONS.activateItem,
  FILE_PREVIEW_ACTIONS.open,
  SETTINGS_ACTIONS.openPage,
];
if (actionIds.some((action) => !action.includes("."))) throw new Error("presentation action ID contract failed");

const snapshot: ToolActivitySnapshot = Object.freeze({
  id: "call",
  toolName: "echo",
  status: "running",
  argumentsText: '{"partial":',
});
const presenter: ToolActivityPresenterDefinition = {
  id: "echo",
  key: "tool.echo",
  render: ({ activity, fallback }) => activity.id === "call" ? activity.argumentsText : fallback,
};
if (presenter.render({ activity: snapshot, host: {} as never, fallback: "native" }) !== '{"partial":') {
  throw new Error("tool activity presenter contract failed");
}

const plugin: RuntimePlugin = {
  initialize(params) {
    return {
      hooks: [],
      protocol_version: CAPABILITY_PROTOCOL_V2,
      capabilities: [{ id: REQUEST_TRANSFORM_CAPABILITY, kind: "transform", version: 1 }],
      tools: [{ id: "echo", description: "Echo input", input_schema: { type: "object" } }],
      required_host_services: [],
    };
  },
  invokeCapability(params) {
    return { output: params.output };
  },
  executeTool() {
    return { result: { content: [{ type: "text", text: "ok" }] } };
  },
};

const initialized = await handleRuntimeRequest(plugin, {
  id: "init",
  method: "initialize",
  params: { protocol_version: 1, capability_protocol_version: 2, plugin_id: "test", plugin_root: ".", project_root: ".", wuu_home: "." },
});
if (!("result" in initialized) || initialized.result === null || !("protocol_version" in initialized.result)) {
  throw new Error("initialize did not return the v2 descriptor");
}

async function* input(): AsyncIterable<string> {
  yield '{"id":"tool","method":"tool.execute","params":{"tool_id":"echo","cwd":".","call_id":"1","tool":"echo","arguments":{}}}\n';
  yield "not-json\n";
}
const lines: string[] = [];
await runJSONLRuntime(plugin, { input: input(), output: { write: (line) => lines.push(line) } });
if (lines.length !== 2 || !lines[0]?.includes('"text":"ok"') || !lines[1]?.includes('"id":"invalid"')) {
  throw new Error(`unexpected JSONL responses: ${lines.join("")}`);
}
