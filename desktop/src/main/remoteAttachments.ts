import { createHash } from "node:crypto";

const CHUNK_CHARS = 128 * 1024;
const CACHE_CHARS = 128 * 1024 * 1024;

/** Attachment bytes stay on the trusted desktop. References travel only inside
 * the authenticated, encrypted RPC connection, never as public download URLs. */
export class RemoteAttachments {
  private readonly entries = new Map<string, string>();
  private size = 0;

  project(value: unknown): unknown {
    if (Array.isArray(value)) return value.map(item => this.project(item));
    if (!value || typeof value !== "object") return value;
    const record = value as Record<string, unknown>;
    if (typeof record.media_type === "string" && record.media_type.startsWith("image/") && typeof record.data === "string" && record.data.length > 16 * 1024) {
      const ref = createHash("sha256").update(record.media_type).update("\0").update(record.data).digest("hex");
      if (!this.entries.has(ref)) {
        this.entries.set(ref, record.data);
        this.size += record.data.length;
      }
      this.trim(ref);
      return { ...record, data: "", remote_ref: ref };
    }
    return Object.fromEntries(Object.entries(record).map(([key, item]) => [key, this.project(item)]));
  }

  hydrate(value: unknown): unknown {
    if (Array.isArray(value)) return value.map(item => this.hydrate(item));
    if (!value || typeof value !== "object") return value;
    const record = value as Record<string, unknown>;
    if (typeof record.remote_ref === "string" && typeof record.media_type === "string" && record.media_type.startsWith("image/")) {
      const { remote_ref, ...image } = record;
      return { ...image, data: typeof image.data === "string" && image.data ? image.data : this.get(remote_ref) };
    }
    return Object.fromEntries(Object.entries(record).map(([key, item]) => [key, this.hydrate(item)]));
  }

  read(params: unknown): { data: string; total: number; offset: number } {
    const { ref, offset = 0 } = (params ?? {}) as { ref?: unknown; offset?: unknown };
    if (typeof ref !== "string" || !Number.isSafeInteger(offset) || (offset as number) < 0) throw new Error("Invalid attachment offset or reference");
    const data = this.get(ref);
    if ((offset as number) > data.length) throw new Error("Attachment offset exceeds its size");
    return { data: data.slice(offset as number, (offset as number) + CHUNK_CHARS), total: data.length, offset: offset as number };
  }

  clear(): void { this.entries.clear(); this.size = 0; }

  private get(ref: string): string {
    const data = this.entries.get(ref);
    if (data === undefined) throw new Error("Attachment expired; reopen the conversation to reload it");
    this.entries.delete(ref); this.entries.set(ref, data);
    return data;
  }

  private trim(retain: string): void {
    while ((this.size > CACHE_CHARS || this.entries.size > 2048) && this.entries.size > 1) {
      const key = this.entries.keys().next().value!;
      if (key === retain) { this.get(key); continue; }
      this.size -= this.entries.get(key)!.length;
      this.entries.delete(key);
    }
  }
}
