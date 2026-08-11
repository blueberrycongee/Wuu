import type { AgentSessionRecord, JsonValue } from "@wuu-v2/contracts";
import { type Context, type Plugin } from "@wuu-v2/kernel";
import type { ConversationValue } from "./shared.js";

export const conversationProjectionPlugin: Plugin = function conversationProjection(ctx: Context) {
  ctx.projections.register("conversation", (current, event) => {
    const state = (current as ConversationValue | undefined) ?? { items: [], running: false };
    const next: ConversationValue = {
      ...state,
      items: state.items.map((item) => ({ ...item })),
    };
    const record = event.record as AgentSessionRecord;
    if (record.type === "agent/user-message") {
      next.items.push({
        kind: "message",
        id: record.data.messageId,
        role: "user",
        text: record.data.content.map((item) => item.text).join("\n"),
        status: "complete",
      });
    } else if (record.type === "agent/assistant-started") {
      next.items.push({
        kind: "message",
        id: record.data.messageId,
        role: "assistant",
        text: "",
        status: "streaming",
      });
    } else if (record.type === "agent/assistant-text-delta") {
      const message = next.items.find((item) => item.kind === "message" && item.id === record.data.messageId);
      if (message?.kind === "message") message.text += record.data.delta;
    } else if (record.type === "agent/assistant-tool-call") {
      next.items.push({
        kind: "tool",
        id: `tool:${record.data.call.callId}`,
        callId: record.data.call.callId,
        name: record.data.call.name,
        input: record.data.call.input,
        result: null,
        status: "running",
      });
    } else if (record.type === "agent/assistant-completed") {
      const index = next.items.findIndex((item) => item.kind === "message" && item.id === record.data.messageId);
      const message = next.items[index];
      if (message?.kind === "message" && record.data.stopReason === "tool_calls" && !message.text) {
        next.items.splice(index, 1);
      } else if (message?.kind === "message") {
        message.status = record.data.stopReason;
      }
    } else if (record.type === "agent/tool-result") {
      const tool = next.items.find((item) => item.kind === "tool" && item.callId === record.data.callId);
      if (tool?.kind === "tool") {
        tool.result = record.data.content.map((item) => item.text).join("\n");
        tool.status = record.data.isError ? "error" : "complete";
      }
    } else if (record.type === "agent/run-state") {
      next.running = record.data.state === "started";
      if (["cancelled", "failed", "interrupted"].includes(record.data.state)) {
        for (const item of next.items) {
          if (item.kind === "tool" && item.status === "running") item.status = record.data.state;
        }
      }
    }
    return next as unknown as JsonValue;
  });
};

conversationProjectionPlugin.inject = ["projections"];
