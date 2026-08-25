import { describe, expect, it } from "vitest";
import {
  partitionAttentionThreads,
  reconcileSidebarSectionOrder,
  reorderSidebarSections,
  SIDEBAR_SECTION_PINNED,
} from "./AppSidebar";
import { SCRATCH_PSEUDO_PROJECT_ID } from "./AppState";
import type { ThreadSummary } from "./AppState";

function thread(overrides: Partial<ThreadSummary> = {}): ThreadSummary {
  return {
    id: "thread",
    title: "Thread",
    preview: "Thread",
    status: "completed",
    turns: [{ id: "turn-1", status: "completed" }],
    turn_count: 1,
    archived: false,
    pinned: false,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    ...overrides,
  } as ThreadSummary;
}

describe("partitionAttentionThreads", () => {
  it("puts running sessions before unread sessions and excludes read sessions", () => {
    const running = thread({ id: "running", status: "in_progress" });
    const unread = thread({ id: "unread", updated_at: "2026-01-02T00:00:00Z" });
    const read = thread({ id: "read" });

    const attention = partitionAttentionThreads(
      [unread, read, running],
      undefined,
      undefined,
      { read: "turn-1" },
    );

    expect(attention.running.map(({ id }) => id)).toEqual(["running"]);
    expect(attention.unread.map(({ id }) => id)).toEqual(["unread"]);
  });

  it("includes the active running session without treating an active settled session as unread", () => {
    const activeRunning = thread({ id: "active-running", status: "in_progress" });
    const backgroundRunning = thread({ id: "background-running", status: "in_progress" });
    const activeUnread = thread({ id: "active-unread" });

    const runningAttention = partitionAttentionThreads(
      [activeRunning, backgroundRunning],
      activeRunning.id,
      undefined,
      {},
    );
    const settledAttention = partitionAttentionThreads(
      [activeUnread],
      activeUnread.id,
      undefined,
      {},
    );

    expect(runningAttention.running.map(({ id }) => id)).toEqual([
      "active-running",
      "background-running",
    ]);
    expect(settledAttention.unread).toEqual([]);
  });
});

describe("reconcileSidebarSectionOrder", () => {
  it("builds the default order from scratch and project ids", () => {
    expect(
      reconcileSidebarSectionOrder(undefined, ["project-1", "project-2"]),
    ).toEqual([SCRATCH_PSEUDO_PROJECT_ID, "project-1", "project-2"]);
  });

  it("preserves stored workspace order and appends newly seen projects", () => {
    expect(
      reconcileSidebarSectionOrder(
        ["project-2", SCRATCH_PSEUDO_PROJECT_ID, "project-1"],
        ["project-1", "project-2", "project-3"],
      ),
    ).toEqual([
      "project-2",
      SCRATCH_PSEUDO_PROJECT_ID,
      "project-1",
      "project-3",
    ]);
  });

  it("drops retired fixed, unknown, and duplicate ids", () => {
    expect(
      reconcileSidebarSectionOrder(
        [
          "__wuu_group__",
          SIDEBAR_SECTION_PINNED,
          "project-2",
          "__wuu_agents__",
          "project-2",
          "__unknown__",
        ],
        ["project-1", "project-2"],
      ),
    ).toEqual([SCRATCH_PSEUDO_PROJECT_ID, "project-2", "project-1"]);
  });

  it("restores scratch without disturbing a stored project prefix", () => {
    expect(
      reconcileSidebarSectionOrder(["project-2", "project-1"], ["project-1", "project-2"]),
    ).toEqual([SCRATCH_PSEUDO_PROJECT_ID, "project-2", "project-1"]);
  });

  it("preserves the stored reference when reconciliation makes no changes", () => {
    const stored = [SCRATCH_PSEUDO_PROJECT_ID, "project-1", "project-2"];

    expect(
      reconcileSidebarSectionOrder(stored, ["project-1", "project-2"]),
    ).toBe(stored);
  });
});

describe("reorderSidebarSections", () => {
  it("moves one workspace section to another section's position", () => {
    expect(
      reorderSidebarSections(
        [SCRATCH_PSEUDO_PROJECT_ID, "project-1", "project-2"],
        "project-2",
        SCRATCH_PSEUDO_PROJECT_ID,
      ),
    ).toEqual(["project-2", SCRATCH_PSEUDO_PROJECT_ID, "project-1"]);
  });

  it("keeps the original order when a drag has no valid destination", () => {
    const order = [SCRATCH_PSEUDO_PROJECT_ID, "project-1"];
    expect(reorderSidebarSections(order, "project-1", null)).toBe(order);
    expect(reorderSidebarSections(order, "project-1", "unknown")).toBe(order);
    expect(reorderSidebarSections(order, "unknown", "project-1")).toBe(order);
  });
});
