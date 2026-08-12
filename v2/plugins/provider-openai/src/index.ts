import type {
  AssistantContent,
  JsonValue,
  ModelMessage,
  ModelProvider,
  ModelRequest,
  ModelStreamEvent,
  ModelUsage,
  ToolCallContent,
} from "@wuu-v2/contracts";
import { type Context, type Plugin } from "@wuu-v2/kernel";

export interface OpenAIProviderConfig {
  apiKey: string;
  id?: string;
  model: string;
  baseUrl?: string;
  promptCaching?: boolean;
  reportUsage?: boolean;
}

interface ToolCallAccumulator {
  id: string;
  name: string;
  arguments: string;
}

function toOpenAIMessage(message: ModelMessage): Record<string, unknown> {
  if (message.role === "user") return message;
  if (message.role === "tool") {
    return {
      role: "tool",
      tool_call_id: message.callId,
      content: message.content.map((item) => item.text).join("\n"),
    };
  }

  const text = message.content
    .filter((item): item is Extract<AssistantContent, { type: "text" }> =>
      item.type === "text",
    )
    .map((item) => item.text)
    .join("");
  const calls = message.content.filter(
    (item): item is ToolCallContent => item.type === "tool_call",
  );
  return {
    role: "assistant",
    content: text || null,
    ...(calls.length
      ? {
          tool_calls: calls.map((call) => ({
            id: call.callId,
            type: "function",
            function: {
              name: call.name,
              arguments: JSON.stringify(call.input),
            },
          })),
        }
      : {}),
  };
}

async function* readSse(response: Response): AsyncIterable<string> {
  if (!response.body) throw new Error("model response has no body");
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  let dataLines: string[] = [];
  while (true) {
    const { done, value } = await reader.read();
    buffer += decoder.decode(value, { stream: !done });
    let newline = buffer.indexOf("\n");
    while (newline >= 0) {
      let line = buffer.slice(0, newline);
      buffer = buffer.slice(newline + 1);
      if (line.endsWith("\r")) line = line.slice(0, -1);
      if (!line) {
        if (dataLines.length) {
          yield dataLines.join("\n");
          dataLines = [];
        }
      } else if (line.startsWith("data:")) {
        dataLines.push(line.slice(5).trimStart());
      }
      newline = buffer.indexOf("\n");
    }
    if (done) break;
  }
  let tail = buffer;
  if (tail.endsWith("\r")) tail = tail.slice(0, -1);
  if (tail.startsWith("data:")) dataLines.push(tail.slice(5).trimStart());
  if (dataLines.length) {
    yield dataLines.join("\n");
  }
}

class OpenAIProvider implements ModelProvider {
  readonly id: string;
  readonly displayName: string;
  readonly requestIdentity: string;

  constructor(private readonly config: OpenAIProviderConfig) {
    this.id = config.id ?? "openai";
    this.displayName = config.model;
    this.requestIdentity = JSON.stringify({
      baseUrl: (config.baseUrl ?? "https://api.openai.com/v1").replace(/\/$/, ""),
      model: config.model,
      promptCaching: config.promptCaching ?? "auto",
    });
  }

