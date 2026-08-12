import { randomUUID } from "node:crypto";
import { spawn, type ChildProcess } from "node:child_process";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import {
  app,
  BrowserWindow,
  type Event,
  type IpcMainEvent,
  type IpcMainInvokeEvent,
  ipcMain,
  shell,
  type WebContents,
} from "electron";
import type { JsonValue, ProjectionFrame } from "@wuu-v2/contracts";
import { buildDefaultClientBootManifest } from "@wuu-v2/profile-default/client";
import {
  bridgeChannels,
  isJsonValue,
  isSessionId,
  isSubscriptionId,
  type DesktopBootResult,
  type HarnessInboundMessage,
  type HarnessOutboundMessage,
  type HarnessState,
} from "../shared/bridge.js";

const directory = fileURLToPath(new URL(".", import.meta.url));

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

class HarnessController {
  private child: ChildProcess | undefined;
  private ready: Promise<string> = new Promise(() => {});
  private transition: Promise<void> = Promise.resolve();
  private resolveReady: ((sessionId: string) => void) | undefined;
  private rejectReady: ((error: Error) => void) | undefined;
  private watchdog: ReturnType<typeof setTimeout> | undefined;
  private heartbeat: ReturnType<typeof setInterval> | undefined;
  private heartbeatDeadline: ReturnType<typeof setTimeout> | undefined;
  private heartbeatId: string | undefined;
  private failure: Error | undefined;
  private failedChild: ChildProcess | undefined;
  private sessionId: string | undefined;
  private readonly pending = new Map<string, {
    resolve(value: JsonValue | undefined): void;
    reject(error: Error): void;
  }>();

  constructor(
    private readonly dataDirectory: string,
    private readonly workspace: string,
    private readonly subscriptions: () => Iterable<readonly [string, { sessionId: string }]>,
    private readonly onProjection: (subscriptionId: string, frame: ProjectionFrame) => void,
    private readonly onState: (state: HarnessState) => void,
  ) {}

  start(): Promise<string> {
    this.failure = undefined;
    this.onState({ state: "starting" });
    const ready = this.transition.then(async () => {
      await this.stopChild(new Error("Harness restarting"), false);
      return this.launch();
    });
    this.ready = ready;
    this.transition = ready.then(() => {}, () => {});
    return ready;
  }

  private launch(): Promise<string> {
    this.failure = undefined;
    this.failedChild = undefined;
    const ready = new Promise<string>((resolve, reject) => {
      this.resolveReady = resolve;
      this.rejectReady = reject;
    });
    const worker = join(directory, "worker.js");
    const child = spawn(process.execPath, [worker], {
      env: {
        ...process.env,
        ELECTRON_RUN_AS_NODE: "1",
        WUU_V2_DATA_DIR: this.dataDirectory,
        WUU_V2_WORKSPACE: this.workspace,
        ...(this.sessionId ? { WUU_V2_SESSION_ID: this.sessionId } : {}),
      },
      stdio: ["ignore", "pipe", "pipe", "ipc"],
    });
    this.child = child;
    child.stdout?.on("data", (chunk) => console.log(`[harness] ${String(chunk).trimEnd()}`));
    child.stderr?.on("data", (chunk) => console.error(`[harness] ${String(chunk).trimEnd()}`));
    child.on("message", (message: HarnessOutboundMessage) => this.receive(child, message));
    child.once("error", (error) => this.failed(child, error));
    child.once("exit", (code, signal) => {
      if (this.child !== child) return;
      this.failed(child, new Error(`Harness exited (${signal ?? code ?? "unknown"})`), true);
    });
    this.watchdog = setTimeout(() => {
      if (this.child !== child || !this.rejectReady) return;
      this.failed(child, new Error("Harness did not become ready within 15 seconds"));
      if (!child.killed) child.kill("SIGKILL");
    }, 15_000);
    return ready;
  }

  private receive(child: ChildProcess, message: HarnessOutboundMessage): void {
    if (this.child !== child) return;
    if (this.failedChild === child) return;
    if (message.type === "pong" && message.id !== this.heartbeatId) return;
    this.ackHeartbeat();
    if (message.type === "ready") {
      if (this.watchdog) clearTimeout(this.watchdog);
      this.watchdog = undefined;
      this.sessionId = message.sessionId;
      this.resolveReady?.(message.sessionId);
      this.resolveReady = undefined;
      this.rejectReady = undefined;
      this.startHeartbeat(child);
      this.onState({ state: "ready" });
      for (const [subscriptionId, subscription] of this.subscriptions()) {
        this.send({ type: "follow", subscriptionId, sessionId: subscription.sessionId });
      }
      return;
    }
    if (message.type === "fatal") {
      this.failed(child, new Error(message.error));
      return;
    }
    if (message.type === "projection") {
      this.onProjection(message.subscriptionId, message.frame);
      return;
    }
    if (message.type === "pong") {
      return;
    }
    const pending = this.pending.get(message.id);
    if (!pending) return;
    this.pending.delete(message.id);
    if (message.error) pending.reject(new Error(message.error));
    else pending.resolve(message.value);
  }

