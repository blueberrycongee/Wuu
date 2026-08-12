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
  type ToolResultProjectionService,
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
}[], toolResultProjection: string): string {
  return `sha256:${createHash("sha256")
    .update(JSON.stringify({ systemPrompt, tools, toolResultProjection }))
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

async function reconstructMessages(
  events: readonly SessionEvent[],
  strict: boolean,
  signal: AbortSignal,
  toolResults: ToolResultProjectionService,
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
        const projected = await toolResults.project(
          event.sessionId,
          record.data,
          signal,
        );
        messages.push({
          role: "tool",
          callId: record.data.callId,
          name: record.data.name,
          content: projected.content,
          isError: projected.isError,
        });
        pendingCalls.delete(record.data.callId);
        break;
      }
      case "agent/run-state":
        if (record.data.state === "interrupted" && (pending.size || pendingCalls.size)) {
          messages.length = safeLength;
          pending.clear();
          pendingCalls.clear();
        }
        break;
      default:
        break;
    }
    if (!pending.size && !pendingCalls.size) {
      safeLength = messages.length;
      safeSeq = event.seq;
    }
  }
  const incomplete = pending.size > 0 || pendingCalls.size > 0;
  if (strict && incomplete) {
    if (pendingCalls.size) {
      throw new Error(`unpaired tool call: ${pendingCalls.keys().next().value}`);
    }
    throw new Error(`incomplete assistant message: ${pending.keys().next().value}`);
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

  async messages(sessionId: string, signal: AbortSignal) {
    return this.projectMessages(sessionId, signal, this.ctx.toolResultProjection);
  }

  private async projectMessages(
    sessionId: string,
    signal: AbortSignal,
    toolResults: ToolResultProjectionService,
  ) {
    signal.throwIfAborted();
    const events = await this.ctx.sessions.load(sessionId);
    return (await reconstructMessages(
      events,
      true,
      signal,
      toolResults,
    )).messages;
  }

  async build(sessionId: string, signal: AbortSignal) {
    const toolResults = this.ctx.toolResultProjection;
    const messages = await this.projectMessages(sessionId, signal, toolResults);

    const prompt = await this.ctx.prompts.render(sessionId);
    const tools = [];
    for (const [, tool] of [...this.ctx.tools.entries()].sort(([left], [right]) => left.localeCompare(right))) {
      if (tool.available && !await tool.available(sessionId)) continue;
      tools.push({
        name: tool.name,
        description: tool.description,
        inputSchema: canonicalJson(tool.inputSchema),
      });
    }
    const toolResultSource = `tool-result-projection:${toolResults.generation}`;
    const sources = [
      ...prompt.sources,
      toolResultSource,
      ...tools.map((tool) => `tool:${tool.name}`),
    ];

    return {
      messages,
      replay: (replaySignal: AbortSignal) =>
        this.projectMessages(sessionId, replaySignal, toolResults),
      tools,
      systemPrompt: prompt.text,
      generation: surfaceGeneration(
        prompt.text,
        tools,
        toolResults.generation,
      ),
      sources,
    };
  }

  async snapshot(sessionId: string, signal: AbortSignal) {
    signal.throwIfAborted();
    const events = await this.ctx.sessions.load(sessionId);
    const { messages, sourceSeq } = await reconstructMessages(
      events,
      false,
      signal,
      this.ctx.toolResultProjection,
    );
    return { messages, sourceSeq };
  }
}

export const contextProjectionPlugin: Plugin = function contextProjection(
  ctx: Context,
) {
  new DefaultModelContextService(ctx);
};

contextProjectionPlugin.inject = ["sessions", "prompts", "toolResultProjection", "tools"];
contextProjectionPlugin.provide = "modelContext";