  async *stream(request: ModelRequest): AsyncIterable<ModelStreamEvent> {
    const baseUrl = this.config.baseUrl ?? "https://api.openai.com/v1";
    const officialEndpoint = !this.config.baseUrl
      || new URL(baseUrl).hostname === "api.openai.com";
    const promptCaching = this.config.promptCaching ?? officialEndpoint;
    const reportUsage = this.config.reportUsage ?? officialEndpoint;
    const messages = [
      ...(request.systemPrompt
        ? [{ role: "system", content: request.systemPrompt }]
        : []),
      ...request.messages.map(toOpenAIMessage),
    ];
    const response = await fetch(`${baseUrl.replace(/\/$/, "")}/chat/completions`, {
      method: "POST",
      headers: {
        authorization: `Bearer ${this.config.apiKey}`,
        "content-type": "application/json",
      },
      body: JSON.stringify({
        model: this.config.model,
        stream: true,
        ...(reportUsage ? { stream_options: { include_usage: true } } : {}),
        ...(promptCaching ? { prompt_cache_key: request.cache.key } : {}),
        messages,
        tools: request.tools.map((tool) => ({
          type: "function",
          function: {
            name: tool.name,
            description: tool.description,
            parameters: tool.inputSchema,
          },
        })),
      }),
      signal: request.signal,
    });
    if (!response.ok) {
      throw new Error(`model request failed (${response.status}): ${await response.text()}`);
    }

    const calls = new Map<number, ToolCallAccumulator>();
    let stopReason: "stop" | "tool_calls" | undefined;
    let usage: ModelUsage | undefined;
    for await (const data of readSse(response)) {
      request.signal.throwIfAborted();
      if (data === "[DONE]") break;
      const chunk = JSON.parse(data) as {
        choices?: Array<{
          delta?: {
            content?: string;
            tool_calls?: Array<{
              index: number;
              id?: string;
              function?: { name?: string; arguments?: string };
            }>;
          };
          finish_reason?: string | null;
        }>;
        usage?: {
          prompt_tokens?: number;
          completion_tokens?: number;
          prompt_tokens_details?: {
            cached_tokens?: number;
          };
        };
      };
      if (chunk.usage) {
        // `cached_tokens` is the only settled cache accounting defined by the
        // official Chat Completions response. Compatible endpoints often add
        // fields with different meanings, so they remain ordinary input usage.
        const cacheReadTokens = officialEndpoint
          ? chunk.usage.prompt_tokens_details?.cached_tokens ?? 0
          : 0;
        usage = {
          inputTokens: Math.max(0, (chunk.usage.prompt_tokens ?? 0) - cacheReadTokens),
          outputTokens: chunk.usage.completion_tokens ?? 0,
          cacheReadTokens,
          cacheWriteTokens: 0,
        };
      }
      const choice = chunk.choices?.[0];
      if (!choice) continue;
      if (choice.delta?.content) {
        yield { type: "text_delta", delta: choice.delta.content };
      }
      for (const update of choice.delta?.tool_calls ?? []) {
        const current = calls.get(update.index) ?? { id: "", name: "", arguments: "" };
        if (update.id) current.id = update.id;
        if (update.function?.name) current.name += update.function.name;
        if (update.function?.arguments) current.arguments += update.function.arguments;
        calls.set(update.index, current);
      }
      if (choice.finish_reason) {
        if (choice.finish_reason !== "stop" && choice.finish_reason !== "tool_calls") {
          throw new Error(`model response ended with ${choice.finish_reason}`);
        }
        if (stopReason) throw new Error("model response emitted multiple finish reasons");
        stopReason = choice.finish_reason;
      }
    }

    if (!stopReason) throw new Error("model response ended without a finish reason");
    if ((stopReason === "tool_calls") !== Boolean(calls.size)) {
      throw new Error("model tool calls do not match its finish reason");
    }
    for (const [, call] of [...calls.entries()].sort(([left], [right]) => left - right)) {
      if (!call.id || !call.name) throw new Error("incomplete model tool call");
      let input: JsonValue;
      try {
        input = JSON.parse(call.arguments || "{}") as JsonValue;
      } catch {
        throw new Error(`invalid JSON arguments for tool ${call.name}`);
      }
      yield {
        type: "tool_call",
        call: { type: "tool_call", callId: call.id, name: call.name, input },
      };
    }
    if (usage) yield { type: "usage", usage };
    yield { type: "done", stopReason };
  }
}

export const openAIProviderPlugin: Plugin<OpenAIProviderConfig> = function providerOpenAI(
  ctx: Context,
  config: OpenAIProviderConfig,
) {
  if (!config.apiKey) throw new Error("OpenAI API key is required");
  const provider = new OpenAIProvider(config);
  ctx.providers.register(provider.id, provider);
};

openAIProviderPlugin.inject = ["providers"];
