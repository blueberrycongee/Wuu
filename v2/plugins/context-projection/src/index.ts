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
  messageId: string;
  content: AssistantContent[];
  text: string;
}

function seedError(messages: readonly ModelMessage[]): string | undefined {
  const calls = new Set<string>();
  for (const message of messages) {
    if (message.role === "tool") {
      if (!calls.delete(message.callId)) {
        return `orphan tool result in model context seed: ${message.callId}`;
      }
      continue;
    }
    if (calls.size) return "model context seed interrupts a tool result batch";
    if (message.role !== "assistant") continue;
    for (const item of message.content) {
      if (item.type !== "tool_call") continue;
      if (calls.has(item.callId)) {
        return `duplicate tool call id in model context seed: ${item.callId}`;
      }
      calls.add(item.callId);
    }
  }
  if (calls.size) return `unpaired tool call in model context seed: ${calls.values().next().value}`;
}

async function reconstructMessages(
  events: readonly SessionEvent[],
  strict: boolean,
  signal: AbortSignal,
  toolResults: ToolResultProjectionService,
) {
  const messages: ModelMessage[] = [];
  let pendingAssistant: PendingAssistant | undefined;
  const pendingCalls: Array<{ callId: string; name: string }> = [];
  let safeLength = 0;
  let safeSeq = 0;
  let sawSeed = false;
  let failure: string | undefined;

  const reject = (message: string) => {
    failure = message;
  };

  for (const event of events) {
    signal.throwIfAborted();
    const record = event.record as
      | AgentSessionRecord
      | CompositionReceiptRecord
      | ModelContextSeedRecord;
    switch (record.type) {
      case "context/model-seed":
        if (sawSeed || messages.length || pendingAssistant || pendingCalls.length) {
          reject("model context seed must precede conversation history");
          break;
        }
        const invalidSeed = seedError(record.data.messages);
        if (invalidSeed) {
          reject(invalidSeed);
          break;
        }
        sawSeed = true;
        messages.push(...record.data.messages);
        break;
      case "agent/user-message":
        if (pendingAssistant || pendingCalls.length) {
          reject("user message interrupts an unfinished assistant or tool batch");
          break;
        }
        messages.push({
          role: "user",
          content: record.data.content.map((item) => item.text).join("\n"),
        });
        break;
      case "agent/assistant-started":
        if (pendingAssistant || pendingCalls.length) {
          reject("assistant message interrupts an unfinished assistant or tool batch");
          break;
        }
        pendingAssistant = { messageId: record.data.messageId, content: [], text: "" };
        break;
      case "agent/assistant-text-delta": {
        if (!pendingAssistant || pendingAssistant.messageId !== record.data.messageId) {
          reject(`text delta has no active assistant message: ${record.data.messageId}`);
          break;
        }
        pendingAssistant.text += record.data.delta;
        break;
      }
      case "agent/assistant-tool-call": {
        if (!pendingAssistant || pendingAssistant.messageId !== record.data.messageId) {
          reject(`tool call has no active assistant message: ${record.data.messageId}`);
          break;
        }
        pendingAssistant.content.push(record.data.call);
        break;
      }
      case "agent/assistant-completed": {
        const assistant = pendingAssistant;
        if (!assistant || assistant.messageId !== record.data.messageId) {
          reject(`completion has no active assistant message: ${record.data.messageId}`);
          break;
        }
        const calls = assistant.content.filter(
          (item): item is Extract<AssistantContent, { type: "tool_call" }> =>
            item.type === "tool_call",
        );
        if (record.data.stopReason === "tool_calls" && !calls.length) {
          reject("assistant completed for tool calls without a tool batch");
          break;
        }
        if (record.data.stopReason !== "tool_calls") {
          assistant.content = assistant.content.filter((item) => item.type !== "tool_call");
        }
        if (assistant.text) {
          const text: TextContent = { type: "text", text: assistant.text };
          assistant.content.unshift(text);
        }
        if (assistant.content.length) {
          messages.push({ role: "assistant", content: assistant.content });
          for (const call of calls) {
            if (pendingCalls.some((item) => item.callId === call.callId)) {
              reject(`duplicate tool call id: ${call.callId}`);
              break;
            }
            pendingCalls.push({ callId: call.callId, name: call.name });
          }
        }
        pendingAssistant = undefined;
        break;
      }
      case "agent/tool-result": {
        const expected = pendingCalls[0];
        if (!expected) {
          reject(`orphan tool result: ${record.data.callId}`);
          break;
        }
        if (expected.callId !== record.data.callId || expected.name !== record.data.name) {
          reject(`tool result is not the next call in its batch: ${record.data.callId}`);
          break;
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
        pendingCalls.shift();
        break;
      }
      case "agent/run-state":
        if (record.data.state === "interrupted" && (pendingAssistant || pendingCalls.length)) {
          messages.length = safeLength;
          pendingAssistant = undefined;
          pendingCalls.length = 0;
          // This is a recovery boundary, not part of the reusable transcript.
          continue;
        }
        if (pendingAssistant || pendingCalls.length) {
          reject("run state interrupts an unfinished assistant or tool batch");
        }
        break;
      default:
        break;
    }
    if (failure) break;
    if (!pendingAssistant && !pendingCalls.length) {
      safeLength = messages.length;
      safeSeq = event.seq;
    }
  }
  if (!failure && (pendingAssistant || pendingCalls.length)) {
    const firstPendingCall = pendingCalls[0];
    failure = firstPendingCall
      ? `unpaired tool call: ${firstPendingCall.callId}`
      : `incomplete assistant message: ${pendingAssistant!.messageId}`;
  }
  if (failure && strict) {
    throw new Error(failure);
  }
  if (failure) {
    return {
      messages: messages.slice(0, safeLength),
      sourceSeq: safeSeq,
    };
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
