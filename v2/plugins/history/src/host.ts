import { randomUUID } from "node:crypto";
import type {
  AgentSessionRecord,
  JsonValue,
  SessionEvent,
} from "@wuu-v2/contracts";
import { Service, type Context, type Plugin } from "@wuu-v2/kernel";
import type { HistoryEntry, HistoryRecord } from "./shared.js";

const source = { pluginId: "history", generation: "v1" } as const;

function hasMarker(events: readonly SessionEvent[]): boolean {
  return events.some((event) => event.record.type === "history/session-created");
}

function firstPrompt(events: readonly SessionEvent[]): string | undefined {
  for (const event of events) {
    const record = event.record as AgentSessionRecord;
    if (record.type !== "agent/user-message") continue;
    const text = record.data.content.map((item) => item.text).join(" ").replace(/\s+/g, " ").trim();
    if (text) return text.length > 72 ? `${text.slice(0, 69)}…` : text;
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
    ctx.hostActions.register("history/list", async () => ({ sessions: await this.list() }));
    ctx.hostActions.register("history/create", async () => ({ sessionId: await this.create() }));
  }

  async list(): Promise<HistoryEntry[]> {
    const entries: HistoryEntry[] = [];
    for (const id of await this.ctx.sessions.list()) {
      const events = await this.ctx.sessions.load(id);
      if (!hasMarker(events)) continue;
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

  async create(): Promise<string> {
    return this.ensure(`session-${randomUUID()}`);
  }

  async ensure(sessionId: string): Promise<string> {
    const events = await this.ctx.sessions.load(sessionId);
    if (hasMarker(events)) return sessionId;
    await this.ctx.modelRouting.initialize(sessionId);
    await this.ctx.toolPolicy.initialize(sessionId, "full-access");
    await this.ctx.sessions.append(sessionId, source, {
      type: "history/session-created",
      data: { version: 1 },
    } satisfies HistoryRecord);
    return sessionId;
  }

  async openLatestOrCreate(): Promise<string> {
    return (await this.list())[0]?.id ?? this.create();
  }
}

declare module "cordis" {
  interface Context {
    historySessions: HistorySessionsService;
  }
}

const historyHost: Plugin = function history(ctx) {
  new HistorySessionsService(ctx);
};

historyHost.inject = ["hostActions", "modelRouting", "sessions", "toolPolicy"];
historyHost.provide = "historySessions";
export default historyHost;
