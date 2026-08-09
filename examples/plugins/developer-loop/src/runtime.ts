import {
  COMPACTION_CAPABILITY,
  REQUEST_TRANSFORM_CAPABILITY,
  SYSTEM_PROMPT_SECTION_CAPABILITY,
  runJSONLRuntime,
  type CompactionInput,
  type RuntimePlugin,
} from "@wuu/plugin-sdk";

const plugin: RuntimePlugin = {
  initialize() {
    return {
      protocol_version: 2,
      capabilities: [
        { id: REQUEST_TRANSFORM_CAPABILITY, kind: "transform", version: 1 },
        { id: SYSTEM_PROMPT_SECTION_CAPABILITY, kind: "transform", version: 1 },
        { id: COMPACTION_CAPABILITY, kind: "decision", version: 1 },
      ],
      tools: [{
        id: "developer-loop-echo",
        description: "Return a short confirmation for developer-loop checks.",
        input_schema: { type: "object", properties: {} },
        display: { capability: "developer-loop.tool.echo", kind: "acceptance" },
      }],
    };
  },
  invokeCapability({ capability, input, output }) {
    switch (capability) {
      case SYSTEM_PROMPT_SECTION_CAPABILITY:
        return { output: { text: "Use the developer-loop tool when asked to verify plugin activation." } };
      case COMPACTION_CAPABILITY:
        return { output: { messages: (input as CompactionInput).messages } };
      default:
        return { output };
    }
  },
  executeTool() {
    return { result: { content: [{ type: "text", text: "developer-loop tool ok" }] } };
  },
};

runJSONLRuntime(plugin, { input: process.stdin, output: process.stdout }).catch((error: unknown) => {
  console.error(error instanceof Error ? error.message : String(error));
  process.exitCode = 1;
});
