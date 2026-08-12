import { createHash, randomUUID } from "node:crypto";
import { isDeepStrictEqual } from "node:util";
import type {
  AgentLoop,
  AgentLoopInput,
  AgentLoopPrepareInput,
  AgentRunResult,
  AgentSessionRecord,
  CompositionReceiptRecord,
  EventSource,
  JsonValue,
  ToolCallContent,
  ToolDefinition,
  ToolResult,
  ModelCacheHint,
  ModelMessage,
  ModelProvider,
  ModelTool,
  ModelUsage,
} from "@wuu-v2/contracts";
import { type Context, type Plugin } from "@wuu-v2/kernel";

export interface DefaultAgentLoopConfig {
  agentId?: string;
}

const source: EventSource = {
  pluginId: "default-agent-loop",
  generation: "v1",
};

function cacheHint(sessionId: string, systemPrompt: string, messages: readonly ModelMessage[]): ModelCacheHint {
  let lastUser = -1;
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    if (messages[index]?.role === "user") {
      lastUser = index;
      break;
    }
  }
  const stablePrefixMessages = lastUser < 0 ? messages.length : lastUser;
  return {
    key: createHash("sha256")
      .update("wuu-v2-model-cache\0")
      .update(sessionId)
      .digest("hex"),
    stableSystem: Boolean(systemPrompt),
    stablePrefixMessages,
    turnPrefixMessages: lastUser < 0 ? stablePrefixMessages : lastUser + 1,
  };
}

function surfaceGeneration(
  contextGeneration: string,
  providerIdentity: string,
  tools: readonly ModelTool[],
): string {
  return `sha256:${createHash("sha256")
    .update(JSON.stringify({ contextGeneration, providerIdentity, tools }))
    .digest("hex")}`;
}

function assertAppendOnlyTranscript(
  previous: readonly ModelMessage[],
  current: readonly ModelMessage[],
): void {
  if (current.length < previous.length) {
    throw new Error("model context shortened during an active run");
  }
  for (let index = 0; index < previous.length; index += 1) {
    if (!isDeepStrictEqual(previous[index], current[index])) {
      throw new Error(`model context changed inside the frozen prefix at message ${index}`);
    }
  }
}

function schemaError(schema: JsonValue, value: JsonValue, path = "$"): string | undefined {
  if (schema === true) return;
  if (schema === false) return `${path} is not allowed`;
  if (!schema || typeof schema !== "object" || Array.isArray(schema)) {
    return `${path} has an invalid schema`;
  }
  if (Array.isArray(schema.enum) && !schema.enum.some((item) =>
    JSON.stringify(item) === JSON.stringify(value))) {
    return `${path} is not one of the allowed values`;
  }
  const expected = schema.type;
  if (expected === "object") {
    if (!value || typeof value !== "object" || Array.isArray(value)) return `${path} must be an object`;
    const properties = schema.properties && typeof schema.properties === "object" && !Array.isArray(schema.properties)
      ? schema.properties
      : {};
    if (Array.isArray(schema.required)) {
      for (const key of schema.required) {
        if (typeof key === "string" && !(key in value)) return `${path}.${key} is required`;
      }
    }
    for (const [key, child] of Object.entries(value)) {
      const childSchema = properties[key];
      if (childSchema === undefined) {
        if (schema.additionalProperties === false) return `${path}.${key} is not allowed`;
        continue;
      }
      const error = schemaError(childSchema, child, `${path}.${key}`);
      if (error) return error;
    }
    return;
  }
  if (expected === "array") {
    if (!Array.isArray(value)) return `${path} must be an array`;
    if (typeof schema.minItems === "number" && value.length < schema.minItems) {
      return `${path} must contain at least ${schema.minItems} items`;
    }
    if (schema.items !== undefined) {
      for (const [index, child] of value.entries()) {
        const error = schemaError(schema.items, child, `${path}[${index}]`);
        if (error) return error;
      }
    }
    return;
  }
  if (expected === "string") return typeof value === "string" ? undefined : `${path} must be a string`;
  if (expected === "number") return typeof value === "number" && Number.isFinite(value)
    ? undefined
    : `${path} must be a number`;
  if (expected === "integer") return typeof value === "number" && Number.isInteger(value)
    ? undefined
    : `${path} must be an integer`;
  if (expected === "boolean") return typeof value === "boolean" ? undefined : `${path} must be a boolean`;
  if (expected === "null") return value === null ? undefined : `${path} must be null`;
  return expected === undefined ? undefined : `${path} uses an unsupported schema type`;
}

