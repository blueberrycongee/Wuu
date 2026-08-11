import { randomUUID } from "node:crypto";
import { tmpdir } from "node:os";
import { join } from "node:path";
import type { AgentSessionRecord } from "@wuu-v2/contracts";
import { createDefaultHostProfile } from "@wuu-v2/profile-default/host";

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

const apiKey = process.env.WUU_V2_OPENAI_API_KEY;
const model = process.env.WUU_V2_MODEL;
if (!smoke && (!apiKey || !model)) {
  throw new Error("Set WUU_V2_OPENAI_API_KEY and WUU_V2_MODEL");
}
const runtime = await createDefaultHostProfile({
  cwd,
  dataDirectory: directory,
  providers: [smoke
    ? {
        kind: "scripted",
        config: {
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
        },
      }
    : {
        kind: "openai",
        config: {
          apiKey: apiKey!,
          model: model!,
          ...(process.env.WUU_V2_OPENAI_BASE_URL
            ? { baseUrl: process.env.WUU_V2_OPENAI_BASE_URL }
            : {}),
        },
      }],
});
const { ctx, modelId } = runtime;

const shutdown = async () => {
  await runtime.dispose();
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
  const recoveredContext = smoke
    ? await ctx.modelContext.build(recoverySessionId!, new AbortController().signal)
    : undefined;
  const sideParentSessionId = `${sessionId}-side-parent`;
  await ctx.sessions.append(sideParentSessionId, {
    pluginId: "harness-smoke",
    generation: "v1",
  }, {
    type: "agent/user-message",
    data: {
      messageId: randomUUID(),
      content: [{ type: "text", text: "Parent context for Side." }],
    },
  } satisfies AgentSessionRecord);
  const sideResolution = await ctx.hostActions.execute("side/resolve", {
    sessionId: sideParentSessionId,
  });
  if (
    !sideResolution ||
    Array.isArray(sideResolution) ||
    typeof sideResolution !== "object" ||
    typeof sideResolution.sessionId !== "string"
  ) {
    throw new Error("side session was not resolved");
  }
  const sideSessionId = sideResolution.sessionId;
  const sideContext = await ctx.modelContext.build(
    sideSessionId,
    new AbortController().signal,
  );
  const sideTools = await ctx.toolPolicy.allowedTools(sideSessionId, ["read", "write"]);
  if (
    await ctx.modelRouting.resolve(sideSessionId) !== modelId ||
    sideContext.messages[0]?.role !== "user" ||
    sideContext.messages[0].content !== "Parent context for Side." ||
    !sideTools.has("read") ||
    sideTools.has("write")
  ) {
    throw new Error("side session did not inherit model with read-only permission");
  }
  const acceptance = await ctx.hostActions.execute("agent/prompt", { sessionId, text: prompt });
  if (!acceptance || Array.isArray(acceptance) || typeof acceptance !== "object") {
    throw new Error("agent prompt was not accepted");
  }
  const runId = acceptance.runId;
  if (typeof runId !== "string") throw new Error("agent prompt acceptance omitted runId");
  const result = await ctx.agentRuns.wait(sessionId, runId);
  const events = await ctx.sessions.load(sessionId);
  const projections = await ctx.projections.build(ctx.sessions, sessionId);
  const projectionFrame = await ctx.projectionFeed.snapshot(sessionId);
  const inspection = ctx.runtimeInspection.snapshot();
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
      recoveredContext?.messages.at(-1)?.role !== "tool" ||
      !recoveryTypes.includes("agent/tool-result") ||
      !recoveryTypes.includes("agent/assistant-completed") ||
      !recordTypes.includes("agent/assistant-tool-call") ||
      !recordTypes.includes("agent/tool-result") ||
      !inspection.services.includes("sessions") ||
      !inspection.tools.includes("read") ||
      inspection.fibers.some(({ pending }) => pending.length) ||
      projectionFrame.lastDurableSeq !== events.at(-1)?.seq ||
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
