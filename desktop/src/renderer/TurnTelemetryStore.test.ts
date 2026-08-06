import { afterEach, describe, expect, it, vi } from "vitest";
import type { ServerEvent } from "../shared/protocol";
import {
  TURN_TELEMETRY_NOTIFY_INTERVAL_MS,
  TurnTelemetryStore,
} from "./TurnTelemetryStore";

function notification(
  method: string,
  params: Record<string, unknown>,
): ServerEvent {
  return {
    kind: "notification",
    workdir: "/repo",
    message: { method, params },
  };
}

const turn = { thread_id: "thread-1", turn_id: "turn-1" };

afterEach(() => {
  vi.useRealTimers();
});

describe("TurnTelemetryStore", () => {
  it("measures raw text deltas independently from renderer publication cadence", () => {
    const store = new TurnTelemetryStore();
    store.ingest(
      notification("item/agentMessage/delta", { ...turn, delta: "aaaaaaaa" }),
      1_000,
    );
    store.ingest(
      notification("item/agentMessage/delta", { ...turn, delta: "bbbbbbbb" }),
      2_000,
    );

    expect(store.getSnapshot("turn-1")).toMatchObject({
      tokensPerSecond: 2,
      source: "estimated",
      sampledAt: 2_000,
    });
    store.reset();
  });

  it("does not estimate visible generation speed from tool-call JSON", () => {
    const store = new TurnTelemetryStore();
    store.ingest(
      notification("item/toolCall/delta", {
        ...turn,
        delta: JSON.stringify({ patch: "x".repeat(100_000) }),
      }),
      1_000,
    );

    expect(store.getSnapshot("turn-1").source).toBe("none");
    store.reset();
  });

  it("keeps real provider usage authoritative after fallback estimates", () => {
    const store = new TurnTelemetryStore();
    store.ingest(
      notification("item/reasoning/delta", { ...turn, delta: "aaaaaaaa" }),
      500,
    );
    store.ingest(
      notification("turn/usage", {
        ...turn,
        output_tokens: 100,
        input_tokens: 500,
        context_tokens: 600,
        context_window_tokens: 1_000,
      }),
      1_000,
    );
    store.ingest(
      notification("turn/usage", {
        ...turn,
        output_tokens: 120,
        input_tokens: 500,
        context_tokens: 620,
        context_window_tokens: 1_000,
      }),
      2_000,
    );
    store.ingest(
      notification("item/agentMessage/delta", {
        ...turn,
        delta: "z".repeat(100_000),
      }),
      2_100,
    );

    expect(store.getSnapshot("turn-1")).toMatchObject({
      tokensPerSecond: 20,
      source: "real",
      sampledAt: 2_000,
      contextUsage: {
        turnID: "turn-1",
        used: 620,
        window: 1_000,
      },
    });
    store.reset();
  });

  it("coalesces subscriber notifications without dropping raw samples", () => {
    vi.useFakeTimers();
    const store = new TurnTelemetryStore();
    const listener = vi.fn();
    store.subscribe(listener);

    for (let index = 0; index < 100; index += 1) {
      store.ingest(
        notification("item/agentMessage/delta", { ...turn, delta: "xxxx" }),
        1_000 + index,
      );
    }
    expect(listener).not.toHaveBeenCalled();

    vi.advanceTimersByTime(TURN_TELEMETRY_NOTIFY_INTERVAL_MS);
    expect(listener).toHaveBeenCalledTimes(1);
    expect(store.getSnapshot("turn-1").tokensPerSecond).toBeGreaterThan(0);
    store.reset();
  });
});
