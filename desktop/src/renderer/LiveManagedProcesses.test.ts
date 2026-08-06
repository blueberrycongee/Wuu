import { afterEach, describe, expect, it, vi } from "vitest";
import type { ManagedProcessSummary, WuuDesktopApi } from "../shared/protocol";
import {
  isManagedProcessLive,
  liveManagedProcessList,
  managedProcessResourceID,
  preferManagedProcess,
  resetLiveManagedProcessStores,
} from "./LiveManagedProcesses";

afterEach(() => {
  resetLiveManagedProcessStores();
  delete (globalThis as { wuu?: WuuDesktopApi }).wuu;
  vi.useRealTimers();
});

function process(overrides: Partial<ManagedProcessSummary> = {}): ManagedProcessSummary {
  return {
    id: "proc-1",
    owner_kind: "main_agent",
    owner_id: "thread-1",
    lifecycle: "managed",
    status: "running",
    pid: 100,
    command: "npm run dev",
    cwd: "/repo",
    started_at: "2026-08-05T08:00:00Z",
    updated_at: "2026-08-05T08:00:00Z",
    ...overrides,
  };
}

describe("live managed processes", () => {
  // A record left behind by a departed app-server has nothing to take over, so
  // offering it as a live process would open a terminal onto nothing.
  it("treats only controllable statuses as live", () => {
    for (const status of ["starting", "running", "stopping"] as const) {
      expect(isManagedProcessLive(process({ status }))).toBe(true);
    }
    for (const status of ["stopped", "failed"] as const) {
      expect(isManagedProcessLive(process({ status }))).toBe(false);
    }
  });

  // Polling and push events race. The older snapshot must not win, and a stale
  // live snapshot must not resurrect a process that already settled.
  it("keeps the newer snapshot and never revives a settled process", () => {
    const older = process({ updated_at: "2026-08-05T08:00:00Z" });
    const newer = process({ updated_at: "2026-08-05T08:00:05Z", command: "npm test" });
    // Whichever order they arrive in, the newer snapshot is the one kept.
    expect(preferManagedProcess(older, newer).command).toBe("npm test");
    expect(preferManagedProcess(newer, older).command).toBe("npm test");

    const settled = process({ status: "stopped", updated_at: "2026-08-05T08:00:10Z" });
    expect(preferManagedProcess(settled, process({ status: "running" })).status).toBe("stopped");
  });

  it("lists live processes newest first and skips settled ones", () => {
    const list = liveManagedProcessList({
      old: process({ id: "old", updated_at: "2026-08-05T08:00:00Z" }),
      fresh: process({ id: "fresh", updated_at: "2026-08-05T08:00:09Z" }),
      gone: process({ id: "gone", status: "stopped", updated_at: "2026-08-05T08:00:20Z" }),
    });
    expect(list.map((entry) => entry.id)).toEqual(["fresh", "old"]);
  });

  // The terminal workspace already addresses a live process by this id, so the
  // environment panel must not invent a second addressing scheme.
  it("addresses a process the way the terminal workspace does", () => {
    expect(managedProcessResourceID("proc-9")).toBe("managed:proc-9");
  });
});
