import { createDefaultHostProfile } from "@wuu-v2/profile-default/host";
import { providersFromEnvironment } from "@wuu-v2/profile-default/provider-environment";
import type {
  HarnessInboundMessage,
  HarnessOutboundMessage,
} from "../shared/bridge.js";
import {
  isJsonValue,
  isSessionId,
  isSubscriptionId,
} from "../shared/bridge.js";

function send(message: HarnessOutboundMessage): void {
  if (process.connected) process.send?.(message);
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

async function run(): Promise<void> {
  const cwd = process.env.WUU_V2_WORKSPACE;
  const dataDirectory = process.env.WUU_V2_DATA_DIR;
  if (!cwd || !dataDirectory) {
    throw new Error("Harness requires a workspace and data directory");
  }

  const runtime = await createDefaultHostProfile({
    cwd,
    dataDirectory,
    providers: providersFromEnvironment(process.env, true),
    ...(process.env.WUU_V2_DEFAULT_MODEL
      ? { defaultModelId: process.env.WUU_V2_DEFAULT_MODEL }
      : {}),
  });
  const subscriptions = new Map<string, () => void>();
  let sessionId: string;
  try {
    await runtime.ctx.agentRuns.recoverAll();
    sessionId = await runtime.openSession(process.env.WUU_V2_SESSION_ID);
  } catch (error) {
    await runtime.dispose();
    throw error;
  }

  process.on("message", (candidate: unknown) => {
    if (!candidate || typeof candidate !== "object" || Array.isArray(candidate)) return;
    const message = candidate as Partial<HarnessInboundMessage>;
    try {
      if (message.type === "ping") {
        if (typeof message.id === "string" && message.id) send({ type: "pong", id: message.id });
        return;
      }
      if (message.type === "follow") {
        if (
          !isSubscriptionId(message.subscriptionId) ||
          !isSessionId(message.sessionId)
        ) return;
        subscriptions.get(message.subscriptionId)?.();
        subscriptions.set(
          message.subscriptionId,
          runtime.ctx.projectionFeed.follow(message.sessionId, (frame) => {
            send({ type: "projection", subscriptionId: message.subscriptionId!, frame });
          }),
        );
        return;
      }
      if (message.type === "unfollow") {
        if (!isSubscriptionId(message.subscriptionId)) return;
        subscriptions.get(message.subscriptionId)?.();
        subscriptions.delete(message.subscriptionId);
        return;
      }
      if (message.type === "action") {
        if (typeof message.id !== "string" || !message.id) return;
        if (typeof message.action !== "string" || !message.action || !isJsonValue(message.input)) {
          send({ type: "response", id: message.id, error: "Invalid Harness action request" });
          return;
        }
        void runtime.ctx.hostActions.execute(message.action, message.input).then(
          (value) => send({
            type: "response",
            id: message.id!,
            ...(value === undefined ? {} : { value }),
          }),
          (error) => send({ type: "response", id: message.id!, error: errorMessage(error) }),
        );
      }
    } catch (error) {
      console.error(`[harness] rejected inbound message: ${errorMessage(error)}`);
    }
  });

  let closing = false;
  async function close(exitCode: number): Promise<void> {
    if (closing) return;
    closing = true;
    for (const stop of subscriptions.values()) stop();
    subscriptions.clear();
    await runtime.dispose();
    process.exit(exitCode);
  }

  process.once("SIGTERM", () => void close(0));
  process.once("SIGINT", () => void close(0));
  process.once("disconnect", () => void close(0));
  send({ type: "ready", sessionId });
}

void run().catch((error) => {
  const message: HarnessOutboundMessage = { type: "fatal", error: errorMessage(error) };
  if (process.connected && process.send) {
    process.send(message, () => process.exit(1));
  } else {
    process.exit(1);
  }
});
