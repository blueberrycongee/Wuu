import type {
  AssistantContent,
  JsonValue,
  ModelMessage,
  ModelProvider,
  ModelRequest,
  ModelStreamEvent,
  ToolCallContent,
} from "@wuu-v2/contracts";
import { type Context, type Plugin } from "@wuu-v2/kernel";

export interface OpenAIProviderConfig {
  apiKey: string;
  id?: string;
  model: string;
  baseUrl?: string;
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
  while (true) {
    const { done, value } = await reader.read();
    buffer += decoder.decode(value, { stream: !done });
    const frames = buffer.split("\n\n");
    buffer = frames.pop() ?? "";
    for (const frame of frames) {
      for (const line of frame.split("\n")) {
        if (line.startsWith("data:")) yield line.slice(5).trim();
      }
    }
    if (done) break;
  }
  if (buffer.trim()) {
    for (const line of buffer.split("\n")) {
      if (line.startsWith("data:")) yield line.slice(5).trim();
    }
  }
}

class OpenAIProvider implements ModelProvider {
  readonly id: string;
  readonly displayName: string;

  constructor(private readonly config: OpenAIProviderConfig) {
    this.id = config.id ?? "openai";
    this.displayName = config.model;
  }

  async *stream(request: ModelRequest): AsyncIterable<ModelStreamEvent> {
    const baseUrl = this.config.baseUrl ?? "https://api.openai.com/v1";
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
      };
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
