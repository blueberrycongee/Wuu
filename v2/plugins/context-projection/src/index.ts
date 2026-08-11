import { createHash } from "node:crypto";
import type {
  AgentSessionRecord,
  AssistantContent,
  CompositionReceiptRecord,
  JsonValue,
  ModelContextSeedRecord,
  ModelMessage,
  SessionEvent,
  TextContent,
} from "@wuu-v2/contracts";
import {
  Service,
  type Context,
  type ModelContextService,
  type Plugin,
} from "@wuu-v2/kernel";

function canonicalJson(value: JsonValue): JsonValue {
  if (Array.isArray(value)) return value.map(canonicalJson);
  if (!value || typeof value !== "object") return value;
  return Object.fromEntries(
    Object.entries(value)
      .sort(([left], [right]) => left.localeCompare(right))
      .map(([key, child]) => [key, canonicalJson(child)]),
  );
}

function surfaceGeneration(systemPrompt: string, tools: readonly {
  name: string;
  description: string;
  inputSchema: JsonValue;
}[]): string {
  return `sha256:${createHash("sha256")
    .update(JSON.stringify({ systemPrompt, tools }))
    .digest("hex")}`;
}

interface PendingAssistant {
  content: AssistantContent[];
  text: string;
}

function validateSeed(messages: readonly ModelMessage[]): void {
  const calls = new Set<string>();
  for (const message of messages) {
    if (message.role === "tool") {
      if (!calls.delete(message.callId)) {
        throw new Error(`orphan tool result in model context seed: ${message.callId}`);
      }
      continue;
    }
    if (calls.size) throw new Error("model context seed interrupts a tool result batch");
    if (message.role !== "assistant") continue;
    for (const item of message.content) {
      if (item.type !== "tool_call") continue;
      if (calls.has(item.callId)) {
        throw new Error(`duplicate tool call id in model context seed: ${item.callId}`);
      }
      calls.add(item.callId);
    }
  }
  if (calls.size) throw new Error(`unpaired tool call in model context seed: ${calls.values().next().value}`);
}

function reconstructMessages(
  events: readonly SessionEvent[],
  strict: boolean,
  signal: AbortSignal,
) {
  const messages: ModelMessage[] = [];
  const pending = new Map<string, PendingAssistant>();
  const pendingCalls = new Map<string, string>();
  let safeLength = 0;
  let safeSeq = 0;
  let sawSeed = false;

  for (const event of events) {
    signal.throwIfAborted();
    const record = event.record as
      | AgentSessionRecord
      | CompositionReceiptRecord
      | ModelContextSeedRecord;
    switch (record.type) {
      case "context/model-seed":
        if (sawSeed || messages.length || pending.size || pendingCalls.size) {
          throw new Error("model context seed must precede conversation history");
        }
        sawSeed = true;
        validateSeed(record.data.messages);
        messages.push(...record.data.messages);
        break;
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
    if (!pending.size && !pendingCalls.size) {
      safeLength = messages.length;
      safeSeq = event.seq;
    }
  }
  if (strict && pendingCalls.size) {
    throw new Error(`unpaired tool call: ${pendingCalls.keys().next().value}`);
  }
  return {
    messages: strict ? messages : messages.slice(0, safeLength),
    sourceSeq: strict ? events.at(-1)?.seq ?? 0 : safeSeq,
  };
}

class DefaultModelContextService extends Service implements ModelContextService {
  constructor(ctx: Context) {
    super(ctx, "modelContext");
  }

  async build(sessionId: string, signal: AbortSignal) {
    signal.throwIfAborted();
    const events = await this.ctx.sessions.load(sessionId);
    const { messages } = reconstructMessages(events, true, signal);

    const prompt = await this.ctx.prompts.render(sessionId);
    const tools = this.ctx.tools.entries()
      .map(([, tool]) => ({
        name: tool.name,
        description: tool.description,
        inputSchema: canonicalJson(tool.inputSchema),
      }))
      .sort((left, right) => left.name.localeCompare(right.name));
    const sources = [...prompt.sources, ...tools.map((tool) => `tool:${tool.name}`)];

    return {
      messages,
      tools,
      systemPrompt: prompt.text,
      generation: surfaceGeneration(prompt.text, tools),
      sources,
    };
  }

  async snapshot(sessionId: string, signal: AbortSignal) {
    signal.throwIfAborted();
    const events = await this.ctx.sessions.load(sessionId);
    const { messages, sourceSeq } = reconstructMessages(events, false, signal);
    return { messages, sourceSeq };
  }
}

export const contextProjectionPlugin: Plugin = function contextProjection(
  ctx: Context,
) {
  new DefaultModelContextService(ctx);
};

contextProjectionPlugin.inject = ["sessions", "prompts", "tools"];
contextProjectionPlugin.provide = "modelContext";
