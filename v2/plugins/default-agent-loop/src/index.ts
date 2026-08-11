import { createHash, randomUUID } from "node:crypto";
import type {
  AgentLoop,
  AgentLoopInput,
  AgentRunResult,
  AgentSessionRecord,
  CompositionReceiptRecord,
  EventSource,
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

function surfaceGeneration(systemPrompt: string, tools: readonly ModelTool[]): string {
  return `sha256:${createHash("sha256")
    .update(JSON.stringify({ systemPrompt, tools }))
    .digest("hex")}`;
}

class DefaultAgentLoop implements AgentLoop {
  constructor(
    private readonly ctx: Context,
  ) {}

  private append(sessionId: string, record: AgentSessionRecord) {
    return this.ctx.sessions.append(sessionId, source, record);
  }

  async run(input: AgentLoopInput): Promise<AgentRunResult> {
    const signal = input.signal;
    const runId = input.runId;
    let activeMessageId: string | undefined;
    let unfinishedCalls: ToolCallContent[] = [];
    let surface: {
      systemPrompt: string;
      tools: ModelTool[];
      sources: string[];
      generation: string;
      provider: ModelProvider;
      executors: ReadonlyMap<string, ToolDefinition>;
    } | undefined;

    try {
      while (true) {
        signal.throwIfAborted();
        const context = await this.ctx.modelContext.build(input.sessionId, signal);
        if (!surface) {
          const availableTools = new Map(this.ctx.tools.entries());
          const allowedTools = await this.ctx.toolPolicy.allowedTools(
            input.sessionId,
            context.tools.map((tool) => tool.name),
          );
          const tools = context.tools.filter((tool) =>
            allowedTools.has(tool.name) && availableTools.has(tool.name));
          const sources = [
            ...context.sources.filter((entry) => !entry.startsWith("tool:")),
            ...tools.map((tool) => `tool:${tool.name}`),
          ];
          surface = {
            systemPrompt: context.systemPrompt,
            tools,
            sources,
            generation: surfaceGeneration(context.systemPrompt, tools),
            provider: this.ctx.providers.require(
              await this.ctx.modelRouting.resolve(input.sessionId),
            ),
            executors: new Map(tools.map((tool) => [tool.name, availableTools.get(tool.name)!])),
          };
        }
        const requestCache = cacheHint(input.sessionId, surface.systemPrompt, context.messages);
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
          messages: context.messages,
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
        for (const call of calls) {
          signal.throwIfAborted();
          let result: ToolResult;
          const tool = surface.executors.get(call.name);
          try {
            result = tool
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
            },
          });
          unfinishedCalls.shift();
        }
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
