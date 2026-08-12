import { randomUUID } from "node:crypto";
import { spawn } from "node:child_process";
import { rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import type { JsonValue, ProjectionFrame } from "@wuu-v2/contracts";
import type {
  HarnessInboundMessage,
  HarnessOutboundMessage,
} from "../shared/bridge.js";

const directory = join(tmpdir(), `wuu-v2-desktop-smoke-${process.pid}`);
const worker = fileURLToPath(new URL("./worker.ts", import.meta.url));
const child = spawn(process.execPath, ["--import", "tsx", worker], {
  cwd: process.cwd(),
  env: {
    ...process.env,
    WUU_V2_DATA_DIR: directory,
    WUU_V2_PROVIDER: "scripted",
    WUU_V2_WORKSPACE: process.cwd(),
  },
  stdio: ["ignore", "pipe", "pipe", "ipc"],
});
child.stdout?.pipe(process.stdout);
child.stderr?.pipe(process.stderr);

function send(message: HarnessInboundMessage): void {
  if (!child.connected) throw new Error("Harness smoke worker disconnected");
  child.send(message);
}

function objectValue(value: JsonValue | undefined): Record<string, JsonValue> | undefined {
  return value && typeof value === "object" && !Array.isArray(value) ? value : undefined;
}

function completed(frame: ProjectionFrame): boolean {
  const value = objectValue(frame.projections.find(({ key }) => key === "conversation")?.value);
  if (!value || value.running !== false || !Array.isArray(value.items)) return false;
  return value.items.some((candidate) => {
    const message = objectValue(candidate);
    return message?.kind === "message" &&
      message.role === "assistant" &&
      message.status === "stop" &&
      message.text === "Desktop Harness smoke completed.";
  });
}

let settled = false;
const exited = new Promise<void>((resolve) => child.once("exit", () => resolve()));
const timeout = setTimeout(() => {
  if (!settled) child.kill("SIGKILL");
}, 10_000);

try {
  const result = await new Promise<{ sessionId: string; lastDurableSeq: number }>((resolve, reject) => {
    child.once("error", reject);
    child.once("exit", (code, signal) => {
      if (!settled) reject(new Error(`Harness smoke exited (${signal ?? code ?? "unknown"})`));
    });
    child.on("message", (message: HarnessOutboundMessage) => {
      if (message.type === "fatal") {
        reject(new Error(message.error));
        return;
      }
      if (message.type === "response" && message.error) {
        reject(new Error(message.error));
        return;
      }
      if (message.type === "ready") {
        send({ type: "follow", subscriptionId: randomUUID(), sessionId: "../invalid" });
        const subscriptionId = randomUUID();
        send({ type: "follow", subscriptionId, sessionId: message.sessionId });
        send({
          type: "action",
          id: randomUUID(),
          action: "agent/prompt",
          input: { sessionId: message.sessionId, text: "Read package.json." },
        });
        return;
      }
      if (message.type === "projection" && completed(message.frame)) {
        send({ type: "unfollow", subscriptionId: message.subscriptionId });
        resolve({
          sessionId: message.frame.sessionId,
          lastDurableSeq: message.frame.lastDurableSeq,
        });
      }
    });
  });
  settled = true;
  console.log(JSON.stringify({ runtime: "wuu-v2-desktop-harness", status: "ready", ...result }));
} finally {
  settled = true;
  clearTimeout(timeout);
  if (child.exitCode === null && child.signalCode === null) child.kill("SIGTERM");
  await exited;
  await rm(directory, { recursive: true, force: true });
}
