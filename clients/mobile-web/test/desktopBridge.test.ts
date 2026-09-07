import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Credentials, RemoteClientOptions } from "@wuu/remote-core";

const remote = vi.hoisted(() => ({ call: vi.fn(), options: {} as RemoteClientOptions, attached: true }));
vi.mock("@wuu/remote-core", () => ({
  RemoteClient: class {
    call = remote.call;
    constructor(_credentials: Credentials, options: RemoteClientOptions) { remote.options = options; }
    isAttached = () => remote.attached;
    start = () => remote.options.onAttach?.({ session: "first", resumed: false });
    waitAttached = async () => {};
    latestState = () => null;
    stop = async () => { remote.attached = false; remote.options.onDetach?.(); };
  },
  pair: vi.fn(),
}));

import { RemoteDesktopBridge, UnavailableHostOperationError } from "../src/lib/desktopBridge";

beforeEach(() => {
  remote.attached = true;
  remote.call.mockReset().mockResolvedValue({ current: "/paired/workspace" });
  const values = new Map<string, string>();
  vi.stubGlobal("localStorage", {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => values.set(key, value),
  });
  vi.stubGlobal("navigator", { userAgent: "iPhone Mac OS", language: "en-US" });
  vi.stubGlobal("window", { open: vi.fn() });
});

function api() {
  return new RemoteDesktopBridge({ host_pub: "paired-host" } as Credentials).api;
}

describe("browser host contract", () => {
  it("leaves optional Electron integrations absent and rejects unsupported actions", async () => {
    const host = api();
    expect(host.reportBrowserBounds).toBeUndefined();
    expect(host.openSideThread).toBeUndefined();
    expect(host.onBrowserInvalidate).toBeUndefined();
    expect(host.unsupportedMethods).toContain("startTerminalSession");
    await expect(host.startTerminalSession()).rejects.toBeInstanceOf(UnavailableHostOperationError);
    await expect(host.gitStatus()).rejects.toMatchObject({ code: "host_operation_unavailable" });
    await expect(host.updateVoiceInputSettings({ polish_enabled: true, language: "en-US" }))
      .rejects.toBeInstanceOf(UnavailableHostOperationError);
    await expect(host.selectProject("another-computer")).rejects.toThrow("Unknown remote workspace");
    expect(remote.call).not.toHaveBeenCalled();
  });

  it("preserves explicit resets and thread-scoped model selection over RPC", async () => {
    await api().updateRuntimeSettings(undefined, undefined, "", undefined, "", "read_only", "thread-1");
    expect(remote.call).toHaveBeenCalledWith("config/model/update", {
      thread_id: "thread-1", effort: "", variant: "", permission_mode: "read_only",
    }, 30_000);
  });

  it("returns host engine inventory and routes process input to its owning thread", async () => {
    const host = api();
    const inventory = { engines: [{ id: "codex", installed: true }] };
    remote.call.mockResolvedValueOnce(inventory);
    expect(await host.listEngines()).toBe(inventory);
    await host.writeManagedProcess("thread-1", "process-2", "\u0003");
    expect(remote.call).toHaveBeenLastCalledWith("process/write", {
      thread_id: "thread-1", process_id: "process-2", input: "\u0003",
    }, 30_000);
  });

  it("forwards question holds and preserves mixed message parts", async () => {
    const host = api();
    await host.holdUserQuestion("question-1");
    expect(remote.call).toHaveBeenLastCalledWith("user-question/hold", { request_id: "question-1" }, 30_000);
    const parts = [{ type: "text" as const, text: "Review this file" }];
    await host.startTurn("thread-1", "Review this file", [], [], "read_only", { path: "src/main.go" }, parts);
    expect(remote.call).toHaveBeenLastCalledWith("turn/start", expect.objectContaining({
      thread_id: "thread-1", active_document: { path: "src/main.go" }, content_parts: parts,
    }), 30_000);
  });

  it("reads and resolves files in the selected conversation worktree on the host", async () => {
    const bridge = await connectBridge();
    await bridge.api.listWorkspaceDirectory("src", "/paired/worktree");
    expect(remote.call).toHaveBeenLastCalledWith("workspace/directory/list", { path: "src", root: "/paired/worktree" }, 30_000);
    await bridge.api.readWorkspaceFile("src/main.go", "/paired/worktree");
    expect(remote.call).toHaveBeenLastCalledWith("workspace/file/read", { path: "src/main.go", root: "/paired/worktree" }, 30_000);
    await bridge.api.resolveWorkspaceFileReference("main.go:12");
    expect(remote.call).toHaveBeenLastCalledWith("workspace/file/resolve", { reference: "main.go:12", root: "/paired/workspace" }, 30_000);
  });

  it("does not open executable URL schemes", async () => {
    const host = api();
    await expect(host.openExternal("javascript:alert(1)")).rejects.toThrow("HTTP");
    expect(window.open).not.toHaveBeenCalled();
    await host.openExternal("https://example.com/docs");
    expect(window.open).toHaveBeenCalledWith("https://example.com/docs", "_blank", "noopener,noreferrer");
  });
});


