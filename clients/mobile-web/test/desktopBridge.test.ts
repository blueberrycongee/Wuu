import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Credentials } from "@wuu/remote-core";

const remote = vi.hoisted(() => ({ call: vi.fn() }));
vi.mock("@wuu/remote-core", () => ({
  RemoteClient: class {
    call = remote.call;
  },
  pair: vi.fn(),
}));

import { RemoteDesktopBridge, UnavailableHostOperationError } from "../src/lib/desktopBridge";

beforeEach(() => {
  remote.call.mockReset().mockResolvedValue({});
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

  it("does not open executable URL schemes", async () => {
    const host = api();
    await expect(host.openExternal("javascript:alert(1)")).rejects.toThrow("HTTP");
    expect(window.open).not.toHaveBeenCalled();
    await host.openExternal("https://example.com/docs");
    expect(window.open).toHaveBeenCalledWith("https://example.com/docs", "_blank", "noopener,noreferrer");
  });
});
