import {
  CAPABILITY_PROTOCOL_V2,
  PRESENTATION_TARGETS,
  REQUEST_TRANSFORM_CAPABILITY,
  handleRuntimeRequest,
  runJSONLRuntime,
  type RuntimePlugin,
  type PresenterDefinition,
  type ToolActivityPresenterDefinition,
  type ToolActivitySnapshot,
} from "./index.js";

if (!PRESENTATION_TARGETS.includes("conversation.tool-activity") || !PRESENTATION_TARGETS.includes("settings")) {
  throw new Error("presentation target contract failed");
}
const genericPresenter: PresenterDefinition = {
  id: "preview",
  target: "content.preview",
  mode: "wrap",
  priority: 10,
  render: ({ contractVersion, fallback }) => contractVersion === 1 ? fallback : null,
};
if (genericPresenter.target !== "content.preview") throw new Error("presenter definition contract failed");

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
