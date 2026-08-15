import type { ServerEvent } from "../shared/protocol";

export type TokenSpeedSource = "real" | "estimated" | "none";

export type LiveTurnContextUsage = {
  turnID: string;
  used: number;
  window: number;
  inputTokens: number;
  cacheCreationTokens: number;
  cacheReadTokens: number;
};

export type TurnTelemetrySnapshot = {
  tokensPerSecond: number;
  source: TokenSpeedSource;
  sampledAt?: number;
  contextUsage?: LiveTurnContextUsage;
  /**
   * Cumulative input tokens for the turn, provider-reported. Zero when the
   * provider does not report stream-time usage (input has no text-delta
   * estimate).
   */
  inputTokens: number;
  /**
   * Cumulative streamed output tokens for the turn. When
   * `source === "real"` this is the latest provider-reported total plus the
   * delta estimate accumulated since that sample arrived; before any real
   * usage it is the running estimate built from agent-message and reasoning
   * text deltas. Monotonic while the turn runs.
   */
  outputTokens: number;
};

type TokenSample = { tokens: number; at: number };

type TurnTelemetryEntry = {
  realSeen: boolean;
  realInputTokens: number;
  realTokens: number;
  /**
   * Delta-estimated output streamed since the last real usage sample. Once
   * real usage arrives this is re-anchored to zero because the provider
   * total already bills everything streamed so far; it then measures only
   * the in-flight output the provider has not reported yet.
   */
  estimatedTokens: number;
  realSamples: TokenSample[];
  estimatedSamples: TokenSample[];
  contextUsage?: LiveTurnContextUsage;
  snapshot: TurnTelemetrySnapshot;
};

const TOKEN_SPEED_WINDOW_MS = 2_000;
export const TURN_TELEMETRY_NOTIFY_INTERVAL_MS = 250;
const RETAINED_TURN_TELEMETRY_LIMIT = 24;
const EMPTY_SNAPSHOT: TurnTelemetrySnapshot = Object.freeze({
  tokensPerSecond: 0,
  source: "none",
  inputTokens: 0,
  outputTokens: 0,
});

const ESTIMATED_TEXT_DELTA_METHODS = new Set([
  "item/agentMessage/delta",
  "item/reasoning/delta",
]);

export class TurnTelemetryStore {
  private readonly entries = new Map<string, TurnTelemetryEntry>();
  private readonly listeners = new Set<() => void>();
  private notifyTimer: ReturnType<typeof setTimeout> | undefined;

