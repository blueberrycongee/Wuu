import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import * as React from "react";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, expect, it, vi } from "vitest";
import { PluginHost, type PluginGenerationApi } from "./PluginHost";

const source = readFileSync(resolve(process.cwd(), "../internal/plugin/bundled/goal/desktop.js"), "utf8");
const activate = Function(source.replace("export async function activate(api)", "return async function activate(api)"))() as (api: PluginGenerationApi) => Promise<void>;

afterEach(() => vi.useRealTimers());

it("renders live goal controls, scopes actions to the session and cleans up on disable", async () => {
  vi.useFakeTimers();
  let goal = { id: "goal-one", objective: "Finish the task", status: "active", tokens_used: 12, token_budget: 100, time_used_seconds: 2 };
  const invoke = vi.fn(async ({ method, input }: { method: string; input?: unknown }) => {
    expect(input).toMatchObject({ thread_id: "thread-one" });
    if (method === "pause") goal = { ...goal, status: "paused" };
    if (method === "resume") goal = { ...goal, status: "active" };
    return { goal };
  });
  const host = new PluginHost({ react: React, invokeRuntime: invoke });
  await host.activateGeneration({ pluginId: "goal", generation: "one", register: activate });
  const container = document.createElement("div"); document.body.append(container);
  const root = createRoot(container);
  const Content = host.getInspectorSections()[0].render as React.ComponentType<{ snapshot: unknown }>;
  try {
    await act(async () => root.render(<Content snapshot={{ contractVersion: 1, session: { id: "thread-one", status: "idle" } }} />));
    expect(container.textContent).toContain("Finish the task");
    const click = async (text: string) => {
      const button = [...container.querySelectorAll("button")].find((node) => node.textContent === text);
      expect(button).toBeDefined();
      await act(async () => button!.click());
    };
    await click("暂停");
    expect(invoke).toHaveBeenCalledWith(expect.objectContaining({ method: "pause", input: { thread_id: "thread-one" } }));
    expect(container.querySelector("input")).not.toBeNull();
    await click("继续");
    expect(invoke).toHaveBeenCalledWith(expect.objectContaining({ method: "resume", input: { thread_id: "thread-one" } }));
    const status = host.getComposerStatusSources()[0].getSnapshot({ threadId: "thread-one", mainConversation: true });
    expect(status[0].state).toBe("running");
    act(() => root.unmount());
    host.disable("goal");
    const calls = invoke.mock.calls.length;
    await vi.advanceTimersByTimeAsync(5000);
    expect(invoke).toHaveBeenCalledTimes(calls);
    expect(host.getComposerStatusSources()).toEqual([]);
    expect(host.getInspectorSections()).toEqual([]);
  } finally {
    act(() => root.unmount()); host.disable("goal"); container.remove();
  }
});

it("discards a stale response and keeps runtime errors visible", async () => {
  vi.useFakeTimers();
  const pending: Array<{ resolve: (value: unknown) => void; reject: (error: Error) => void }> = [];
  const host = new PluginHost({ react: React, invokeRuntime: () => new Promise((resolve, reject) => pending.push({ resolve, reject })) });
  await host.activateGeneration({ pluginId: "goal", generation: "one", register: activate });
  const source = host.getComposerStatusSources()[0];
  const context = { threadId: "thread-one", mainConversation: true as const };
  const dispose = source.subscribe(context, () => {});
  try {
    await vi.advanceTimersByTimeAsync(1500);
    pending[1].resolve({ goal: { id: "new", objective: "Current goal", status: "paused", tokens_used: 10 } });
    await Promise.resolve(); await Promise.resolve();
    pending[0].resolve({ goal: { id: "old", objective: "Stale goal", status: "active", tokens_used: 0 } });
    await Promise.resolve(); await Promise.resolve();
    expect(source.getSnapshot(context)[0].id).toBe("new");
    const container = document.createElement("div"); const root = createRoot(container);
    const Content = host.getInspectorSections()[0].render as React.ComponentType<{ snapshot: unknown }>;
    await act(async () => root.render(<Content snapshot={{ session: { id: "thread-one" } }} />));
    await act(async () => pending.at(-1)!.reject(new Error("storage unavailable")));
    expect(container.textContent).toContain("storage unavailable");
    act(() => root.unmount());
  } finally { dispose(); host.disable("goal"); }
});
