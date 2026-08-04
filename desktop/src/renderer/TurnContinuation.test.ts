import { describe, expect, it } from "vitest";
import type { Thread, Turn } from "../shared/protocol";
import { canResumeInterruptedTurn } from "./TurnContinuation";

function threadWith(turn: Turn, orchestrationInterrupted = false): Thread {
  return {
    id: "thread-1",
    preview: "",
    model_provider: "fake",
    model: "fake-model",
    model_variant: "",
    model_effort: "",
    permission_mode: "default",
    cwd: "/tmp",
    status: "idle",
    orchestration_interrupted: orchestrationInterrupted,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    turns: [turn],
  };
}

describe("canResumeInterruptedTurn", () => {
  it("accepts an ordinary interrupted tail turn", () => {
    expect(canResumeInterruptedTurn(threadWith({
      id: "turn-1",
      status: "interrupted",
      items_view: "full",
      items: [],
    }))).toBe(true);
  });

  it("excludes interrupted subagent orchestration", () => {
    expect(canResumeInterruptedTurn(threadWith({
      id: "turn-1",
      status: "interrupted",
      items_view: "full",
      items: [],
    }, true))).toBe(false);
  });
});
