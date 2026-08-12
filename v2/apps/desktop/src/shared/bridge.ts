import type { JsonValue, ProjectionFrame } from "@wuu-v2/contracts";
import type { ClientBootEntry } from "@wuu-v2/profile-default/client";

export type DesktopBootResult =
  | { ready: true; sessionId: string; manifest: ClientBootEntry[] }
  | { ready: false; manifest: ClientBootEntry[]; error: string };

export type HarnessState =
  | { state: "starting" }
  | { state: "ready" }
  | { state: "failed"; error: string };

export interface DesktopBridge {
  boot(): Promise<DesktopBootResult>;
  restart(): Promise<DesktopBootResult>;
  action(action: string, input: JsonValue): Promise<JsonValue | undefined>;
  follow(sessionId: string, listener: (frame: ProjectionFrame) => void): () => void;
  onHarnessState(listener: (state: HarnessState) => void): () => void;
}

export type HarnessInboundMessage =
  | { type: "action"; id: string; action: string; input: JsonValue }
  | { type: "follow"; subscriptionId: string; sessionId: string }
  | { type: "unfollow"; subscriptionId: string }
  | { type: "ping"; id: string };

export type HarnessOutboundMessage =
  | { type: "ready"; sessionId: string }
  | { type: "response"; id: string; value?: JsonValue; error?: string }
  | { type: "projection"; subscriptionId: string; frame: ProjectionFrame }
  | { type: "pong"; id: string }
  | { type: "fatal"; error: string };

export const bridgeChannels = {
  boot: "wuu-v2:boot",
  restart: "wuu-v2:restart",
  action: "wuu-v2:action",
  follow: "wuu-v2:follow",
  unfollow: "wuu-v2:unfollow",
  projection: "wuu-v2:projection",
  state: "wuu-v2:harness-state",
} as const;

export function isJsonValue(value: unknown, seen = new WeakSet<object>()): value is JsonValue {
  if (value === null || typeof value === "string" || typeof value === "boolean") return true;
  if (typeof value === "number") return Number.isFinite(value);
  if (!value || typeof value !== "object" || seen.has(value)) return false;
  seen.add(value);
  const valid = Array.isArray(value)
    ? value.every((item) => isJsonValue(item, seen))
    : (Object.getPrototypeOf(value) === Object.prototype || Object.getPrototypeOf(value) === null) &&
      Object.values(value).every((item) => isJsonValue(item, seen));
  seen.delete(value);
  return valid;
}

export function isSessionId(value: unknown): value is string {
  return typeof value === "string" &&
    /^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/.test(value) &&
    !value.includes("..");
}

export function isSubscriptionId(value: unknown): value is string {
  return typeof value === "string" && /^[A-Za-z0-9-]{1,128}$/.test(value);
}
