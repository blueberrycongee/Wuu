import { createElement } from "react";
import { describe, expect, it } from "vitest";
import type { ThreadItem, Turn } from "../shared/protocol";
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

function makeFrontendPreview(status: "completed" | "failed" = "completed"): ThreadItem {
  return {
    id: nextID("frontend-preview"),
    type: "tool_call",
    status,
    name: "render_frontend_preview",
    arguments: JSON.stringify({ version: 1, title: "Button", html: "<button>Save</button>" }),
    result: status === "completed" ? "Frontend preview ready." : undefined,
    error: status === "failed" ? "invalid frontend preview" : undefined,
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

function makeGeneratedQuery(text: string): ThreadItem {
  return {
    id: nextID("generated-query"),
    type: "user_message",
    status: "completed",
    text,
    read_only: true,
    origin: "plugin",
    presentation_kind: "query_bubble",
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

describe("buildAssistantTurnDisplay generated queries", () => {
  it("leaves generated queries to the user-message renderer", () => {
    const display = build(
      makeTurn("completed", [
        makeGeneratedQuery("后台结果二已更新"),
        makeCommentary("子代理 1: ok\n子代理 2: ok"),
        makeGeneratedQuery("后台结果一已更新"),
        makeCommentary("已确认"),
      ]),
    );
    expect(display.entries.map((entry) => entry.kind)).toEqual([
      "commentary",
      "commentary",
    ]);
  });

  it("does not duplicate generated queries beside tool activity", () => {
    const display = build(
      makeTurn("completed", [makeToolCall(), makeGeneratedQuery("检查已完成")]),
    );
    expect(display.entries.map((entry) => entry.kind)).toEqual(["activity"]);
  });

  it("does not duplicate generated queries beside reasoning", () => {
    const display = build(
      makeTurn("completed", [makeReasoning(), makeGeneratedQuery("检查已完成")]),
    );
    expect(display.entries).toHaveLength(1);
    expect(display.entries[0].item.type).toBe("reasoning");
  });

  it("does not create an assistant display for a generated-query-only turn", () => {
    expect(
      buildAssistantTurnDisplay(
        makeTurn("completed", [makeGeneratedQuery("一"), makeGeneratedQuery("二")]),
        undefined,
        stubRenderer,
      ),
    ).toBeUndefined();
  });

  it("ignores ordinary user messages too", () => {
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

describe("buildAssistantTurnDisplay frontend previews", () => {
  it("keeps a successful preview as standalone answer content between activities", () => {
    const preview = makeFrontendPreview();
    const display = build(makeTurn("completed", [makeToolCall(), preview, makeToolCall()]));
    expect(display.entries.map((entry) => entry.kind)).toEqual([
      "activity",
      "presentation",
      "activity",
    ]);
    expect(display.entries[1]).toMatchObject({
      item: preview,
      position: "answer",
      settled: true,
      streaming: false,
    });
    expect(display.entries[1].items).toBeUndefined();
    expect(display.hasAnswer).toBe(true);
    expect(display.missingReplyMessage).toBeUndefined();
  });

  it("treats a preview-only completed turn as an answered turn", () => {
    const display = build(makeTurn("completed", [makeFrontendPreview()]));
    expect(display.hasAnswer).toBe(true);
    expect(display.entries.map((entry) => entry.kind)).toEqual(["presentation"]);
    expect(display.missingReplyMessage).toBeUndefined();
  });

  it("leaves failed previews in ordinary process activity", () => {
    const display = build(makeTurn("failed", [makeFrontendPreview("failed")]));
    expect(display.entries).toHaveLength(1);
    expect(display.entries[0]).toMatchObject({ kind: "activity", position: "process" });
    expect(display.hasAnswer).toBe(false);
  });
});
