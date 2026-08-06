import { describe, expect, it } from "vitest";
import type { ComposerGoalSummary } from "../shared/protocol";
import {
  goalSummaryForActiveThread,
  requireGoalMutationSuccess,
} from "./GoalSummaryState";

const summary: ComposerGoalSummary = {
  id: "goal-a",
  thread_id: "thread-a",
  text: "Ship the goal flow",
  status: "active",
};

describe("goalSummaryForActiveThread", () => {
  it("hides a previous thread goal as soon as the active thread changes", () => {
    expect(goalSummaryForActiveThread(summary, "thread-b")).toBeNull();
  });

  it("returns only the goal owned by the active thread", () => {
    expect(goalSummaryForActiveThread(summary, "thread-a")).toBe(summary);
    expect(goalSummaryForActiveThread(summary, undefined)).toBeNull();
  });
});

describe("requireGoalMutationSuccess", () => {
  it("rejects resolved mutation responses that report ok=false", () => {
    expect(() => requireGoalMutationSuccess({ ok: false }, "pause")).toThrow(
      "Goal pause was not applied",
    );
  });

  it("accepts an applied mutation", () => {
    expect(() => requireGoalMutationSuccess({ ok: true }, "pause")).not.toThrow();
  });
});
