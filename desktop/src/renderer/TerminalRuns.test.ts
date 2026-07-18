import { describe, expect, it } from "vitest";
import type { Thread, ThreadItem, Turn } from "../shared/protocol";
import {
  agentRunGroupsForThread,
  agentRunsForTurn,
  isCommandToolCall,
  preferredAgentRun,
  selectAgentRun,
} from "./TerminalRuns";

function turn(items: ThreadItem[], status: Turn["status"] = "completed"): Turn {
  return { id: "turn-1", items, items_view: "full", status };
}

function commandItem(overrides: Partial<ThreadItem> = {}): ThreadItem {
  return {
    id: "call-1",
    type: "tool_call",
    status: "completed",
    name: "bash",
    arguments: JSON.stringify({ command: "npm test" }),
    display: { kind: "command", capability: "command.bash" },
    result: JSON.stringify({
      exit_code: 0,
      duration_ms: 1234,
      stdout_tail: "ok\n",
      stderr_tail: "",
      full_log_ref: "/tmp/run.log",
    }),
    ...overrides,
  };
}

describe("terminal run records", () => {
  it("recognizes command capabilities and legacy command tools only", () => {
    expect(isCommandToolCall(commandItem())).toBe(true);
    expect(isCommandToolCall(commandItem({ name: "custom", display: { capability: "command.background" } }))).toBe(true);
    expect(isCommandToolCall(commandItem({ name: "run_shell", display: undefined }))).toBe(true);
    expect(isCommandToolCall(commandItem({ name: "grep", display: { kind: "search" } }))).toBe(false);
    expect(isCommandToolCall(commandItem({ type: "agent_message" }))).toBe(false);
  });

  it("extracts retained output and command metadata", () => {
    const [run] = agentRunsForTurn("thread-1", turn([commandItem()]));

    expect(run).toMatchObject({
      kind: "agent_run",
      threadID: "thread-1",
      turnID: "turn-1",
      toolCallID: "call-1",
      command: "npm test",
      capability: "command.bash",
      status: "completed",
      stdout: "ok\n",
      exitCode: 0,
      durationMs: 1234,
      timedOut: false,
      truncated: false,
      fullLogRef: "/tmp/run.log",
    });
    expect(run.output).toBeUndefined();
  });

  it("marks non-zero exits and timeouts as failed even when the item completed", () => {
    const failed = commandItem({
      result: JSON.stringify({ exit_code: 2, stderr_tail: "failed\n" }),
    });
    const timedOut = commandItem({
      id: "call-2",
      result_detail: {
        structured_content: {
          exit_code: 0,
          timed_out: true,
          output: "timeout",
          stdout_tail_truncated: true,
        },
      },
      result: undefined,
    });
    const runs = agentRunsForTurn("thread-1", turn([failed, timedOut]));

    expect(runs[0]).toMatchObject({ status: "failed", exitCode: 2, stderr: "failed\n" });
    expect(runs[1]).toMatchObject({ status: "failed", timedOut: true, truncated: true, output: "timeout" });
  });

  it("preserves plain legacy output and interrupted status", () => {
    const [run] = agentRunsForTurn(
      "thread-1",
      turn([
        commandItem({
          status: "in_progress",
          result: "partial output",
        }),
      ], "interrupted"),
    );

    expect(run.status).toBe("interrupted");
    expect(run.output).toBe("partial output");
  });

  it("lists settled turns only and prefers the last failed run", () => {
    const completed = turn([
      commandItem(),
      commandItem({
        id: "call-2",
        arguments: JSON.stringify({ command: "npm run lint" }),
        result: JSON.stringify({ exit_code: 1, stderr_tail: "lint failed" }),
      }),
      commandItem({
        id: "call-3",
        arguments: JSON.stringify({ command: "npm run build" }),
      }),
    ]);
    const running = { ...turn([commandItem({ id: "call-live" })], "in_progress"), id: "turn-live" };
    const thread: Pick<Thread, "id" | "turns"> = {
      id: "thread-1",
      turns: [turn([{ id: "message-1", type: "agent_message", text: "done" }]), completed, running],
    };
    const groups = agentRunGroupsForThread(thread);

    expect(groups).toHaveLength(1);
    expect(groups[0].turnNumber).toBe(2);
    expect(preferredAgentRun(groups[0].runs)?.toolCallID).toBe("call-2");
    expect(selectAgentRun(groups, { threadID: "thread-1", turnID: "turn-1" })?.toolCallID).toBe("call-2");
    expect(selectAgentRun(groups, {
      threadID: "thread-1",
      turnID: "turn-1",
      toolCallID: "call-3",
    })?.toolCallID).toBe("call-3");
    expect(selectAgentRun(groups, {
      threadID: "another-thread",
      turnID: "turn-1",
    })).toBeUndefined();
  });
});
