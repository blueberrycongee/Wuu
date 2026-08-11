import { randomUUID } from "node:crypto";
import type {
  AgentRunResult,
  AgentSessionRecord,
  JsonValue,
  ModelContextSeedRecord,
  SessionEvent,
} from "@wuu-v2/contracts";
import { Service, type Context, type Plugin } from "@wuu-v2/kernel";
import type {} from "@wuu-v2/plugin-agent-runtime";
import type {
  SubagentRecord,
  SubagentStatus,
  SubagentValue,
} from "./shared.js";

export interface SubagentHostConfig {
  agentId: string;
}

const source = { pluginId: "subagent", generation: "v1" } as const;
type TerminalStatus = Exclude<SubagentStatus, "starting" | "running">;

function objectInput(input: JsonValue): Record<string, JsonValue> {
  if (!input || typeof input !== "object" || Array.isArray(input)) {
    throw new Error("subagent input must be an object");
  }
  return input;
}

function stringField(input: Record<string, JsonValue>, field: string): string {
  const value = input[field];
  if (typeof value !== "string" || !value.trim()) {
    throw new Error(`missing string field: ${field}`);
  }
  return value.trim();
}

function foldSubagents(
  current: readonly SubagentValue[] | undefined,
  event: SessionEvent,
): SubagentValue[] | undefined {
  const values = current ? [...current] : [];
  if (event.record.type === "subagent/created") {
    const record = event.record as SubagentRecord & { type: "subagent/created" };
    return [...values, { ...record.data, status: "starting", runId: null }];
  }
  if (event.record.type === "subagent/run-started") {
    const record = event.record as SubagentRecord & { type: "subagent/run-started" };
    return values.map((value) => value.id === record.data.id
      ? { ...value, runId: record.data.runId, status: "running" }
      : value);
  }
  if (event.record.type === "subagent/settled") {
    const record = event.record as SubagentRecord & { type: "subagent/settled" };
    return values.map((value) => value.id === record.data.id
      ? { ...value, status: record.data.status }
      : value);
  }
  return current ? [...current] : undefined;
}

async function subagents(ctx: Context, sessionId: string): Promise<SubagentValue[]> {
  let values: SubagentValue[] | undefined;
  for (const event of await ctx.sessions.load(sessionId)) {
    values = foldSubagents(values, event);
  }
  return values ?? [];
}

async function lineage(ctx: Context, sessionId: string) {
  for (const event of await ctx.sessions.load(sessionId)) {
    if (event.record.type === "subagent/lineage") {
      return event.record as SubagentRecord & { type: "subagent/lineage" };
    }
  }
}

function runStatus(events: readonly SessionEvent[], runId: string): TerminalStatus | undefined {
  let status: AgentRunResult["status"] | "started" | undefined;
  for (const event of events) {
    const record = event.record as AgentSessionRecord;
    if (record.type === "agent/run-state" && record.data.runId === runId) {
      status = record.data.state;
    }
  }
  return status && status !== "started" ? status : undefined;
}

function latestRun(events: readonly SessionEvent[]): { runId: string; status?: TerminalStatus } | undefined {
  const states = new Map<string, AgentRunResult["status"] | "started">();
  for (const event of events) {
    const record = event.record as AgentSessionRecord;
    if (record.type === "agent/run-state") states.set(record.data.runId, record.data.state);
  }
  const latest = [...states].at(-1);
  if (!latest) return undefined;
  const [runId, status] = latest;
  return status === "started" ? { runId } : { runId, status };
}

export class SubagentService extends Service {
  private readonly observers = new Set<Promise<void>>();
  private readonly active = new Map<string, string>();
  private readonly pendingStarts = new Set<Promise<JsonValue>>();
  private closing = false;

  constructor(ctx: Context, private readonly agentId: string) {
    super(ctx, "subagents");
    ctx.hostActions.register("subagent/start", async (input) => {
      const value = objectInput(input);
      return this.startTracked(
        stringField(value, "sessionId"),
        stringField(value, "task"),
      );
    });
    ctx.hostActions.register("subagent/cancel", async (input) => {
      const value = objectInput(input);
      const sessionId = stringField(value, "sessionId");
      const id = stringField(value, "id");
      const child = (await subagents(ctx, sessionId)).find((entry) => entry.id === id);
      if (!child) throw new Error(`unknown subagent: ${id}`);
      return { cancelled: ctx.agentRuns.cancel(child.childSessionId) };
    });
    ctx.projections.register("subagents", (current, event) =>
      foldSubagents(current as SubagentValue[] | undefined, event));
    ctx.prompts.register("subagent", async (sessionId) => {
      const record = await lineage(ctx, sessionId);
      if (!record) return undefined;
      return [
        `Assigned subagent task: ${record.data.task}`,
        "Work only on this delegated task. Return a concise result in this child session.",
      ].join("\n");
    });
    ctx.effect(() => async () => {
      this.closing = true;
      for (const childSessionId of this.active.keys()) ctx.agentRuns.cancel(childSessionId);
      await Promise.allSettled(this.pendingStarts);
      for (const childSessionId of this.active.keys()) ctx.agentRuns.cancel(childSessionId);
      await Promise.allSettled(this.observers);
    }, "stop Subagent runs");
  }

  private assertOpen(): void {
    if (this.closing) throw new Error("Subagent plugin is stopping");
  }

  private async startTracked(parentSessionId: string, task: string): Promise<JsonValue> {
    this.assertOpen();
    const pending = this.start(parentSessionId, task);
    this.pendingStarts.add(pending);
    try {
      return await pending;
    } finally {
      this.pendingStarts.delete(pending);
    }
  }

