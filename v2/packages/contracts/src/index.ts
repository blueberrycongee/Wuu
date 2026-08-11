export type JsonValue =
  | null
  | boolean
  | number
  | string
  | JsonValue[]
  | { [key: string]: JsonValue };

export interface TextContent {
  type: "text";
  text: string;
}

export interface ToolCallContent {
  type: "tool_call";
  callId: string;
  name: string;
  input: JsonValue;
}

export type AssistantContent = TextContent | ToolCallContent;

export interface SessionRecord<Type extends string = string, Data = unknown> {
  type: Type;
  data: Data;
}

export type AgentSessionRecord =
  | SessionRecord<"agent/user-message", {
      messageId: string;
      content: TextContent[];
    }>
  | SessionRecord<"agent/assistant-started", {
      messageId: string;
    }>
  | SessionRecord<"agent/assistant-text-delta", {
      messageId: string;
      delta: string;
    }>
  | SessionRecord<"agent/assistant-tool-call", {
      messageId: string;
      call: ToolCallContent;
    }>
  | SessionRecord<"agent/assistant-completed", {
      messageId: string;
      stopReason: "stop" | "tool_calls" | "cancelled" | "error";
    }>
  | SessionRecord<"agent/tool-result", {
      callId: string;
      name: string;
      content: TextContent[];
      isError: boolean;
    }>
  | SessionRecord<"agent/model-usage", {
      messageId: string;
      inputTokens: number;
      outputTokens: number;
      cacheReadTokens: number;
      cacheWriteTokens: number;
    }>
  | SessionRecord<"agent/run-state", {
      runId: string;
      state: "started" | "completed" | "cancelled" | "failed" | "interrupted";
      error?: string;
    }>;

export type CompositionReceiptRecord = SessionRecord<
  "context/composition-receipt",
  {
      generation: string;
      sources: string[];
      cache: ModelCacheHint;
  }
>;

export type ModelContextSeedRecord = SessionRecord<
  "context/model-seed",
  {
    sourceSessionId: string;
    sourceSeq: number;
    messages: ModelMessage[];
  }
>;

export interface EventSource {
  pluginId: string;
  generation: string;
}

export interface SessionEvent<R extends SessionRecord = SessionRecord> {
  id: string;
  sessionId: string;
  seq: number;
  time: string;
  source: EventSource;
  record: R;
}

export interface ProjectionFrame {
  sessionId: string;
  lastDurableSeq: number;
  projections: Array<{
    key: string;
    seq: number;
    value?: JsonValue;
  }>;
}

export interface ModelTool {
  name: string;
  description: string;
  inputSchema: JsonValue;
}

export type ModelMessage =
  | { role: "user"; content: string }
  | { role: "assistant"; content: AssistantContent[] }
  | {
      role: "tool";
      callId: string;
      name: string;
      content: TextContent[];
      isError: boolean;
    };

export interface ModelRequest {
  messages: ModelMessage[];
  tools: ModelTool[];
  systemPrompt: string;
  cache: ModelCacheHint;
  signal: AbortSignal;
}

export interface ModelCacheHint {
  key: string;
  stableSystem: boolean;
  stablePrefixMessages: number;
  turnPrefixMessages: number;
}

export interface ModelUsage {
  /** Input tokens that were neither read from nor written to a provider cache. */
  inputTokens: number;
  outputTokens: number;
  cacheReadTokens: number;
  cacheWriteTokens: number;
}

export type ModelStreamEvent =
  | { type: "text_delta"; delta: string }
  | { type: "tool_call"; call: ToolCallContent }
  | { type: "usage"; usage: ModelUsage }
  | { type: "done"; stopReason: "stop" | "tool_calls" };

export interface ModelProvider {
  readonly id: string;
  readonly displayName?: string;
  stream(request: ModelRequest): AsyncIterable<ModelStreamEvent>;
}

export interface ToolExecution {
  sessionId: string;
  callId: string;
  signal: AbortSignal;
}

export interface ToolResult {
  content: TextContent[];
  isError?: boolean;
}

export interface ToolDefinition {
  name: string;
  description: string;
  access: "read" | "write" | "execute";
  inputSchema: JsonValue;
  execute(input: JsonValue, execution: ToolExecution): Promise<ToolResult>;
}

export interface AgentLoopInput {
  sessionId: string;
  runId: string;
  signal: AbortSignal;
}

export interface AgentRunResult {
  runId: string;
  status: "completed" | "cancelled" | "failed" | "interrupted";
}

export interface AgentLoop {
  run(input: AgentLoopInput): Promise<AgentRunResult>;
}

export type AgentLoopFactory = () => AgentLoop;
