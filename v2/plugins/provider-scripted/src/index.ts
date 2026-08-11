import type {
  ModelProvider,
  ModelRequest,
  ModelStreamEvent,
  ToolCallContent,
} from "@wuu-v2/contracts";
import { type Context, type Plugin } from "@wuu-v2/kernel";

export interface ScriptedRound {
  text?: string;
  toolCalls?: ToolCallContent[];
}

export interface ScriptedProviderConfig {
  id?: string;
  rounds: ScriptedRound[];
}

class ScriptedProvider implements ModelProvider {
  readonly id: string;
  readonly displayName = "Scripted";
  private round = 0;

  constructor(id: string, private readonly rounds: ScriptedRound[]) {
    this.id = id;
  }

  async *stream(request: ModelRequest): AsyncIterable<ModelStreamEvent> {
    request.signal.throwIfAborted();
    const round = this.rounds[this.round++];
    if (!round) throw new Error("scripted provider has no remaining round");
    if (round.text) yield { type: "text_delta", delta: round.text };
    for (const call of round.toolCalls ?? []) yield { type: "tool_call", call };
    yield {
      type: "done",
      stopReason: round.toolCalls?.length ? "tool_calls" : "stop",
    };
  }
}

export const scriptedProviderPlugin: Plugin<ScriptedProviderConfig> =
  function providerScripted(ctx: Context, config: ScriptedProviderConfig) {
    const provider = new ScriptedProvider(config.id ?? "scripted", config.rounds);
    ctx.providers.register(provider.id, provider);
  };

scriptedProviderPlugin.inject = ["providers"];
