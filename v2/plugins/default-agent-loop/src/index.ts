import { randomUUID } from "node:crypto";
import type {
  AgentLoop,
  AgentLoopInput,
  AgentRunResult,
  AgentSessionRecord,
  CompositionReceiptRecord,
  EventSource,
  ToolCallContent,
  ToolResult,
} from "@wuu-v2/contracts";
import { type Context, type Plugin } from "@wuu-v2/kernel";

export interface DefaultAgentLoopConfig {
  providerId: string;
}

const source: EventSource = {
  pluginId: "default-agent-loop",
  generation: "v1",
};

class DefaultAgentLoop implements AgentLoop {
  constructor(
    private readonly ctx: Context,
    private readonly providerId: string,
  ) {}

  private append(sessionId: string, record: AgentSessionRecord) {
    return this.ctx.sessions.append(sessionId, source, record);
  }

  async run(input: AgentLoopInput): Promise<AgentRunResult> {
    const signal = input.signal;
    const runId = input.runId;
    let activeMessageId: string | undefined;
    let unfinishedCalls: ToolCallContent[] = [];

    try {
      while (true) {
        signal.throwIfAborted();
        const context = await this.ctx.modelContext.build(input.sessionId, signal);
        const receipt: CompositionReceiptRecord = {
          type: "context/composition-receipt",
          data: { generation: context.generation, sources: context.sources },
        };
        await this.ctx.sessions.append(input.sessionId, {
          pluginId: "context-projection",
          generation: context.generation,
        }, receipt);

        const messageId = randomUUID();
        activeMessageId = messageId;
        await this.append(input.sessionId, {
          type: "agent/assistant-started",
          data: { messageId },
        });

        const calls: ToolCallContent[] = [];
        let stopReason: "stop" | "tool_calls" = "stop";
        const provider = this.ctx.providers.require(this.providerId);
        for await (const event of provider.stream({
          messages: context.messages,
          tools: context.tools,
          systemPrompt: context.systemPrompt,
          signal,
        })) {
          signal.throwIfAborted();
          if (event.type === "text_delta") {
            await this.append(input.sessionId, {
              type: "agent/assistant-text-delta",
              data: { messageId, delta: event.delta },
            });
          } else if (event.type === "tool_call") {
            calls.push(event.call);
            await this.append(input.sessionId, {
              type: "agent/assistant-tool-call",
              data: { messageId, call: event.call },
            });
          } else {
            stopReason = event.stopReason;
          }
        }

        if (stopReason === "tool_calls" && !calls.length) {
          throw new Error("provider ended with tool_calls but emitted no tool call");
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
          const tool = this.ctx.tools.get(call.name);
          try {
            result = tool
              ? await tool.execute(call.input, {
                  sessionId: input.sessionId,
                  callId: call.callId,
                  signal,
                })
              : { content: [{ type: "text", text: `unknown tool: ${call.name}` }], isError: true };
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
    ctx.providers.require(config.providerId);
    ctx.agents.register("default", () => new DefaultAgentLoop(ctx, config.providerId));
  };

defaultAgentLoopPlugin.inject = [
  "agents",
  "modelContext",
  "providers",
  "sessions",
  "tools",
];
