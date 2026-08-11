import type {
  AgentSessionRecord,
  AssistantContent,
  CompositionReceiptRecord,
  ModelMessage,
  TextContent,
} from "@wuu-v2/contracts";
import {
  Service,
  type Context,
  type ModelContextService,
  type Plugin,
} from "@wuu-v2/kernel";

interface PendingAssistant {
  content: AssistantContent[];
  text: string;
}

class DefaultModelContextService extends Service implements ModelContextService {
  constructor(ctx: Context) {
    super(ctx, "modelContext");
  }

  async build(sessionId: string, signal: AbortSignal) {
    signal.throwIfAborted();
    const events = await this.ctx.sessions.load(sessionId);
    const messages: ModelMessage[] = [];
    const pending = new Map<string, PendingAssistant>();
    const pendingCalls = new Map<string, string>();

    for (const event of events) {
      signal.throwIfAborted();
      const record = event.record as AgentSessionRecord | CompositionReceiptRecord;
      switch (record.type) {
        case "agent/user-message":
          messages.push({
            role: "user",
            content: record.data.content.map((item) => item.text).join("\n"),
          });
          break;
        case "agent/assistant-started":
          pending.set(record.data.messageId, { content: [], text: "" });
          break;
        case "agent/assistant-text-delta": {
          const assistant = pending.get(record.data.messageId);
          if (assistant) assistant.text += record.data.delta;
          break;
        }
        case "agent/assistant-tool-call": {
          const assistant = pending.get(record.data.messageId);
          if (assistant) assistant.content.push(record.data.call);
          break;
        }
        case "agent/assistant-completed": {
          const assistant = pending.get(record.data.messageId);
          if (!assistant) break;
          if (record.data.stopReason !== "tool_calls") {
            assistant.content = assistant.content.filter((item) => item.type !== "tool_call");
          }
          if (assistant.text) {
            const text: TextContent = { type: "text", text: assistant.text };
            assistant.content.unshift(text);
          }
          if (assistant.content.length) {
            messages.push({ role: "assistant", content: assistant.content });
            for (const item of assistant.content) {
              if (item.type !== "tool_call") continue;
              if (pendingCalls.has(item.callId)) {
                throw new Error(`duplicate tool call id: ${item.callId}`);
              }
              pendingCalls.set(item.callId, item.name);
            }
          }
          pending.delete(record.data.messageId);
          break;
        }
        case "agent/tool-result": {
          const expectedName = pendingCalls.get(record.data.callId);
          if (!expectedName) throw new Error(`orphan tool result: ${record.data.callId}`);
          if (expectedName !== record.data.name) {
            throw new Error(`tool result name mismatch: ${record.data.callId}`);
          }
          messages.push({
            role: "tool",
            callId: record.data.callId,
            name: record.data.name,
            content: record.data.content,
            isError: record.data.isError,
          });
          pendingCalls.delete(record.data.callId);
          break;
        }
        default:
          break;
      }
    }
    if (pendingCalls.size) {
      throw new Error(`unpaired tool call: ${pendingCalls.keys().next().value}`);
    }

    const prompt = this.ctx.prompts.render();
    const tools = this.ctx.tools.entries().map(([, tool]) => ({
      name: tool.name,
      description: tool.description,
      inputSchema: tool.inputSchema,
    }));
    const sources = [...prompt.sources, ...tools.map((tool) => `tool:${tool.name}`)];

    return {
      messages,
      tools,
      systemPrompt: prompt.text,
      generation: sources.join("|") || "empty",
      sources,
    };
  }
}

export const contextProjectionPlugin: Plugin = function contextProjection(
  ctx: Context,
) {
  new DefaultModelContextService(ctx);
};

contextProjectionPlugin.inject = ["sessions", "prompts", "tools"];
contextProjectionPlugin.provide = "modelContext";
