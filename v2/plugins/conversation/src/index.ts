import type { AgentSessionRecord, JsonValue } from "@wuu-v2/contracts";
import { type Context, type Plugin } from "@wuu-v2/kernel";

type ConversationState = {
  messages: Array<{ id: string; role: "user" | "assistant"; text: string; status: string }>;
  running: boolean;
};

export const conversationProjectionPlugin: Plugin = function conversationProjection(ctx: Context) {
  ctx.projections.register("conversation", (current, event) => {
    const state = (current as ConversationState | undefined) ?? { messages: [], running: false };
    const next: ConversationState = { ...state, messages: state.messages.map((message) => ({ ...message })) };
    const record = event.record as AgentSessionRecord;
    if (record.type === "agent/user-message") {
      next.messages.push({ id: record.data.messageId, role: "user", text: record.data.content.map((item) => item.text).join("\n"), status: "complete" });
    } else if (record.type === "agent/assistant-started") {
      next.messages.push({ id: record.data.messageId, role: "assistant", text: "", status: "streaming" });
    } else if (record.type === "agent/assistant-text-delta") {
      const message = next.messages.find((item) => item.id === record.data.messageId);
      if (message) message.text += record.data.delta;
    } else if (record.type === "agent/assistant-completed") {
      const index = next.messages.findIndex((item) => item.id === record.data.messageId);
      const message = next.messages[index];
      if (message && record.data.stopReason === "tool_calls" && !message.text) {
        next.messages.splice(index, 1);
      } else if (message) {
        message.status = record.data.stopReason;
      }
    } else if (record.type === "agent/run-state") {
      next.running = record.data.state === "started";
    }
    return next as unknown as JsonValue;
  });
};

conversationProjectionPlugin.inject = ["projections"];
