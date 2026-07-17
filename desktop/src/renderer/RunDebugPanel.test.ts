import { afterEach, describe, expect, it } from "vitest";
import { initialState } from "./AppState";
import {
  runDebugEventFromServerEvent,
  runDebugModelSelection,
  runDebugPhaseForState,
} from "./RunDebugPanel";
import { setActiveLocale } from "./i18n";
import type { InitializeResult, Thread, Turn } from "../shared/protocol";

afterEach(() => {
  setActiveLocale("zh-CN");
});

describe("RunDebugPanel localization", () => {
  it("localizes runtime phases in English", () => {
    setActiveLocale("en-US");

    expect(runDebugPhaseForState(initialState)).toMatchObject({
      label: "Runtime not ready",
      detail: "Connecting",
      tone: "running",
    });
  });

  it("localizes server event details and locale-formats counts", () => {
    setActiveLocale("en-US");
    const deltaSeen = new Set<string>();

    expect(
      runDebugEventFromServerEvent(
        {
          kind: "server-request",
          workdir: "/repo",
          message: { id: "request-1", method: "approval/request", params: {} },
        },
        deltaSeen,
      ),
    ).toMatchObject({ detail: "The server is waiting for a client response" });

    expect(
      runDebugEventFromServerEvent(
        {
          kind: "notification",
          workdir: "/repo",
          message: {
            method: "item/agentMessage/delta",
            params: { turn_id: "turn-1", item_id: "item-1", delta: "hello" },
          },
        },
        deltaSeen,
      ),
    ).toMatchObject({ detail: "First chunk: 5 characters" });
  });
});

describe("runDebugModelSelection", () => {
  const initialized = {
    provider: "provider-b",
    model: "model-b",
    variant: "",
    effort: "",
  } as unknown as InitializeResult;

  it("reports the global default when no thread or turn is active", () => {
    expect(runDebugModelSelection(initialized, undefined, undefined)).toEqual({
      provider: "provider-b",
      model: "model-b",
      variant: "",
      effort: "",
    });
  });

  it("prefers the active thread's pinned selection over the global default", () => {
    const thread = {
      model_provider: "provider-a",
      model: "model-a",
      model_variant: "high",
      model_effort: "medium",
    } as unknown as Thread;
    expect(runDebugModelSelection(initialized, thread, undefined)).toEqual({
      provider: "provider-a",
      model: "model-a",
      variant: "high",
      effort: "medium",
    });
  });

  it("prefers the active turn's captured model over thread and global", () => {
    const thread = {
      model_provider: "provider-a",
      model: "model-a",
    } as unknown as Thread;
    const turn = {
      model_provider: "provider-c",
      model: "model-c",
    } as unknown as Turn;
    expect(runDebugModelSelection(initialized, thread, turn)).toMatchObject({
      provider: "provider-c",
      model: "model-c",
    });
  });

  it("returns undefined when the runtime is not initialized", () => {
    expect(runDebugModelSelection(undefined, undefined, undefined)).toBeUndefined();
  });
});