class DefaultAgentLoop implements AgentLoop {
  private surface: {
    systemPrompt: string;
    tools: ModelTool[];
    sources: string[];
    generation: string;
    provider: ModelProvider;
    executors: ReadonlyMap<string, ToolDefinition>;
    replay(signal: AbortSignal): Promise<ModelMessage[]>;
  } | undefined;

  constructor(
    private readonly ctx: Context,
  ) {}

  private append(sessionId: string, record: AgentSessionRecord) {
    return this.ctx.sessions.append(sessionId, source, record);
  }

  async prepare(input: AgentLoopPrepareInput): Promise<void> {
    if (this.surface) throw new Error("Agent loop surface is already prepared");
    const context = await this.ctx.modelContext.build(input.sessionId, input.signal);
    const availableTools = new Map(this.ctx.tools.entries());
    const allowedTools = await this.ctx.toolPolicy.allowedTools(
      input.sessionId,
      context.tools.map((tool) => tool.name),
    );
    input.signal.throwIfAborted();
    const tools = context.tools.filter((tool) =>
      allowedTools.has(tool.name) && availableTools.has(tool.name));
    const provider = this.ctx.providers.require(
      await this.ctx.modelRouting.resolve(input.sessionId),
    );
    this.surface = {
      systemPrompt: context.systemPrompt,
      tools,
      sources: [
        ...context.sources.filter((entry) => !entry.startsWith("tool:")),
        ...tools.map((tool) => `tool:${tool.name}`),
      ],
      generation: surfaceGeneration(context.generation, provider.requestIdentity, tools),
      provider,
      executors: new Map(tools.map((tool) => [tool.name, availableTools.get(tool.name)!])),
      replay: context.replay,
    };
  }

