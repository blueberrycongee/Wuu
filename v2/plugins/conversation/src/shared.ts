import type { JsonValue } from "@wuu-v2/contracts";

export interface ConversationMessageItem {
  [key: string]: JsonValue;
  kind: "message";
  id: string;
  role: "user" | "assistant";
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

export type ConversationItem = ConversationMessageItem | ConversationToolItem;

export interface ConversationValue {
  [key: string]: JsonValue;
  items: ConversationItem[];
  running: boolean;
}