  private async start(parentSessionId: string, task: string): Promise<JsonValue> {
    if (await lineage(this.ctx, parentSessionId)) {
      throw new Error("nested subagents are not supported");
    }
    this.assertOpen();
    const id = randomUUID();
    const childSessionId = `subagent-${id}`;
    const snapshot = await this.ctx.modelContext.snapshot(
      parentSessionId,
      new AbortController().signal,
    );
    this.assertOpen();
    await this.ctx.sessions.append(parentSessionId, source, {
      type: "subagent/created",
      data: { id, childSessionId, parentSeq: snapshot.sourceSeq, task },
    } satisfies SubagentRecord);

    try {
      this.assertOpen();
      await this.ctx.sessions.appendBatch(childSessionId, source, [
        {
          type: "context/model-seed",
          data: {
            sourceSessionId: parentSessionId,
            sourceSeq: snapshot.sourceSeq,
            messages: snapshot.messages,
          },
        } satisfies ModelContextSeedRecord,
        {
          type: "subagent/lineage",
          data: { id, parentSessionId, parentSeq: snapshot.sourceSeq, task },
        } satisfies SubagentRecord,
      ]);
      this.assertOpen();
      await this.ctx.modelRouting.initialize(childSessionId, parentSessionId, snapshot.sourceSeq);
      this.assertOpen();
      await this.ctx.toolPolicy.initialize(childSessionId, "read-only");
      this.assertOpen();
      const acceptance = await this.ctx.agentRuns.startWith(this.agentId, {
        sessionId: childSessionId,
        text: task,
      });
      this.active.set(childSessionId, id);
      if (this.closing) {
        this.ctx.agentRuns.cancel(childSessionId);
        const result = await this.ctx.agentRuns.wait(childSessionId, acceptance.runId);
        this.active.delete(childSessionId);
        await this.settle(parentSessionId, id, result.status);
        throw new Error("Subagent plugin stopped while starting a child run");
      }
      let started: SessionEvent;
      try {
        started = await this.ctx.sessions.append(parentSessionId, source, {
          type: "subagent/run-started",
          data: { id, runId: acceptance.runId },
        } satisfies SubagentRecord);
      } catch (error) {
        this.ctx.agentRuns.cancel(childSessionId);
        const result = await this.ctx.agentRuns.wait(childSessionId, acceptance.runId);
        this.active.delete(childSessionId);
        await this.settle(parentSessionId, id, result.status);
        throw error;
      }
      this.observe(parentSessionId, id, childSessionId, acceptance.runId);
      return {
        id,
        childSessionId,
        runId: acceptance.runId,
        acceptedSeq: started.seq,
      };
    } catch (error) {
      await this.settle(parentSessionId, id, "failed");
      throw error;
    }
  }

  private observe(
    parentSessionId: string,
    id: string,
    childSessionId: string,
    runId: string,
  ): void {
    this.active.set(childSessionId, id);
    const observer = this.ctx.agentRuns.wait(childSessionId, runId)
      .then((result) => this.settle(parentSessionId, id, result.status))
      .catch(async (error) => {
        this.ctx.logger.error(error);
        await this.settle(parentSessionId, id, "failed");
      })
      .finally(() => {
        this.active.delete(childSessionId);
        this.observers.delete(observer);
      });
    this.observers.add(observer);
  }

  private async settle(
    parentSessionId: string,
    id: string,
    status: TerminalStatus,
  ): Promise<void> {
    const current = (await subagents(this.ctx, parentSessionId)).find((entry) => entry.id === id);
    if (!current || (current.status !== "starting" && current.status !== "running")) return;
    await this.ctx.sessions.append(parentSessionId, source, {
      type: "subagent/settled",
      data: { id, status },
    } satisfies SubagentRecord);
  }

  async recover(): Promise<void> {
    for (const parentSessionId of await this.ctx.sessions.list()) {
      for (const entry of await subagents(this.ctx, parentSessionId)) {
        if (entry.status !== "starting" && entry.status !== "running") continue;
        let childEvents = await this.ctx.sessions.load(entry.childSessionId);
        const run = entry.runId
          ? { runId: entry.runId, status: runStatus(childEvents, entry.runId) }
          : latestRun(childEvents);
        if (!run) {
          await this.settle(parentSessionId, entry.id, "failed");
          continue;
        }
        if (!entry.runId) {
          await this.ctx.sessions.append(parentSessionId, source, {
            type: "subagent/run-started",
            data: { id: entry.id, runId: run.runId },
          } satisfies SubagentRecord);
        }
        if (!run.status) {
          await this.ctx.agentRuns.recoverSession(entry.childSessionId);
          childEvents = await this.ctx.sessions.load(entry.childSessionId);
        }
        const status = runStatus(childEvents, run.runId);
        await this.settle(parentSessionId, entry.id, status ?? "interrupted");
      }
    }
  }
}

declare module "cordis" {
  interface Context {
    subagents: SubagentService;
  }
}

const subagentHost: Plugin<SubagentHostConfig> = async function subagent(ctx, config) {
  const service = new SubagentService(ctx, config.agentId);
  await service.recover();
};

subagentHost.inject = [
  "agentRuns",
  "hostActions",
  "modelContext",
  "modelRouting",
  "projections",
  "prompts",
  "sessions",
  "toolPolicy",
];
subagentHost.provide = "subagents";
export default subagentHost;
