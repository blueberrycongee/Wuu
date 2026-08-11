import { randomUUID } from "node:crypto";
import type {
  AgentRunResult,
  AgentSessionRecord,
  EventSource,
  SessionEvent,
} from "@wuu-v2/contracts";
import { Service, type Context, type Plugin } from "@wuu-v2/kernel";

export interface AgentRuntimeConfig {
  agentId: string;
}

export interface AgentPromptInput {
  sessionId: string;
  text: string;
}

export interface AgentRunAcceptance {
  runId: string;
  acceptedSeq: number;
}

interface ActiveRun {
  runId: string;
  controller: AbortController;
  task: Promise<AgentRunResult>;
}

type RunState = Extract<AgentSessionRecord, { type: "agent/run-state" }>["data"]["state"];
const source: EventSource = { pluginId: "agent-runtime", generation: "v1" };

function objectInput(input: unknown): Record<string, unknown> {
  if (!input || typeof input !== "object" || Array.isArray(input)) {
    throw new Error("action input must be an object");
  }
  return input as Record<string, unknown>;
}

function stringField(input: Record<string, unknown>, field: string): string {
  const value = input[field];
  if (typeof value !== "string" || !value) throw new Error(`missing string field: ${field}`);
  return value;
}

function runStates(events: readonly SessionEvent[]): Map<string, RunState> {
  const states = new Map<string, RunState>();
  for (const event of events) {
    const record = event.record as AgentSessionRecord;
    if (record.type === "agent/run-state") states.set(record.data.runId, record.data.state);
  }
  return states;
}

function openRunIds(events: readonly SessionEvent[]): string[] {
  return [...runStates(events)]
    .filter(([, state]) => state === "started")
    .map(([runId]) => runId);
}

function recoveryRecords(events: readonly SessionEvent[], runId: string): AgentSessionRecord[] {
  const startIndex = events.findIndex((event) => {
    const record = event.record as AgentSessionRecord;
    return record.type === "agent/run-state" && record.data.runId === runId && record.data.state === "started";
  });
  const tail = startIndex < 0 ? [] : events.slice(startIndex + 1);
  const activeMessages = new Set<string>();
  const calls = new Map<string, string>();
  for (const event of tail) {
    const record = event.record as AgentSessionRecord;
    if (record.type === "agent/assistant-started") activeMessages.add(record.data.messageId);
    if (record.type === "agent/assistant-completed") activeMessages.delete(record.data.messageId);
    if (record.type === "agent/assistant-tool-call") calls.set(record.data.call.callId, record.data.call.name);
    if (record.type === "agent/tool-result") calls.delete(record.data.callId);
  }
  return [
    ...[...activeMessages].map((messageId): AgentSessionRecord => ({
      type: "agent/assistant-completed",
      data: { messageId, stopReason: "error" },
    })),
    ...[...calls].map(([callId, name]): AgentSessionRecord => ({
      type: "agent/tool-result",
      data: {
        callId,
        name,
        content: [{
          type: "text",
          text: "Tool outcome is unknown after interruption. Verify external state before retrying.",
        }],
        isError: true,
      },
    })),
    {
      type: "agent/run-state",
      data: { runId, state: "interrupted", error: "Agent run was interrupted by host shutdown" },
    },
  ];
}

export class AgentRuntimeService extends Service {
  private readonly active = new Map<string, ActiveRun>();
  private readonly starting = new Set<string>();

  constructor(ctx: Context, private readonly agentId: string) {
    super(ctx, "agentRuns");
    ctx.hostActions.register("agent/prompt", async (input) => {
      const value = objectInput(input);
      const acceptance = await this.start({
        sessionId: stringField(value, "sessionId"),
        text: stringField(value, "text"),
      });
      return { runId: acceptance.runId, acceptedSeq: acceptance.acceptedSeq };
    });
    ctx.hostActions.register("agent/cancel", (input) => {
      const value = objectInput(input);
      return { cancelled: this.cancel(stringField(value, "sessionId")) };
    });
    this.ctx.effect(() => async () => {
      const active = [...this.active.values()];
      for (const run of active) run.controller.abort(new Error("Agent runtime disposed"));
      await Promise.allSettled(active.map((run) => run.task));
    }, "stop active agent runs");
  }

