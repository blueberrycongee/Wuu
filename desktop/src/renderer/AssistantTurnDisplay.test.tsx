import { createElement } from "react";
import { describe, expect, it } from "vitest";
import type { ThreadItem, Turn } from "../shared/protocol";
import { AGENT_NOTIFICATION_NAME } from "./AgentHandoff";
import { buildAssistantTurnDisplay } from "./AssistantTurnDisplay";

let idCounter = 0;

function nextID(prefix: string): string {
  idCounter += 1;
  return `${prefix}-${idCounter}`;
}

function makeTurn(status: Turn["status"], items: ThreadItem[]): Turn {
  return { id: "turn-1", items, items_view: "full", status };
}

function makeCommentary(text: string): ThreadItem {
  return {
    id: nextID("commentary"),
    type: "agent_message",
    status: "completed",
    phase: "commentary",
    role: "assistant",
    text,
  };
}

function makeReasoning(): ThreadItem {
  return {
    id: nextID("reasoning"),
    type: "reasoning",
    status: "completed",
    text: "thinking",
  };
}

function makeToolCall(): ThreadItem {
  return {
    id: nextID("tool"),
    type: "tool_call",
    status: "completed",
    name: "bash",
  };
}

function makeNotification(taskName: string, status = "completed"): ThreadItem {
  return {
    id: nextID("notification"),
    type: "user_message",
    name: AGENT_NOTIFICATION_NAME,
    status: "completed",
    text: JSON.stringify({
      author: `/root/${taskName}`,
      recipient: "/root",
      content: `<subagent_notification>\n${JSON.stringify({
        agent_path: `/root/${taskName}`,
        status: { task_name: taskName, status },
      })}\n</subagent_notification>`,
      trigger_turn: true,
    }),
  };
}

const stubRenderer = (): JSX.Element => createElement("div");

function build(turn: Turn) {
  const display = buildAssistantTurnDisplay(turn, undefined, stubRenderer);
  if (!display) {
    throw new Error("expected a display");
  }
  return display;
}

describe("buildAssistantTurnDisplay subagent notifications", () => {
  it("leaves notifications to the user-message renderer", () => {
    const display = build(
      makeTurn("completed", [
        makeNotification("ok_agent_two"),
        makeCommentary("子代理 1: ok\n子代理 2: ok"),
        makeNotification("ok_agent_one"),
        makeCommentary("已确认"),
      ]),
    );
    expect(display.entries.map((entry) => entry.kind)).toEqual([
      "commentary",
      "commentary",
    ]);
  });

  it("does not duplicate notifications beside tool activity", () => {
    const display = build(
      makeTurn("completed", [makeToolCall(), makeNotification("lint")]),
    );
    expect(display.entries.map((entry) => entry.kind)).toEqual(["activity"]);
  });

  it("does not duplicate notifications beside reasoning", () => {
    const display = build(
      makeTurn("completed", [makeReasoning(), makeNotification("lint")]),
    );
    expect(display.entries).toHaveLength(1);
    expect(display.entries[0].item.type).toBe("reasoning");
  });

  it("does not create an assistant display for a notification-only completed turn", () => {
    expect(
      buildAssistantTurnDisplay(
        makeTurn("completed", [makeNotification("one"), makeNotification("two")]),
        undefined,
        stubRenderer,
      ),
    ).toBeUndefined();
  });

  it("ignores user messages that are not agent handoffs", () => {
    const display = build(
      makeTurn("completed", [
        {
          id: nextID("user"),
          type: "user_message",
          status: "completed",
          text: "普通用户消息",
        },
        makeCommentary("回复"),
      ]),
    );
    expect(display.entries.map((entry) => entry.kind)).toEqual(["commentary"]);
  });
});