async function connectBridge() {
  const bridge = new RemoteDesktopBridge({ host_pub: "paired-host" } as Credentials);
  await bridge.connect();
  return bridge;
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => { resolve = done; });
  return { promise, resolve };
}

describe("connection recovery", () => {
  it("waits for a real host workspace before presenting the workbench", async () => {
    const bridge = await connectBridge();
    expect(bridge.getConnectionSnapshot().phase).toBe("connected");
    expect((await bridge.api.listProjects()).active_context?.cwd).toBe("/paired/workspace");
  });

  it("restores fresh and resumed connections in place and waits for subscribers", async () => {
    const bridge = await connectBridge();
    for (const resumed of [false, true]) {
      const restore = deferred<void>();
      const handler = vi.fn(() => restore.promise);
      const off = bridge.api.onRuntimeRestore!(handler);
      remote.options.onDetach?.();
      expect(bridge.getConnectionSnapshot().phase).toBe("reconnecting");
      remote.options.onAttach?.({ session: "next", resumed });
      await vi.waitFor(() => expect(handler).toHaveBeenCalledOnce());
      expect(bridge.getConnectionSnapshot().phase).toBe("restoring");
      restore.resolve();
      await vi.waitFor(() => expect(bridge.getConnectionSnapshot().phase).toBe("connected"));
      off();
    }
  });

  it("ignores stale restoration completion after another disconnect", async () => {
    const bridge = await connectBridge();
    const restore = deferred<void>();
    bridge.api.onRuntimeRestore!(() => restore.promise);
    remote.options.onAttach?.({ session: "next", resumed: false });
    await Promise.resolve();
    remote.options.onDetach?.();
    restore.resolve();
    await Promise.resolve();
    await Promise.resolve();
    expect(bridge.getConnectionSnapshot().phase).toBe("reconnecting");
  });

  it("surfaces restore failure and retries without pairing or replacing the bridge", async () => {
    const bridge = await connectBridge();
    const restore = vi.fn().mockRejectedValueOnce(new Error("snapshot failed")).mockResolvedValue(undefined);
    bridge.api.onRuntimeRestore!(restore);
    remote.options.onAttach?.({ session: "next", resumed: false });
    await vi.waitFor(() => expect(bridge.getConnectionSnapshot().phase).toBe("error"));
    expect(bridge.getConnectionSnapshot().error).toBe("snapshot failed");
    await bridge.retryRestore();
    expect(bridge.getConnectionSnapshot().phase).toBe("connected");
    expect(restore).toHaveBeenCalledTimes(2);
  });

  it("never queues offline sends and rejects responses from replaced connections", async () => {
    const bridge = await connectBridge();
    remote.call.mockClear();
    remote.attached = false;
    remote.options.onDetach?.();
    await expect(bridge.api.startTurn("thread", "do work")).rejects.toThrow("disconnected");
    expect(remote.call).not.toHaveBeenCalled();
    remote.attached = true;
    const pending = deferred<unknown>();
    remote.call.mockReturnValueOnce(pending.promise);
    const request = bridge.api.listThreads();
    remote.options.onDetach?.();
    pending.resolve({ threads: [] });
    await expect(request).rejects.toThrow("connection changed");
  });

  it("blocks writes until restoration finishes", async () => {
    const bridge = await connectBridge();
    const restore = deferred<void>();
    bridge.api.onRuntimeRestore!(() => restore.promise);
    remote.options.onAttach?.({ session: "next", resumed: false });
    await expect(bridge.api.startTurn("thread", "do work")).rejects.toThrow("restoring");
    restore.resolve();
  });
});