  async start(input: AgentPromptInput): Promise<AgentRunAcceptance> {
    return this.startWith(this.agentId, input);
  }

  async startWith(agentId: string, input: AgentPromptInput): Promise<AgentRunAcceptance> {
    const text = input.text.trim();
    if (!text) throw new Error("prompt must not be empty");
    if (this.starting.has(input.sessionId) || this.active.has(input.sessionId)) {
      throw new Error(`session already has an active run: ${input.sessionId}`);
    }
    this.starting.add(input.sessionId);
    const runId = randomUUID();
    const controller = new AbortController();
    try {
      const createAgent = this.ctx.agents.require(agentId);
      const existing = openRunIds(await this.ctx.sessions.load(input.sessionId));
      if (existing.length) throw new Error(`session has an unfinished run: ${input.sessionId}`);
      const accepted = await this.ctx.sessions.appendBatch(input.sessionId, source, [
        {
          type: "agent/user-message",
          data: { messageId: randomUUID(), content: [{ type: "text", text }] },
        },
        { type: "agent/run-state", data: { runId, state: "started" } },
      ] satisfies AgentSessionRecord[]);

      const task = Promise.resolve().then(async () => {
        let result: AgentRunResult;
        try {
          result = await createAgent().run({
            sessionId: input.sessionId,
            runId,
            signal: controller.signal,
          });
        } catch (error) {
          result = { runId, status: controller.signal.aborted ? "cancelled" : "failed" };
          if (!controller.signal.aborted) this.ctx.logger.error(error);
        }
        await this.ctx.sessions.append(input.sessionId, source, {
          type: "agent/run-state",
          data: {
            runId,
            state: result.status,
            ...(result.status === "failed" ? { error: "Agent loop failed" } : {}),
          },
        } satisfies AgentSessionRecord);
        return result;
      }).finally(() => {
        if (this.active.get(input.sessionId)?.runId === runId) this.active.delete(input.sessionId);
      });
      this.active.set(input.sessionId, { runId, controller, task });
      return { runId, acceptedSeq: accepted.at(-1)!.seq };
    } finally {
      this.starting.delete(input.sessionId);
    }
  }

  cancel(sessionId: string): boolean {
    const run = this.active.get(sessionId);
    if (!run) return false;
    run.controller.abort(new Error("Cancelled by user"));
    return true;
  }

  isActive(sessionId: string): boolean {
    return this.starting.has(sessionId) || this.active.has(sessionId);
  }

  async wait(sessionId: string, runId: string): Promise<AgentRunResult> {
    const active = this.active.get(sessionId);
    if (active?.runId === runId) return active.task;
    const state = runStates(await this.ctx.sessions.load(sessionId)).get(runId);
    if (!state || state === "started") throw new Error(`run is not settled in this runtime: ${runId}`);
    return { runId, status: state };
  }

  async recoverSession(sessionId: string): Promise<string | undefined> {
    if (this.active.has(sessionId) || this.starting.has(sessionId)) return undefined;
    const events = await this.ctx.sessions.load(sessionId);
    const open = openRunIds(events);
    if (!open.length) return undefined;
    if (open.length > 1) throw new Error(`session has multiple unfinished runs: ${sessionId}`);
    await this.ctx.sessions.appendBatch(sessionId, source, recoveryRecords(events, open[0]!));
    return open[0];
  }

  async recoverAll(): Promise<string[]> {
    const recovered: string[] = [];
    for (const sessionId of await this.ctx.sessions.list()) {
      const runId = await this.recoverSession(sessionId);
      if (runId) recovered.push(runId);
    }
    return recovered;
  }
}

declare module "cordis" {
  interface Context {
    agentRuns: AgentRuntimeService;
  }
}

export const agentRuntimePlugin: Plugin<AgentRuntimeConfig> = function agentRuntime(ctx, config) {
  new AgentRuntimeService(ctx, config.agentId);
};

agentRuntimePlugin.inject = ["agents", "hostActions", "sessions"];
agentRuntimePlugin.provide = "agentRuns";
