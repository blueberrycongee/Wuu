import { randomUUID } from "node:crypto";
import type {
  AgentSessionRecord,
  JsonValue,
  SessionEvent,
} from "@wuu-v2/contracts";
import { Service, type Context, type Plugin } from "@wuu-v2/kernel";
import type {
  HistoryEntry,
  HistoryEntryProjection,
  HistoryRecord,
} from "./shared.js";

const source = { pluginId: "history", generation: "v1" } as const;

function hasMarker(events: readonly SessionEvent[]): boolean {
  return events.some((event) => event.record.type === "history/session-created");
}

function workspaceId(events: readonly SessionEvent[]): string | undefined {
  for (const event of events) {
    if (event.record.type !== "history/session-created") continue;
    const data = (event.record as HistoryRecord).data;
    return data.version === 2 ? data.workspaceId : "conversation";
  }
}

function inputWorkspaceId(input: JsonValue | undefined): string {
  if (!input || typeof input !== "object" || Array.isArray(input)) {
    throw new Error("History requires a Workspace");
  }
  const id = input.workspaceId;
  if (typeof id !== "string" || !/^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/.test(id)) {
    throw new Error("History received an invalid Workspace");
  }
  return id;
}

function promptTitle(record: AgentSessionRecord): string | undefined {
  if (record.type !== "agent/user-message") return;
  const text = record.data.content.map((item) => item.text).join(" ").replace(/\s+/g, " ").trim();
  if (!text) return;
  return text.length > 72 ? `${text.slice(0, 69)}…` : text;
}

function firstPrompt(events: readonly SessionEvent[]): string | undefined {
  for (const event of events) {
    const record = event.record as AgentSessionRecord;
    const title = promptTitle(record);
    if (title) return title;
  }
}

function isRunning(events: readonly SessionEvent[]): boolean {
  const open = new Set<string>();
  for (const event of events) {
    const record = event.record as AgentSessionRecord;
    if (record.type !== "agent/run-state") continue;
    if (record.data.state === "started") open.add(record.data.runId);
    else open.delete(record.data.runId);
  }
  return open.size > 0;
}

export class HistorySessionsService extends Service {
  constructor(ctx: Context) {
    super(ctx, "historySessions");
    ctx.hostActions.register("history/list", async (input) => ({ sessions: await this.list(inputWorkspaceId(input)) }));
    ctx.hostActions.register("history/create", async (input) => ({ sessionId: await this.create(inputWorkspaceId(input)) }));
  }

  async list(workspace: string): Promise<HistoryEntry[]> {
    const entries: HistoryEntry[] = [];
    for (const id of await this.ctx.sessions.list()) {
      const events = await this.ctx.sessions.load(id);
      if (!hasMarker(events)) continue;
      if (workspaceId(events) !== workspace) continue;
      entries.push({
        id,
        title: firstPrompt(events) ?? "New task",
        updatedAt: events.at(-1)?.time ?? new Date(0).toISOString(),
        running: isRunning(events),
      });
    }
    return entries.sort((left, right) =>
      right.updatedAt.localeCompare(left.updatedAt) || left.id.localeCompare(right.id));
  }

  async create(workspace: string): Promise<string> {
    return this.ensure(`session-${randomUUID()}`, workspace);
  }

  async ensure(sessionId: string, workspace = "conversation"): Promise<string> {
    const events = await this.ctx.sessions.load(sessionId);
    if (hasMarker(events)) return sessionId;
    await this.ctx.modelRouting.initialize(sessionId);
    await this.ctx.toolPolicy.initialize(sessionId, "full-access");
    await this.ctx.sessions.append(sessionId, source, {
      type: "history/session-created",
      data: { version: 2, workspaceId: workspace },
    } satisfies HistoryRecord);
    return sessionId;
  }

  async openLatestOrCreate(): Promise<string> {
    return (await this.list("conversation"))[0]?.id ?? this.create("conversation");
  }
}

declare module "cordis" {
  interface Context {
    historySessions: HistorySessionsService;
  }
}

const historyHost: Plugin = function history(ctx) {
  new HistorySessionsService(ctx);
  ctx.projections.register("history/entry", (current, event) => {
    const record = event.record as AgentSessionRecord | HistoryRecord;
    if (record.type === "history/session-created") {
      return {
        title: "New task",
        updatedAt: event.time,
        running: false,
        runningRunIds: [],
        hasPrompt: false,
      } satisfies HistoryEntryProjection;
    }
    if (!current || typeof current !== "object" || Array.isArray(current)) return current;
    const value = current as unknown as HistoryEntryProjection;
    const prompt = value.hasPrompt ? undefined : promptTitle(record as AgentSessionRecord);
    const title = prompt ?? value.title;
    const runningRunIds = [...(Array.isArray(value.runningRunIds) ? value.runningRunIds : [])];
    if (record.type === "agent/run-state") {
      if (record.data.state === "started") {
        if (!runningRunIds.includes(record.data.runId)) runningRunIds.push(record.data.runId);
      } else {
        const index = runningRunIds.indexOf(record.data.runId);
        if (index >= 0) runningRunIds.splice(index, 1);
      }
    }
    return {
      title,
      updatedAt: event.time,
      running: runningRunIds.length > 0,
      runningRunIds,
      hasPrompt: value.hasPrompt || prompt !== undefined,
    } satisfies HistoryEntryProjection;
  });
};

historyHost.inject = ["hostActions", "modelRouting", "projections", "sessions", "toolPolicy"];
historyHost.provide = "historySessions";
export default historyHost;