  async run(input: AgentLoopInput): Promise<AgentRunResult> {
    const signal = input.signal;
    const runId = input.runId;
    let activeMessageId: string | undefined;
    let unfinishedCalls: ToolCallContent[] = [];
    let previousMessages: readonly ModelMessage[] | undefined;
    let runCacheKey: string | undefined;
    const surface = this.surface;
    if (!surface) throw new Error("Agent loop must be prepared before it runs");

    try {
      while (true) {
        signal.throwIfAborted();
        const messages = await surface.replay(signal);
        if (previousMessages) assertAppendOnlyTranscript(previousMessages, messages);
        const requestCache = cacheHint(input.sessionId, surface.systemPrompt, messages);
        if (runCacheKey && requestCache.key !== runCacheKey) {
          throw new Error("model cache affinity changed during an active run");
        }
        runCacheKey = requestCache.key;
        const receipt: CompositionReceiptRecord = {
          type: "context/composition-receipt",
          data: {
            generation: surface.generation,
            sources: surface.sources,
            cache: requestCache,
          },
        };
        await this.ctx.sessions.append(input.sessionId, {
          pluginId: "context-projection",
          generation: surface.generation,
        }, receipt);

        const messageId = randomUUID();
        activeMessageId = messageId;
        await this.append(input.sessionId, {
          type: "agent/assistant-started",
          data: { messageId },
        });

        const calls: ToolCallContent[] = [];
        const callIds = new Set<string>();
        let stopReason: "stop" | "tool_calls" | undefined;
        let usage: ModelUsage | undefined;
        for await (const event of surface.provider.stream({
          messages,
          tools: surface.tools,
          systemPrompt: surface.systemPrompt,
          cache: requestCache,
          signal,
        })) {
          signal.throwIfAborted();
          if (stopReason) throw new Error("provider emitted output after done");
          if (event.type === "text_delta") {
            await this.append(input.sessionId, {
              type: "agent/assistant-text-delta",
              data: { messageId, delta: event.delta },
            });
          } else if (event.type === "tool_call") {
            if (!event.call.callId || !event.call.name) {
              throw new Error("provider emitted an invalid tool call identity");
            }
            if (callIds.has(event.call.callId)) {
              throw new Error(`provider emitted duplicate tool call id: ${event.call.callId}`);
            }
            callIds.add(event.call.callId);
            calls.push(event.call);
            await this.append(input.sessionId, {
              type: "agent/assistant-tool-call",
              data: { messageId, call: event.call },
            });
          } else if (event.type === "usage") {
            if (usage) throw new Error("provider emitted multiple usage summaries");
            usage = event.usage;
          } else {
            stopReason = event.stopReason;
          }
        }

        if (!stopReason) throw new Error("provider stream ended without done");
        if ((stopReason === "tool_calls") !== Boolean(calls.length)) {
          throw new Error("provider tool calls do not match its stop reason");
        }
        if (usage) {
          await this.append(input.sessionId, {
            type: "agent/model-usage",
            data: { messageId, ...usage },
          });
        }
        await this.append(input.sessionId, {
          type: "agent/assistant-completed",
          data: { messageId, stopReason },
        });
        activeMessageId = undefined;

        if (!calls.length) {
          return { runId, status: "completed" };
        }

        unfinishedCalls = [...calls];
        const callErrors = new Map(calls.map((call) => {
          const tool = surface.executors.get(call.name);
          return [call.callId, tool ? schemaError(tool.inputSchema, call.input) : undefined] as const;
        }));
        for (const call of calls) {
          signal.throwIfAborted();
          let result: ToolResult;
          const tool = surface.executors.get(call.name);
          const validationError = callErrors.get(call.callId);
          try {
            result = validationError
              ? {
                  content: [{ type: "text", text: `invalid tool input: ${validationError}` }],
                  isError: true,
                }
              : tool
              ? await tool.execute(call.input, {
                  sessionId: input.sessionId,
                  callId: call.callId,
                  signal,
                })
              : {
                  content: [{
                    type: "text",
                    text: `tool not available in the frozen run surface: ${call.name}`,
                  }],
                  isError: true,
                };
          } catch (error) {
            result = {
              content: [{ type: "text", text: error instanceof Error ? error.message : String(error) }],
              isError: true,
            };
          }
          await this.append(input.sessionId, {
            type: "agent/tool-result",
            data: {
              callId: call.callId,
              name: call.name,
              content: result.content,
              isError: result.isError ?? false,
              ...(result.meta === undefined ? {} : { meta: result.meta }),
            },
          });
          unfinishedCalls.shift();
        }
        previousMessages = messages;
      }
    } catch (error) {
      const cancelled = signal.aborted;
      if (activeMessageId) {
        await this.append(input.sessionId, {
          type: "agent/assistant-completed",
          data: { messageId: activeMessageId, stopReason: cancelled ? "cancelled" : "error" },
        });
      }
      for (const call of unfinishedCalls) {
        await this.append(input.sessionId, {
          type: "agent/tool-result",
          data: {
            callId: call.callId,
            name: call.name,
            content: [{ type: "text", text: cancelled ? "Tool execution cancelled" : "Tool execution failed" }],
            isError: true,
          },
        });
      }
      return { runId, status: cancelled ? "cancelled" : "failed" };
    }
  }
}

export const defaultAgentLoopPlugin: Plugin<DefaultAgentLoopConfig> =
  function defaultAgentLoop(ctx: Context, config: DefaultAgentLoopConfig) {
    ctx.agents.register(config.agentId ?? "default", () => new DefaultAgentLoop(ctx));
  };

defaultAgentLoopPlugin.inject = [
  "agents",
  "modelContext",
  "modelRouting",
  "providers",
  "sessions",
  "tools",
  "toolPolicy",
];
