import { randomUUID } from "node:crypto";
import { tmpdir } from "node:os";
import { join } from "node:path";
import type { AgentSessionRecord } from "@wuu-v2/contracts";
import { agentRuntimePlugin } from "@wuu-v2/plugin-agent-runtime";
import { contextProjectionPlugin } from "@wuu-v2/plugin-context-projection";
import { conversationProjectionPlugin } from "@wuu-v2/plugin-conversation";
import { defaultAgentLoopPlugin } from "@wuu-v2/plugin-default-agent-loop";
import { corePromptPlugin } from "@wuu-v2/plugin-prompt-core";
import { openAIProviderPlugin } from "@wuu-v2/plugin-provider-openai";
import { scriptedProviderPlugin } from "@wuu-v2/plugin-provider-scripted";
import { jsonlSessionPlugin } from "@wuu-v2/plugin-session-jsonl";
import { basicToolsPlugin } from "@wuu-v2/plugin-tools-basic";
import { createKernelContext, kernelPlugin } from "@wuu-v2/kernel";

const smoke = process.argv.includes("--smoke");
const cwd = process.cwd();
const sessionId = process.env.WUU_V2_SESSION_ID ?? `session-${randomUUID()}`;
const directory = smoke
  ? join(tmpdir(), `wuu-v2-smoke-${process.pid}`)
  : process.env.WUU_V2_DATA_DIR ?? join(cwd, ".wuu-v2", "sessions");
const prompt = smoke
  ? "Read package.json and then confirm the smoke run completed."
  : process.argv.slice(2).filter((arg) => !arg.startsWith("--")).join(" ").trim();

if (!prompt) {
  throw new Error("Pass a prompt or use --smoke");
}

const ctx = createKernelContext();
const fibers: Array<{ dispose(): Promise<void> }> = [];
fibers.push(await ctx.plugin(kernelPlugin));
fibers.push(await ctx.plugin(jsonlSessionPlugin, { directory }));
fibers.push(await ctx.plugin(corePromptPlugin, { cwd }));
fibers.push(await ctx.plugin(basicToolsPlugin, { cwd }));
fibers.push(await ctx.plugin(contextProjectionPlugin));
fibers.push(await ctx.plugin(conversationProjectionPlugin));

let providerId: string;
if (smoke) {
  providerId = "scripted";
  fibers.push(await ctx.plugin(scriptedProviderPlugin, {
    rounds: [
      {
        toolCalls: [{
          type: "tool_call",
          callId: "smoke-read",
          name: "read",
          input: { path: "package.json" },
        }],
      },
      { text: "Smoke run completed." },
    ],
  }));
} else {
  providerId = "openai";
  const apiKey = process.env.WUU_V2_OPENAI_API_KEY;
  const model = process.env.WUU_V2_MODEL;
  if (!apiKey || !model) {
    throw new Error("Set WUU_V2_OPENAI_API_KEY and WUU_V2_MODEL");
  }
  fibers.push(await ctx.plugin(openAIProviderPlugin, {
    apiKey,
    model,
    ...(process.env.WUU_V2_OPENAI_BASE_URL
      ? { baseUrl: process.env.WUU_V2_OPENAI_BASE_URL }
      : {}),
  }));
}
fibers.push(await ctx.plugin(defaultAgentLoopPlugin, { providerId }));
fibers.push(await ctx.plugin(agentRuntimePlugin, { agentId: "default" }));

const shutdown = async () => {
  for (const fiber of fibers.reverse()) await fiber.dispose();
  await ctx.fiber.dispose();
};
process.once("SIGINT", () => void shutdown().finally(() => process.exit(130)));
process.once("SIGTERM", () => void shutdown().finally(() => process.exit(143)));

try {
  let recoverySessionId: string | undefined;
  let recoveryRunId: string | undefined;
  if (smoke) {
    recoverySessionId = `${sessionId}-recovery`;
    recoveryRunId = randomUUID();
    await ctx.sessions.appendBatch(recoverySessionId, {
      pluginId: "harness-smoke",
      generation: "v1",
    }, [
      {
        type: "agent/run-state",
        data: { runId: recoveryRunId, state: "started" },
      },
      {
        type: "agent/assistant-started",
        data: { messageId: "interrupted-message" },
      },
      {
        type: "agent/assistant-tool-call",
        data: {
          messageId: "interrupted-message",
          call: {
            type: "tool_call",
            callId: "interrupted-call",
            name: "write",
            input: { path: "unknown" },
          },
        },
      },
    ] satisfies AgentSessionRecord[]);
  }

  const recovered = await ctx.agentRuns.recoverAll();
  const acceptance = await ctx.hostActions.execute("agent/prompt", { sessionId, text: prompt });
  if (!acceptance || Array.isArray(acceptance) || typeof acceptance !== "object") {
    throw new Error("agent prompt was not accepted");
  }
  const runId = acceptance.runId;
  if (typeof runId !== "string") throw new Error("agent prompt acceptance omitted runId");
  const result = await ctx.agentRuns.wait(sessionId, runId);
  const events = await ctx.sessions.load(sessionId);
  const projections = await ctx.projections.build(ctx.sessions, sessionId);
  const recordTypes = events.map((event) => event.record.type);
  if (smoke) {
    const recoveryEvents = await ctx.sessions.load(recoverySessionId!);
    const recoveryTypes = recoveryEvents.map((event) => event.record.type);
    const recoveryState = recoveryEvents
      .map((event) => event.record as AgentSessionRecord)
      .findLast((record) => record.type === "agent/run-state");
    if (
      result.status !== "completed" ||
      !recovered.includes(recoveryRunId!) ||
      recoveryState?.type !== "agent/run-state" ||
      recoveryState.data.state !== "interrupted" ||
      !recoveryTypes.includes("agent/tool-result") ||
      !recoveryTypes.includes("agent/assistant-completed") ||
      !recordTypes.includes("agent/assistant-tool-call") ||
      !recordTypes.includes("agent/tool-result") ||
      !projections.some(({ key }) => key === "conversation")
    ) {
      throw new Error("smoke run did not complete the model-tool-result loop");
    }
  }
  console.log(JSON.stringify({
    runtime: "wuu-v2",
    sessionId,
    status: result.status,
    lastSeq: events.at(-1)?.seq ?? 0,
    projectionKeys: projections.map(({ key }) => key),
    recordTypes,
  }));
} finally {
  await shutdown();
}
