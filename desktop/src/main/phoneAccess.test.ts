import { EventEmitter } from "node:events";
import { PassThrough } from "node:stream";
import { mkdtemp, writeFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, describe, expect, it, vi } from "vitest";
import { PhoneAccess, phoneAddress, phonePairLink } from "./phoneAccess";
import type { RemoteHostManager } from "./remoteControl";
vi.mock("node:os", async importOriginal => ({ ...(await importOriginal<typeof import("node:os")>()), networkInterfaces: () => ({ en0: [{ address: "192.168.1.8", family: "IPv4", internal: false }] }) }));
const mocks = vi.hoisted(() => ({ spawn: vi.fn() }));
vi.mock("node:child_process", () => ({ spawn: mocks.spawn }));
vi.mock("./wuuCommand", () => ({ resolveWuuCommand: () => ({ command: "wuu", args: [], cwd: "/tmp" }) }));
const roots: string[] = [];
afterEach(async () => { vi.clearAllMocks(); for (const root of roots.splice(0)) await rm(root, { recursive: true, force: true }); });

it("uses a LAN address and keeps the pairing secret out of HTTP queries", () => {
  const entry = (address: string, internal = false) => ({ address, family: "IPv4", internal, netmask: "", cidr: null, mac: "" } as const);
  expect(phoneAddress({ lo0: [entry("127.0.0.1", true)], en0: [entry("192.168.1.8")] })).toBe("192.168.1.8");
  expect(() => phoneAddress({ lo0: [entry("127.0.0.1", true)] })).toThrow();
  const link = new URL(phonePairLink("http://192.168.1.8:8787/", "wuu://pair?secret=abc")!);
  expect(link.search).toBe("");
  expect(new URLSearchParams(link.hash.slice(1)).get("pair")).toBe("wuu://pair?secret=abc");
});

describe("phone access lifecycle", () => {
  async function fixture(fail = false) {
    const root = await mkdtemp(join(tmpdir(), "phone-web-")); roots.push(root);
    await writeFile(join(root, "index.html"), "Web");
    const child = Object.assign(new EventEmitter(), { stdout: new PassThrough(), stderr: new PassThrough(), kill: vi.fn(() => { queueMicrotask(() => child.emit("close", 0)); return true; }) });
    mocks.spawn.mockImplementation(() => {
      queueMicrotask(() => {
        if (fail) { child.stderr.write("address already in use"); child.emit("exit", 1); child.emit("close", 1); }
        else child.stdout.write("connect url: ws://192.168.1.8:8787/v1/connect\n");
      });
      return child;
    });
    const host = { stopHost: vi.fn(async () => {}), startHost: vi.fn(), isRunning: () => true };
    return { service: new PhoneAccess(host as unknown as RemoteHostManager, root, vi.fn()), host, child };
  }
  it("starts Web before pairing and closes the listener with access", async () => {
    const { service, host, child } = await fixture();
    try {
      await service.start("/tmp");
      expect(host.startHost).toHaveBeenCalledWith("/tmp", { pair: true, relay: expect.stringMatching(/^ws:\/\/.*:8787\/v1\/connect$/) });
      expect(service.url()).toMatch(/^http:/);
      await service.stop();
      expect(child.kill).toHaveBeenCalledWith("SIGTERM");
      expect(service.url()).toBeNull();
    } finally { await service.stop(); }
  });
  it("does not open pairing when the listener fails", async () => {
    const { service, host } = await fixture(true);
    await expect(service.start("/tmp")).rejects.toThrow("address already in use");
    expect(host.startHost).not.toHaveBeenCalled();
    expect(service.url()).toBeNull();
  });
});
