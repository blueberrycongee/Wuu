import type { JsonValue } from "@wuu-v2/contracts";

export interface ConversationMessageItem {
  [key: string]: JsonValue;
  kind: "message";
  id: string;
  role: "user" | "assistant";
  runId?: string;
  text: string;
  status: string;
}

export interface ConversationToolItem {
  [key: string]: JsonValue;
  kind: "tool";
  id: string;
  callId: string;
  name: string;
  input: JsonValue;
  result: string | null;
  status: string;
}

export interface ConversationStatusItem {
  [key: string]: JsonValue;
  kind: "status";
  id: string;
  text: string;
  status: string;
}

export type ConversationItem =
  | ConversationMessageItem
  | ConversationStatusItem
  | ConversationToolItem;

export interface ConversationValue {
  [key: string]: JsonValue;
  items: ConversationItem[];
  running: boolean;
  activeRunId?: string;
}
