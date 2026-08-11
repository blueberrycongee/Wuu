import { createHash } from "node:crypto";
import type { JsonValue, ModelContextSeedRecord } from "@wuu-v2/contracts";
import { Service, type Context, type Plugin } from "@wuu-v2/kernel";
import type {} from "@wuu-v2/plugin-agent-runtime";

export interface SideHostConfig {
  agentId: string;
}

const source = { pluginId: "side", generation: "v1" } as const;

function objectInput(input: JsonValue): Record<string, JsonValue> {
  if (!input || typeof input !== "object" || Array.isArray(input)) {
    throw new Error("action input must be an object");
  }
  return input;
}

function stringField(input: Record<string, JsonValue>, field: string): string {
  const value = input[field];
  if (typeof value !== "string" || !value) throw new Error(`missing string field: ${field}`);
  return value;
}

export class SideSessionsService extends Service {
  private readonly pending = new Map<string, Promise<string>>();
  private readonly pendingStarts = new Set<Promise<JsonValue>>();
  private readonly active = new Map<string, string>();
  private readonly observers = new Set<Promise<void>>();
  private readonly closingController = new AbortController();
  private closing = false;

  constructor(ctx: Context, private readonly agentId: string) {
    super(ctx, "sideSessions");
    ctx.hostActions.register("side/resolve", async (input) => {
      const value = objectInput(input);
      return { sessionId: await this.ensure(stringField(value, "sessionId")) };
    });
    ctx.hostActions.register("side/prompt", async (input) => {
      const value = objectInput(input);
      return this.startTracked(
        stringField(value, "sessionId"),
        stringField(value, "text"),
      );
    });
    ctx.hostActions.register("side/cancel", (input) => {
      const value = objectInput(input);
      return { cancelled: ctx.agentRuns.cancel(this.resolve(stringField(value, "sessionId"))) };
    });
    ctx.effect(() => async () => {
      this.closing = true;
      this.closingController.abort(new Error("Side plugin is stopping"));
      for (const sessionId of this.active.keys()) ctx.agentRuns.cancel(sessionId);
      await Promise.allSettled([...this.pending.values(), ...this.pendingStarts]);
      for (const sessionId of this.active.keys()) ctx.agentRuns.cancel(sessionId);
      await Promise.allSettled(this.observers);
    }, "stop Side runs");
  }

  private assertOpen(): void {
    if (this.closing) throw new Error("Side plugin is stopping");
  }

  resolve(mainSessionId: string): string {
    const digest = createHash("sha256")
      .update("wuu-side-session\0")
      .update(mainSessionId)
      .digest("hex")
      .slice(0, 40);
    return `side-${digest}`;
  }

  private async ensure(mainSessionId: string): Promise<string> {
    this.assertOpen();
    const existing = this.pending.get(mainSessionId);
    if (existing) return existing;
    const task = (async () => {
      const sideSessionId = this.resolve(mainSessionId);
      const existing = await this.ctx.sessions.load(sideSessionId);
      this.assertOpen();
      if (!existing.some((event) => event.record.type === "context/model-seed")) {
        const snapshot = await this.ctx.modelContext.snapshot(
          mainSessionId,
          this.closingController.signal,
        );
        this.assertOpen();
        await this.ctx.sessions.append(sideSessionId, source, {
          type: "context/model-seed",
          data: {
            sourceSessionId: mainSessionId,
            sourceSeq: snapshot.sourceSeq,
            messages: snapshot.messages,
          },
        } satisfies ModelContextSeedRecord);
      }
      this.assertOpen();
      await this.ctx.modelRouting.initialize(sideSessionId, mainSessionId);
      this.assertOpen();
      await this.ctx.toolPolicy.initialize(sideSessionId, "read-only");
      this.assertOpen();
      return sideSessionId;
    })();
    this.pending.set(mainSessionId, task);
    try {
      return await task;
    } finally {
      this.pending.delete(mainSessionId);
    }
  }

  private async startTracked(mainSessionId: string, text: string): Promise<JsonValue> {
    this.assertOpen();
    const pending = this.start(mainSessionId, text);
    this.pendingStarts.add(pending);
    try {
      return await pending;
    } finally {
      this.pendingStarts.delete(pending);
    }
  }

  private async start(mainSessionId: string, text: string): Promise<JsonValue> {
    const sideSessionId = await this.ensure(mainSessionId);
    this.assertOpen();
    const acceptance = await this.ctx.agentRuns.startWith(this.agentId, {
      sessionId: sideSessionId,
      text,
    });
    this.active.set(sideSessionId, acceptance.runId);
    if (this.closing) {
      this.ctx.agentRuns.cancel(sideSessionId);
      await this.ctx.agentRuns.wait(sideSessionId, acceptance.runId);
      this.active.delete(sideSessionId);
      throw new Error("Side plugin stopped while starting a run");
    }
    this.observe(sideSessionId, acceptance.runId);
    return { runId: acceptance.runId, acceptedSeq: acceptance.acceptedSeq };
  }

  private observe(sessionId: string, runId: string): void {
    const observer = this.ctx.agentRuns.wait(sessionId, runId)
      .catch((error) => this.ctx.logger.error(error))
      .then(() => undefined)
      .finally(() => {
        if (this.active.get(sessionId) === runId) this.active.delete(sessionId);
        this.observers.delete(observer);
      });
    this.observers.add(observer);
  }
}

declare module "cordis" {
  interface Context {
    sideSessions: SideSessionsService;
  }
}

const sideHost: Plugin<SideHostConfig> = function side(ctx, config) {
  new SideSessionsService(ctx, config.agentId);
};

sideHost.inject = [
  "agentRuns",
  "hostActions",
  "modelContext",
  "modelRouting",
  "sessions",
  "toolPolicy",
];
sideHost.provide = "sideSessions";
export default sideHost;
