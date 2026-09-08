// @vitest-environment jsdom
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const remote = vi.hoisted(() => ({ pair: vi.fn(), connect: vi.fn(), disconnect: vi.fn(), install: vi.fn() }));
const credentials = vi.hoisted(() => ({ load: vi.fn(), save: vi.fn(), clear: vi.fn() }));
vi.mock("../src/lib/credStore", () => ({ webCredStore: credentials }));
vi.mock("../src/lib/desktopBridge", () => ({
  RemoteDesktopBridge: class {
    static pair = remote.pair;
    connect = remote.connect;
    disconnect = remote.disconnect;
    install = remote.install;
    subscribeConnection = () => () => {};
    getConnectionSnapshot = () => connected;
  },
}));
vi.mock("../src/WebWorkspace", () => ({ default: () => createElement("div", null, "Connected workbench") }));
const connected = { phase: "connected", revision: 1 };
import App from "../src/App";
let container: HTMLDivElement;
let root: Root;

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}
async function click(text: string) {
  const button = [...container.querySelectorAll("button")].find((item) => item.textContent === text);
  expect(button, `button ${text}`).toBeTruthy();
  await act(async () => button!.click());
}
async function startPair() {
  const input = container.querySelector("textarea")!;
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(input, "wuu://pair?test");
    input.dispatchEvent(new Event("input", { bubbles: true }));
  });
  await click("配对并进入");
}
beforeEach(async () => {
  vi.resetAllMocks();
  Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });
  credentials.load.mockResolvedValue(null);
  credentials.clear.mockResolvedValue(undefined);
  credentials.save.mockResolvedValue(undefined);
  remote.disconnect.mockResolvedValue(undefined);
  remote.connect.mockResolvedValue(undefined);
  container = document.createElement("div"); document.body.append(container);
  root = createRoot(container);
  await act(async () => root.render(createElement(App)));
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });

describe("Web connection ownership", () => {
  it("does not save or connect a pairing response after the user cancels it", async () => {
    const pending = deferred<unknown>(); remote.pair.mockReturnValue(pending.promise);
    await startPair(); await click("清除旧配对");
    await act(async () => pending.resolve({ host_pub: "cancelled-computer" }));
    expect(credentials.save).not.toHaveBeenCalled();
    expect(remote.connect).not.toHaveBeenCalled();
    expect(container.textContent).toContain("配对并进入");
  });

  it("ignores a late failure from a cancelled pairing while another pairing runs", async () => {
    const old = deferred<unknown>(), next = deferred<unknown>();
    remote.pair.mockReturnValueOnce(old.promise).mockReturnValueOnce(next.promise);
    await startPair(); await click("清除旧配对"); await startPair();
    await act(async () => old.reject(new Error("old pairing failed")));
    expect(container.textContent).toContain("正在连接电脑");
    expect(container.textContent).not.toContain("old pairing failed");
  });

  it("returns to pairing immediately while an old connection is still closing", async () => {
    const closing = deferred<void>(), connecting = deferred<void>();
    remote.pair.mockResolvedValue({ host_pub: "computer" });
    remote.connect.mockReturnValue(connecting.promise);
    remote.disconnect.mockReturnValue(closing.promise);
    await startPair(); await click("清除旧配对");
    expect(container.textContent).toContain("配对并进入");
    await act(async () => connecting.resolve());
    expect(remote.install).not.toHaveBeenCalled();
    await act(async () => closing.resolve());
  });

  it("disconnects and invalidates an in-flight connection when the shell unmounts", async () => {
    const connecting = deferred<void>();
    remote.pair.mockResolvedValue({ host_pub: "computer" });
    remote.connect.mockReturnValue(connecting.promise);
    await startPair();
    await act(async () => root.unmount());
    expect(remote.disconnect).toHaveBeenCalled();
    await act(async () => connecting.resolve());
    expect(remote.install).not.toHaveBeenCalled();
  });
});
