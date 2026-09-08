import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import { networkInterfaces } from "node:os";
import { access } from "node:fs/promises";
import { join } from "node:path";
import { resolveWuuCommand } from "./wuuCommand";
import { RemoteHostManager } from "./remoteControl";

export function phoneAddress(interfaces = networkInterfaces()): string {
  const entries = Object.entries(interfaces).sort(([a], [b]) => Number(b === "en0") - Number(a === "en0"));
  for (const [, addresses] of entries) {
    for (const item of addresses ?? []) {
      if (item.family === "IPv4" && !item.internal && /^(10\.|192\.168\.|172\.(1[6-9]|2\d|3[01])\.)/.test(item.address)) return item.address;
    }
  }
  throw new Error("未找到局域网地址，请先将电脑连接到 Wi-Fi。");
}

export function phonePairLink(base: string | null, uri: string | null): string | null {
  if (!base || !uri) return null;
  const url = new URL(base);
  url.hash = new URLSearchParams({ pair: uri }).toString();
  return url.href;
}

/** Owns the LAN listener alongside the existing encrypted remote host. */
export class PhoneAccess {
  private relay: ChildProcessWithoutNullStreams | null = null;
  private exited: Promise<void> | null = null;
  private base: string | null = null;
  private queue: Promise<unknown> = Promise.resolve();
  constructor(private readonly host: RemoteHostManager, private readonly webRoot: string, private readonly changed: () => void) {}
  url(): string | null { return this.base; }
  run<T>(action: () => Promise<T>): Promise<T> {
    const next = this.queue.then(action);
    this.queue = next.catch(() => {});
    return next;
  }
  async start(workdir: string, pair = true): Promise<void> {
    if (this.relay && this.host.isRunning() && !pair) return;
    if (!this.relay) {
      await access(join(this.webRoot, "index.html"));
      const address = phoneAddress();
      const command = resolveWuuCommand(process.env, workdir, process.env.WUU_SOURCE_ROOT, process.resourcesPath);
      const relay = spawn(command.command, [...command.args, "relay", "--addr", `${address}:8787`, "--web-root", this.webRoot], { cwd: command.cwd, env: process.env });
      this.relay = relay;
      this.exited = new Promise(resolve => relay.once("close", () => resolve()));
      relay.once("close", () => {
        if (this.relay !== relay) return;
        this.relay = null; this.base = null;
        void this.host.stopHost().then(this.changed, this.changed);
      });
      try {
        await new Promise<void>((resolve, reject) => {
          const timer = setTimeout(() => finish(new Error("手机访问服务启动超时")), 15_000);
          let output = "", errors = "";
          const finish = (error?: Error) => {
            clearTimeout(timer);
            relay.stdout.off("data", onData); relay.stderr.off("data", onErrorData);
            relay.off("error", onError); relay.off("exit", onExit);
            error ? reject(error) : resolve();
          };
          const onData = (chunk: Buffer) => { output += chunk.toString(); if (output.includes("connect url:")) finish(); };
          const onErrorData = (chunk: Buffer) => { errors = (errors + chunk.toString()).slice(-2000); };
          const onError = (error: Error) => finish(error);
          const onExit = () => finish(new Error(errors || "手机访问服务未能启动"));
          relay.stdout.on("data", onData); relay.stderr.on("data", onErrorData);
          relay.once("error", onError); relay.once("exit", onExit);
        });
        // Drain ongoing diagnostics without retaining pairing data or logs.
        relay.stdout.resume(); relay.stderr.resume();
        this.base = `http://${address}:8787/`;
      } catch (error) { await this.stop(); throw error; }
    }
    await this.host.stopHost();
    this.host.startHost(workdir, { pair, relay: this.base!.replace("http:", "ws:") + "v1/connect" });
  }
  async stop(): Promise<void> {
    const hostStopped = this.host.stopHost();
    const relay = this.relay, exited = this.exited;
    this.relay = null; this.base = null;
    if (relay) {
      const timer = setTimeout(() => relay.kill("SIGKILL"), 3000);
      try { relay.kill("SIGTERM"); await exited; } finally { clearTimeout(timer); }
    }
    await hostStopped;
    this.changed();
  }
}