  readonly subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  };

  readonly getSnapshot = (turnID: string | undefined): TurnTelemetrySnapshot => {
    if (!turnID) return EMPTY_SNAPSHOT;
    return this.entries.get(turnID)?.snapshot ?? EMPTY_SNAPSHOT;
  };

  ingest(event: ServerEvent, at = Date.now()): void {
    if (event.kind !== "notification") return;
    const params = isRecord(event.message.params) ? event.message.params : undefined;
    const turnID = stringValue(params, "turn_id");
    if (!turnID) return;

    if (event.message.method === "turn/usage") {
      this.ingestRealUsage(turnID, params, at);
      return;
    }
    if (!ESTIMATED_TEXT_DELTA_METHODS.has(event.message.method)) return;
    this.ingestEstimatedDelta(turnID, params, at);
  }

  reset(): void {
    this.entries.clear();
    if (this.notifyTimer !== undefined) {
      clearTimeout(this.notifyTimer);
      this.notifyTimer = undefined;
    }
  }

  private ingestRealUsage(
    turnID: string,
    params: Record<string, unknown> | undefined,
    at: number,
  ): void {
    const entry = this.entry(turnID);
    const outputTokens = Math.max(0, numberValue(params, "output_tokens") ?? 0);
    const inputTokens = Math.max(0, numberValue(params, "input_tokens") ?? 0);
    entry.realSeen = true;
    if (outputTokens > entry.realTokens) {
      entry.realTokens = outputTokens;
      entry.realSamples = appendSample(entry.realSamples, outputTokens, at);
      // The provider total bills every token streamed up to this sample, so
      // re-anchor the delta estimate to zero. From here on it measures only
      // output streamed since this sample, which most providers will not
      // report again until the next request ends. Without this re-anchor the
      // estimate would keep double-counting already-billed output.
      entry.estimatedTokens = 0;
      entry.estimatedSamples = [];
    }
    // Input is cumulative per provider request too; keep the max so later
    // requests in the same turn never regress the displayed total.
    if (inputTokens > entry.realInputTokens) {
      entry.realInputTokens = inputTokens;
    }

    const contextTokens = numberValue(params, "context_tokens") ?? 0;
    const contextWindowTokens = numberValue(params, "context_window_tokens") ?? 0;
    if (contextTokens > 0 && contextWindowTokens > 0) {
      entry.contextUsage = {
        turnID,
        used: contextTokens,
        window: contextWindowTokens,
        inputTokens: Math.max(0, numberValue(params, "input_tokens") ?? 0),
        cacheCreationTokens: Math.max(
          0,
          numberValue(params, "cache_creation_tokens") ?? 0,
        ),
        cacheReadTokens: Math.max(
          0,
          numberValue(params, "cache_read_tokens") ?? 0,
        ),
      };
    }
    this.refreshSnapshot(entry, "real");
    this.scheduleNotify();
  }

  private ingestEstimatedDelta(
    turnID: string,
    params: Record<string, unknown> | undefined,
    at: number,
  ): void {
    const entry = this.entry(turnID);
    const delta = stringValue(params, "delta");
    if (!delta) return;
    const estimatedTokens = estimateStreamingOutputTokens(delta);
    if (estimatedTokens <= 0) return;
    // Providers report stream-time usage only at request boundaries (most of
    // them only at request end), so during a thinking or text phase the
    // deltas are the only live signal. Keep accumulating them even after
    // real usage has been seen: they now represent output streamed since the
    // last real sample and are added on top of that real baseline, which
    // keeps the counter climbing while the model is thinking. Skipping them
    // here froze the counter for every thinking phase after the first
    // provider request of a turn.
    entry.estimatedTokens += estimatedTokens;
    if (!entry.realSeen) {
      entry.estimatedSamples = appendSample(
        entry.estimatedSamples,
        entry.estimatedTokens,
        at,
      );
    }
    this.refreshSnapshot(entry, entry.realSeen ? "real" : "estimated");
    this.scheduleNotify();
  }

  private entry(turnID: string): TurnTelemetryEntry {
    const existing = this.entries.get(turnID);
    if (existing) return existing;
    const created: TurnTelemetryEntry = {
      realSeen: false,
      realInputTokens: 0,
      realTokens: 0,
      estimatedTokens: 0,
      realSamples: [],
      estimatedSamples: [],
      snapshot: EMPTY_SNAPSHOT,
    };
    this.entries.set(turnID, created);
    while (this.entries.size > RETAINED_TURN_TELEMETRY_LIMIT) {
      const oldest = this.entries.keys().next().value as string | undefined;
      if (!oldest) break;
      this.entries.delete(oldest);
    }
    return created;
  }

  private refreshSnapshot(
    entry: TurnTelemetryEntry,
    source: Exclude<TokenSpeedSource, "none">,
  ): void {
    const samples = source === "real" ? entry.realSamples : entry.estimatedSamples;
    entry.snapshot = {
      tokensPerSecond: tokenSpeed(samples),
      source,
      sampledAt: samples.at(-1)?.at,
      inputTokens: source === "real" ? entry.realInputTokens : 0,
      outputTokens:
        source === "real"
          ? entry.realTokens + entry.estimatedTokens
          : entry.estimatedTokens,
      ...(entry.contextUsage ? { contextUsage: entry.contextUsage } : {}),
    };
  }

  private scheduleNotify(): void {
    if (this.notifyTimer !== undefined) return;
    this.notifyTimer = setTimeout(() => {
      this.notifyTimer = undefined;
      for (const listener of this.listeners) listener();
    }, TURN_TELEMETRY_NOTIFY_INTERVAL_MS);
  }
}

function appendSample(samples: TokenSample[], tokens: number, at: number): TokenSample[] {
  const cutoff = at - TOKEN_SPEED_WINDOW_MS;
  return [...samples.filter((sample) => sample.at >= cutoff), { tokens, at }];
}

function tokenSpeed(samples: readonly TokenSample[]): number {
  if (samples.length < 2) return 0;
  const first = samples[0];
  const last = samples[samples.length - 1];
  const elapsed = last.at - first.at;
  const delta = last.tokens - first.tokens;
  return elapsed > 0 && delta > 0 ? (delta / elapsed) * 1_000 : 0;
}

export function estimateStreamingOutputTokens(text: string): number {
  let ascii = 0;
  let nonAscii = 0;
  for (const char of text) {
    const codePoint = char.codePointAt(0) ?? 0;
    if (codePoint <= 0x7f) ascii += 1;
    else nonAscii += 1;
  }
  return ascii / 4 + nonAscii / 1.7;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function stringValue(
  record: Record<string, unknown> | undefined,
  key: string,
): string | undefined {
  const value = record?.[key];
  return typeof value === "string" ? value : undefined;
}

function numberValue(
  record: Record<string, unknown> | undefined,
  key: string,
): number | undefined {
  const value = record?.[key];
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

export const turnTelemetryStore = new TurnTelemetryStore();
