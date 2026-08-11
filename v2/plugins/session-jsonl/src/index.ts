import { randomUUID } from "node:crypto";
import { mkdir, open, readFile, rename, rm } from "node:fs/promises";
import { join } from "node:path";
import type {
  EventSource,
  SessionEvent,
  SessionRecord,
} from "@wuu-v2/contracts";
import {
  Service,
  type Context,
  type Plugin,
  type SessionService,
} from "@wuu-v2/kernel";

export interface JsonlSessionConfig {
  directory: string;
}

function assertSessionId(sessionId: string): void {
  if (
    !/^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/.test(sessionId) ||
    sessionId.includes("..")
  ) {
    throw new Error("invalid session id");
  }
}

function parseEvents(sessionId: string, content: string): SessionEvent[] {
  const events = content
    .split("\n")
    .filter(Boolean)
    .map((line) => JSON.parse(line) as SessionEvent);
  for (const [index, event] of events.entries()) {
    if (
      event.sessionId !== sessionId ||
      typeof event.id !== "string" ||
      event.seq !== index + 1 ||
      typeof event.time !== "string" ||
      typeof event.source?.pluginId !== "string" ||
      typeof event.source?.generation !== "string" ||
      typeof event.record?.type !== "string"
    ) {
      throw new Error(`invalid session record at seq ${index + 1}`);
    }
  }
  return events;
}

class JsonlSessionService extends Service implements SessionService {
  private readonly listeners = new Map<string, Set<(event: SessionEvent) => void>>();
  private readonly tails = new Map<string, Promise<void>>();

  constructor(ctx: Context, private readonly directory: string) {
    super(ctx, "sessions");
  }

  private path(sessionId: string): string {
    assertSessionId(sessionId);
    return join(this.directory, `${sessionId}.jsonl`);
  }

  private async loadCommitted(sessionId: string): Promise<SessionEvent[]> {
    try {
      return parseEvents(sessionId, await readFile(this.path(sessionId), "utf8"));
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code === "ENOENT") return [];
      throw error;
    }
  }

  async load(sessionId: string): Promise<SessionEvent[]> {
    assertSessionId(sessionId);
    await this.tails.get(sessionId);
    return this.loadCommitted(sessionId);
  }

  append<R extends SessionRecord>(
    sessionId: string,
    source: EventSource,
    record: R,
  ): Promise<SessionEvent<R>> {
    assertSessionId(sessionId);
    const previous = this.tails.get(sessionId) ?? Promise.resolve();
    const operation = previous.then(async () => {
      const events = await this.loadCommitted(sessionId);
      const seq = events.at(-1)?.seq ?? 0;
      const event: SessionEvent<R> = {
        id: randomUUID(),
        sessionId,
        seq: seq + 1,
        time: new Date().toISOString(),
        source,
        record,
      };
      const path = this.path(sessionId);
      const temporary = `${path}.${randomUUID()}.tmp`;
      await mkdir(this.directory, { recursive: true });
      try {
        const file = await open(temporary, "wx", 0o600);
        try {
          await file.writeFile(
            [...events, event].map((item) => JSON.stringify(item)).join("\n") + "\n",
            "utf8",
          );
          await file.sync();
        } finally {
          await file.close();
        }
        await rename(temporary, path);
        const parent = await open(this.directory, "r");
        try {
          await parent.sync();
        } finally {
          await parent.close();
        }
      } catch (error) {
        await rm(temporary, { force: true });
        throw error;
      }

      for (const listener of this.listeners.get(sessionId) ?? []) {
        try {
          listener(event);
        } catch (error) {
          this.ctx.logger.error(error);
        }
      }
      return event;
    });
    this.tails.set(sessionId, operation.then(() => undefined, () => undefined));
    return operation;
  }

  subscribe(
    sessionId: string,
    listener: (event: SessionEvent) => void,
  ): () => void {
    assertSessionId(sessionId);
    return this.ctx.effect(() => {
      const listeners = this.listeners.get(sessionId) ?? new Set();
      listeners.add(listener);
      this.listeners.set(sessionId, listeners);
      return () => {
        listeners.delete(listener);
        if (!listeners.size) this.listeners.delete(sessionId);
      };
    }, `subscribe session:${sessionId}`);
  }
}

export const jsonlSessionPlugin: Plugin<JsonlSessionConfig> =
  function sessionJsonl(ctx: Context, config: JsonlSessionConfig) {
    new JsonlSessionService(ctx, config.directory);
  };

jsonlSessionPlugin.provide = "sessions";
