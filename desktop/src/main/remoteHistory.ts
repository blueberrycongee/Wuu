import { createHash } from "node:crypto";

const PAGE_TURNS = 20;
const PAGE_BYTES = 256 * 1024;
type Turn = { id: string; [key: string]: unknown };
type Page = { threadID: string; turns: Turn[]; bytes: number };

/** Immutable cursors keep older history out of live snapshot traffic. They are
 * scoped to one authenticated attachment and never change a running thread. */
export class RemoteHistory {
  private pages = new Map<string, Page>();
  private bytes = 0;

  project(value: unknown): unknown {
    if (Array.isArray(value)) return value.map(item => this.project(item));
    if (!value || typeof value !== "object") return value;
    const record = value as Record<string, unknown>;
    if (typeof record.id === "string" && Array.isArray(record.turns)) {
      const page = this.slice(record.id, record.turns as Turn[]);
      return { ...record, turns: page.turns, history_cursor: page.history_cursor };
    }
    return Object.fromEntries(Object.entries(record).map(([key, item]) => [key, this.project(item)]));
  }

  read(params: unknown): { thread_id: string; cursor: string; turns: Turn[]; history_cursor?: string } {
    const cursor = (params as { cursor?: unknown } | undefined)?.cursor;
    const page = typeof cursor === "string" ? this.pages.get(cursor) : undefined;
    if (!page) throw new Error("History expired; reopen the conversation to reload it");
    return { thread_id: page.threadID, cursor: cursor as string, ...this.slice(page.threadID, page.turns) };
  }

  private slice(threadID: string, turns: Turn[]): { turns: Turn[]; history_cursor?: string } {
    let start = turns.length, bytes = 0;
    while (start > 0 && turns.length - start < PAGE_TURNS) {
      const size = Buffer.byteLength(JSON.stringify(turns[start - 1]));
      if (start < turns.length && bytes + size > PAGE_BYTES) break;
      bytes += size; start--;
    }
    if (start === 0) return { turns };
    const older = turns.slice(0, start), serialized = JSON.stringify(older);
    const cursor = createHash("sha256").update(threadID).update("\0").update(serialized).digest("hex");
    if (!this.pages.has(cursor)) {
      this.pages.set(cursor, { threadID, turns: older, bytes: Buffer.byteLength(serialized) });
      this.bytes += Buffer.byteLength(serialized);
    }
    while ((this.pages.size > 64 || this.bytes > 64 * 1024 * 1024) && this.pages.size > 1) {
      const oldest = this.pages.keys().next().value!;
      if (oldest === cursor) { const keep = this.pages.get(oldest)!; this.pages.delete(oldest); this.pages.set(oldest, keep); continue; }
      this.bytes -= this.pages.get(oldest)!.bytes; this.pages.delete(oldest);
    }
    return { turns: turns.slice(start), history_cursor: cursor };
  }
}