  private failed(child: ChildProcess, error: Error, exited = false): void {
    if (this.child !== child) return;
    const alreadyFailed = this.failedChild === child;
    if (exited) {
      this.child = undefined;
      this.failedChild = undefined;
    } else {
      this.failedChild = child;
    }
    if (alreadyFailed) return;
    this.failure = error;
    if (this.watchdog) clearTimeout(this.watchdog);
    this.watchdog = undefined;
    this.stopHeartbeat();
    this.rejectReady?.(error);
    this.resolveReady = undefined;
    this.rejectReady = undefined;
    for (const request of this.pending.values()) request.reject(error);
    this.pending.clear();
    this.onState({ state: "failed", error: error.message });
  }

  private send(message: HarnessInboundMessage): void {
    if (!this.child?.connected) throw new Error("Harness is not connected");
    this.child.send(message);
  }

  private startHeartbeat(child: ChildProcess): void {
    this.stopHeartbeat();
    this.heartbeat = setInterval(() => {
      if (this.child !== child || this.heartbeatId) return;
      const id = randomUUID();
      this.heartbeatId = id;
      try {
        this.send({ type: "ping", id });
      } catch (error) {
        this.failed(child, error instanceof Error ? error : new Error(String(error)));
        if (!child.killed) child.kill("SIGKILL");
        return;
      }
      this.heartbeatDeadline = setTimeout(() => {
        if (this.child !== child || this.heartbeatId !== id) return;
        this.failed(child, new Error("Harness stopped responding"));
        if (!child.killed) child.kill("SIGKILL");
      }, 10_000);
    }, 5_000);
  }

  private stopHeartbeat(): void {
    if (this.heartbeat) clearInterval(this.heartbeat);
    if (this.heartbeatDeadline) clearTimeout(this.heartbeatDeadline);
    this.heartbeat = undefined;
    this.heartbeatDeadline = undefined;
    this.heartbeatId = undefined;
  }

  private ackHeartbeat(): void {
    if (this.heartbeatDeadline) clearTimeout(this.heartbeatDeadline);
    this.heartbeatDeadline = undefined;
    this.heartbeatId = undefined;
  }

  whenReady(): Promise<string> {
    if (this.failure) return Promise.reject(this.failure);
    return this.ready;
  }

  async action(action: string, input: JsonValue): Promise<JsonValue | undefined> {
    await this.whenReady();
    const id = randomUUID();
    const result = new Promise<JsonValue | undefined>((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
    });
    try {
      this.send({ type: "action", id, action, input });
    } catch (error) {
      this.pending.delete(id);
      throw error;
    }
    return result;
  }

  async follow(subscriptionId: string, sessionId: string): Promise<void> {
    await this.whenReady();
    this.send({ type: "follow", subscriptionId, sessionId });
  }

  unfollow(subscriptionId: string): void {
    if (this.child?.connected) this.send({ type: "unfollow", subscriptionId });
  }

  private async stopChild(error: Error, exposeFailure = true): Promise<void> {
    const child = this.child;
    this.child = undefined;
    this.failedChild = undefined;
    if (this.watchdog) clearTimeout(this.watchdog);
    this.watchdog = undefined;
    this.stopHeartbeat();
    if (exposeFailure) this.failure = error;
    this.rejectReady?.(error);
    this.resolveReady = undefined;
    this.rejectReady = undefined;
    for (const request of this.pending.values()) request.reject(error);
    this.pending.clear();
    if (!child || child.exitCode !== null || child.signalCode !== null) return;
    const exited = new Promise<void>((resolve) => {
      let settled = false;
      const done = () => {
        if (settled) return;
        settled = true;
        resolve();
      };
      child.once("exit", done);
      child.once("close", done);
    });
    if (child.connected) child.disconnect();
    if (!child.killed) child.kill("SIGTERM");
    let timer: ReturnType<typeof setTimeout> | undefined;
    const graceful = await Promise.race([
      exited.then(() => true),
      new Promise<false>((resolve) => {
        timer = setTimeout(() => resolve(false), 3_000);
      }),
    ]);
    if (timer) clearTimeout(timer);
    if (graceful) return;
    if (child.exitCode === null && child.signalCode === null) child.kill("SIGKILL");
    await exited;
  }

  stop(): Promise<void> {
    const stopped = this.transition.then(() => this.stopChild(new Error("Harness stopped")));
    this.transition = stopped.then(() => {}, () => {});
    return stopped;
  }
}

const subscriptions = new Map<string, { senderId: number; sessionId: string }>();
let manifest: ReturnType<typeof buildDefaultClientBootManifest> | undefined;
let controller: HarnessController;

async function loadClientManifest() {
  manifest ??= buildDefaultClientBootManifest();
  try {
    return await manifest;
  } catch (error) {
    manifest = undefined;
    throw error;
  }
}

function broadcastState(state: HarnessState): void {
  for (const window of BrowserWindow.getAllWindows()) {
    window.webContents.send(bridgeChannels.state, state);
  }
}

