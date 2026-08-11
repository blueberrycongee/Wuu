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

export type SessionRecord =
  | {
      kind: "user_message";
      messageId: string;
      content: TextContent[];
    }
  | {
      kind: "assistant_started";
      messageId: string;
    }
  | {
      kind: "assistant_text_delta";
      messageId: string;
      delta: string;
    }
  | {
      kind: "assistant_tool_call";
      messageId: string;
      call: ToolCallContent;
    }
  | {
      kind: "assistant_completed";
      messageId: string;
      stopReason: "stop" | "tool_calls" | "cancelled" | "error";
    }
  | {
      kind: "tool_result";
      callId: string;
      name: string;
      content: TextContent[];
      isError: boolean;
    }
  | {
      kind: "run_state";
      runId: string;
      state: "started" | "completed" | "cancelled" | "failed";
      error?: string;
    }
  | {
      kind: "composition_receipt";
      generation: string;
      sources: string[];
    };

export interface SessionEvent<R extends SessionRecord = SessionRecord> {
  sessionId: string;
  seq: number;
  time: string;
  source: string;
  record: R;
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
  signal: AbortSignal;
}

export type ModelStreamEvent =
  | { type: "text_delta"; delta: string }
  | { type: "tool_call"; call: ToolCallContent }
  | { type: "done"; stopReason: "stop" | "tool_calls" };

export interface ModelProvider {
  readonly id: string;
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
  inputSchema: JsonValue;
  execute(input: JsonValue, execution: ToolExecution): Promise<ToolResult>;
}

export interface AgentRunInput {
  sessionId: string;
  text: string;
  signal?: AbortSignal;
}

export interface AgentRunResult {
  runId: string;
  status: "completed" | "cancelled" | "failed";
}

export interface AgentLoop {
  run(input: AgentRunInput): Promise<AgentRunResult>;
}

export type AgentLoopFactory = () => AgentLoop;