describe("workspace routing", () => {
  const workspaces = {
    current: "/computer/alpha", current_id: "alpha",
    workspaces: [{ id: "alpha", name: "Alpha", path: "/computer/alpha" },
      { id: "beta", name: "Beta", path: "/computer/beta" }],
  };

  it("selects registered workspaces and starts work with the computer's workspace ID", async () => {
    remote.call.mockResolvedValue(workspaces);
    const bridge = await connectBridge();
    expect((await bridge.api.listProjects()).projects.map((project) => project.id)).toEqual(["alpha", "beta"]);
    const selected = await bridge.api.selectProject("beta");
    expect(selected.active_context).toEqual({ kind: "project", project_id: "beta", cwd: "/computer/beta" });
    await bridge.api.listThreads();
    expect(remote.call).toHaveBeenLastCalledWith("thread/list", { cwd: "/computer/beta" }, 30_000);
    await bridge.api.startThread({ model: "chosen-model" });
    expect(remote.call).toHaveBeenLastCalledWith("thread/start", {
      model: "chosen-model", cwd: "/computer/beta", workspace_id: "beta",
    }, 30_000);
    await bridge.api.listWorkspaceDirectory();
    expect(remote.call).toHaveBeenLastCalledWith("workspace/directory/list", { path: undefined, root: "/computer/beta" }, 30_000);
  });

  it("routes background and worktree events to their owning project after switching", async () => {
    remote.call.mockResolvedValue(workspaces);
    const bridge = await connectBridge();
    const events: Array<{ workdir: string }> = [];
    bridge.api.onServerEvent((event) => events.push(event));
    remote.options.onNotification?.("thread/started", { thread: { id: "a", cwd: "/computer/alpha-tree", workspace_id: "alpha", worktree: { base_repo: "/computer/alpha" } } });
    await bridge.api.selectProject("beta");
    remote.options.onNotification?.("item/agentMessage/delta", { thread_id: "a", delta: "still running" });
    expect(events.at(-1)?.workdir).toBe("/computer/alpha");
    remote.options.onNotification?.("thread/started", { thread: { id: "b", cwd: "/computer/beta", workspace_id: "beta" } });
    expect(events.at(-1)?.workdir).toBe("/computer/beta");
    remote.options.onState?.({ ver: 1, host: { workdir: "/computer/alpha" }, running: [{ thread_id: "a" }, { thread_id: "b" }] });
    expect(await bridge.api.getRunningThreadsSnapshot()).toEqual([
      { thread_id: "a", workdir: "/computer/alpha" }, { thread_id: "b", workdir: "/computer/beta" },
    ]);
  });

  it("reopens a known unregistered conversation without creating a native scratch workspace", async () => {
    remote.call.mockResolvedValue(workspaces);
    const bridge = await connectBridge();
    remote.call.mockResolvedValueOnce({ threads: [{ id: "scratch", cwd: "/computer/scratch" }] });
    await bridge.api.listAllThreads();
    expect((await bridge.api.selectNoProject(false, "/computer/scratch")).active_context)
      .toEqual({ kind: "no_project", cwd: "/computer/scratch" });
    await expect(bridge.api.selectNoProject(true)).rejects.toBeInstanceOf(UnavailableHostOperationError);
    await expect(bridge.api.selectNoProject(false, "/unknown")).rejects.toBeInstanceOf(UnavailableHostOperationError);
  });

  it("preserves selection on reconnect and never substitutes a different directory after removal", async () => {
    remote.call.mockResolvedValue(workspaces);
    const bridge = await connectBridge();
    await bridge.api.selectProject("beta");
    remote.options.onAttach?.({ session: "next", resumed: false });
    await vi.waitFor(() => expect(bridge.getConnectionSnapshot().phase).toBe("connected"));
    expect((await bridge.api.listProjects()).active_context?.cwd).toBe("/computer/beta");
    remote.call.mockResolvedValue({ ...workspaces, workspaces: [workspaces.workspaces[0]] });
    const refreshed = await bridge.api.listProjects();
    expect(refreshed.active_context?.cwd).toBe("/computer/beta");
    expect(refreshed.projects.find((project) => project.id === "beta")?.missing).toBe(true);
  });
});
