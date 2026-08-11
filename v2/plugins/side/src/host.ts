import { createHash } from "node:crypto";
import type { JsonValue, ModelContextSeedRecord } from "@wuu-v2/contracts";
import { Service, type Context, type Plugin } from "@wuu-v2/kernel";

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

  constructor(ctx: Context, private readonly agentId: string) {
    super(ctx, "sideSessions");
    ctx.hostActions.register("side/resolve", async (input) => {
      const value = objectInput(input);
      return { sessionId: await this.ensure(stringField(value, "sessionId")) };
    });
    ctx.hostActions.register("side/prompt", async (input) => {
      const value = objectInput(input);
      const mainSessionId = stringField(value, "sessionId");
      const acceptance = await ctx.agentRuns.startWith(this.agentId, {
        sessionId: await this.ensure(mainSessionId),
        text: stringField(value, "text"),
      });
      return { runId: acceptance.runId, acceptedSeq: acceptance.acceptedSeq };
    });
    ctx.hostActions.register("side/cancel", (input) => {
      const value = objectInput(input);
      return { cancelled: ctx.agentRuns.cancel(this.resolve(stringField(value, "sessionId"))) };
    });
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
    const existing = this.pending.get(mainSessionId);
    if (existing) return existing;
    const task = (async () => {
      const sideSessionId = this.resolve(mainSessionId);
      const existing = await this.ctx.sessions.load(sideSessionId);
      if (!existing.some((event) => event.record.type === "context/model-seed")) {
        const snapshot = await this.ctx.modelContext.snapshot(
          mainSessionId,
          new AbortController().signal,
        );
        await this.ctx.sessions.append(sideSessionId, source, {
          type: "context/model-seed",
          data: {
            sourceSessionId: mainSessionId,
            sourceSeq: snapshot.sourceSeq,
            messages: snapshot.messages,
          },
        } satisfies ModelContextSeedRecord);
      }
      await this.ctx.modelRouting.initialize(sideSessionId, mainSessionId);
      await this.ctx.toolPolicy.initialize(sideSessionId, "read-only");
      return sideSessionId;
    })();
    this.pending.set(mainSessionId, task);
    try {
      return await task;
    } finally {
      this.pending.delete(mainSessionId);
    }
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