function sendProjection(subscriptionId: string, frame: ProjectionFrame): void {
  const subscription = subscriptions.get(subscriptionId);
  if (!subscription) return;
  const sender = BrowserWindow.getAllWindows()
    .map((window: BrowserWindow) => window.webContents)
    .find((contents: WebContents) =>
      contents.id === subscription.senderId && !contents.isDestroyed());
  sender?.send(bridgeChannels.projection, { subscriptionId, frame });
}

async function bootResult(): Promise<DesktopBootResult> {
  try {
    const clientManifest = await loadClientManifest();
    const sessionId = await controller.whenReady();
    return { ready: true, sessionId, manifest: clientManifest };
  } catch (error) {
    return { ready: false, manifest: [], error: errorMessage(error) };
  }
}

function removeSenderSubscriptions(sender: WebContents): void {
  for (const [id, subscription] of subscriptions) {
    if (subscription.senderId !== sender.id) continue;
    subscriptions.delete(id);
    controller.unfollow(id);
  }
}

function createWindow(): BrowserWindow {
  const window = new BrowserWindow({
    width: 1180,
    height: 780,
    minWidth: 720,
    minHeight: 520,
    show: false,
    backgroundColor: "#f7f7f5",
    ...(process.platform === "darwin"
      ? { titleBarStyle: "hiddenInset" as const, trafficLightPosition: { x: 18, y: 16 } }
      : { titleBarStyle: "hidden" as const }),
    webPreferences: {
      preload: join(directory, "../preload/index.cjs"),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true,
    },
  });
  window.once("ready-to-show", () => window.show());
  window.webContents.setWindowOpenHandler(({ url }) => {
    let external: URL;
    try {
      external = new URL(url);
    } catch {
      return { action: "deny" };
    }
    if (external.protocol === "https:" || external.protocol === "http:" || external.protocol === "mailto:") {
      void shell.openExternal(external.href).catch((error: unknown) => {
        console.error(`[desktop] Failed to open external URL: ${String(error)}`);
      });
    }
    return { action: "deny" };
  });
  window.webContents.on("will-navigate", (event: Event, url: string) => {
    if (url !== window.webContents.getURL()) event.preventDefault();
  });
  const contents = window.webContents;
  contents.once("destroyed", () => removeSenderSubscriptions(contents));
  if (process.env.ELECTRON_RENDERER_URL) {
    void window.loadURL(process.env.ELECTRON_RENDERER_URL);
  } else {
    void window.loadFile(join(directory, "../renderer/index.html"));
  }
  return window;
}

if (!app.requestSingleInstanceLock()) {
  app.quit();
} else {
  app.on("second-instance", () => {
    const window = BrowserWindow.getAllWindows()[0];
    if (!window) return;
    if (window.isMinimized()) window.restore();
    window.show();
    window.focus();
  });
  void (async () => {
    await app.whenReady();
    controller = new HarnessController(
      join(app.getPath("userData"), "v2", "sessions"),
      process.env.WUU_V2_WORKSPACE ?? process.cwd(),
      () => subscriptions.entries(),
      sendProjection,
      broadcastState,
    );
    void controller.start().catch(() => {});

    ipcMain.handle(bridgeChannels.boot, () => bootResult());
    ipcMain.handle(bridgeChannels.restart, async () => {
      void controller.start().catch(() => {});
      return bootResult();
    });
    ipcMain.handle(bridgeChannels.action, (_event: IpcMainInvokeEvent, action: unknown, input: unknown) => {
      if (typeof action !== "string" || !action || !isJsonValue(input)) {
        throw new Error("Invalid Harness action request");
      }
      return controller.action(action, input);
    });
    ipcMain.on(bridgeChannels.follow, (event: IpcMainEvent, subscriptionId: string, followedSessionId: string) => {
      if (!isSubscriptionId(subscriptionId) || !isSessionId(followedSessionId)) return;
      const existing = subscriptions.get(subscriptionId);
      if (existing && existing.senderId !== event.sender.id) return;
      subscriptions.set(subscriptionId, { senderId: event.sender.id, sessionId: followedSessionId });
      void controller.follow(subscriptionId, followedSessionId).catch(() => {});
    });
    ipcMain.on(bridgeChannels.unfollow, (event: IpcMainEvent, subscriptionId: string) => {
      if (!isSubscriptionId(subscriptionId)) return;
      if (subscriptions.get(subscriptionId)?.senderId !== event.sender.id) return;
      subscriptions.delete(subscriptionId);
      controller.unfollow(subscriptionId);
    });

    createWindow();
    app.on("activate", () => {
      if (!BrowserWindow.getAllWindows().length) createWindow();
    });
    app.on("window-all-closed", () => {
      if (process.platform !== "darwin") app.quit();
    });
    let quitting = false;
    app.on("before-quit", (event) => {
      if (quitting) return;
      event.preventDefault();
      quitting = true;
      void controller.stop().finally(() => app.quit());
    });
  })().catch((error) => {
    console.error(`[desktop] startup failed: ${errorMessage(error)}`);
    app.quit();
  });
}
