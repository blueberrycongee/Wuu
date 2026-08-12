import { randomUUID } from "node:crypto";
import { mkdir, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import type { AgentSessionRecord } from "@wuu-v2/contracts";
import { createDefaultHostProfile } from "@wuu-v2/profile-default/host";
import { providersFromEnvironment } from "@wuu-v2/profile-default/provider-environment";

const smoke = process.argv.includes("--smoke");
const cwd = process.cwd();
const sessionId = process.env.WUU_V2_SESSION_ID ?? `session-${randomUUID()}`;
const directory = smoke
  ? join(tmpdir(), `wuu-v2-smoke-${process.pid}`)
  : process.env.WUU_V2_DATA_DIR ?? join(cwd, ".wuu-v2", "sessions");
const workspacePluginDirectory = smoke
  ? join(directory, "workspace-plugins")
  : join(cwd, ".wuu-v2", "plugins");
const prompt = smoke
  ? "Read package.json and then confirm the smoke run completed."
  : process.argv.slice(2).filter((arg) => !arg.startsWith("--")).join(" ").trim();

if (!prompt) {
  throw new Error("Pass a prompt or use --smoke");
}

if (smoke) {
  await mkdir(join(workspacePluginDirectory, "smoke"), { recursive: true });
  await writeFile(join(workspacePluginDirectory, "smoke", "index.ts"), `
export default function plugin(ctx: any) {
  ctx.tools.register("workspace_smoke", {
    name: "workspace_smoke",
    description: "Workspace loader smoke tool.",
    access: "read",
    inputSchema: { type: "object", properties: {}, additionalProperties: false },
    async execute() {
      return { content: [{ type: "text", text: "workspace-v1" }] };
    },
  });
}
plugin.inject = ["tools"];
`, "utf8");
}
const runtime = await createDefaultHostProfile({
  cwd,
  dataDirectory: directory,
  workspacePluginDirectory,
  providers: smoke
    ? [{
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
      }]
    : providersFromEnvironment(process.env),
  ...(process.env.WUU_V2_DEFAULT_MODEL
    ? { defaultModelId: process.env.WUU_V2_DEFAULT_MODEL }
    : {}),
});
const { ctx, modelId } = runtime;

const smokeWorkspaceLoader = async () => {
  if (!smoke) return;
  const executeWorkspaceSmoke = async () => {
    const result = await ctx.tools.require("workspace_smoke").execute({}, {
      sessionId,
      callId: randomUUID(),
      signal: new AbortController().signal,
    });
    return result.content.map((item) => item.text).join("\n");
  };
  if (await executeWorkspaceSmoke() !== "workspace-v1") {
    throw new Error("workspace plugin was not discovered");
  }
  const runtimeInspection = await ctx.hostActions.execute("workspace-plugin/inspect", {});
  if (
    !runtimeInspection ||
    Array.isArray(runtimeInspection) ||
    typeof runtimeInspection !== "object" ||
    !Array.isArray(runtimeInspection.tools) ||
    !runtimeInspection.tools.includes("workspace_smoke")
  ) {
    throw new Error("workspace runtime inspection omitted the discovered Tool");
  }
  await writeFile(join(workspacePluginDirectory, "smoke", "index.ts"), `
function plugin(ctx: any) {
  ctx.tools.register("workspace_smoke", {
    name: "workspace_smoke",
    description: "Workspace loader smoke tool.",
    access: "read",
    inputSchema: { type: "object", properties: {}, additionalProperties: false },
    async execute() {
      return { content: [{ type: "text", text: "workspace-v2" }] };
    },
  });
}
plugin.inject = ["tools"];
export default plugin;
`, "utf8");
  await ctx.hostActions.execute("workspace-plugin/load", { id: "smoke" });
  if (await executeWorkspaceSmoke() !== "workspace-v2") {
    throw new Error("workspace plugin did not reload");
  }
  await writeFile(join(workspacePluginDirectory, "smoke", "index.ts"), `
function plugin(ctx: any) {
  ctx.tools.register("read", {
    name: "read",
    description: "Invalid duplicate tool.",
    access: "read",
    inputSchema: { type: "object" },
    async execute() { return { content: [] }; },
  });
}
plugin.inject = ["tools"];
export default plugin;
`, "utf8");
  let rejected = false;
  try {
    await ctx.hostActions.execute("workspace-plugin/load", { id: "smoke" });
  } catch {
    rejected = true;
  }
  if (!rejected || await executeWorkspaceSmoke() !== "workspace-v2") {
    throw new Error("workspace plugin rollback did not preserve the previous generation");
  }
  await ctx.hostActions.execute("workspace-plugin/unload", { id: "smoke" });
  if (ctx.tools.get("workspace_smoke")) throw new Error("workspace plugin did not unload");
};

const shutdown = async () => {
  await runtime.dispose();
};
process.once("SIGINT", () => void shutdown().finally(() => process.exit(130)));
process.once("SIGTERM", () => void shutdown().finally(() => process.exit(143)));

try {
  await smokeWorkspaceLoader();
  let recoverySessionId: string | undefined;
  let recoveryRunId: string | undefined;
  let terminalSessionId: string | undefined;
  let terminalRunId: string | undefined;
  if (smoke) {
    recoverySessionId = `${sessionId}-recovery`;
    recoveryRunId = randomUUID();
    await ctx.sessions.appendBatch(recoverySessionId, {
      pluginId: "harness-smoke",
      generation: "v1",
    }, [
      {
        type: "agent/user-message",
        data: {
          messageId: "interrupted-user",
          content: [{ type: "text", text: "Keep this committed user message." }],
        },
      },
      {
        type: "agent/run-state",
        data: { runId: recoveryRunId, state: "started" },
      },
      {
        type: "agent/assistant-started",
        data: { messageId: "interrupted-message" },
      },
      {
        type: "agent/assistant-text-delta",
        data: { messageId: "interrupted-message", delta: "Do not reuse this partial response." },
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
    terminalSessionId = `${sessionId}-terminal`;
    terminalRunId = randomUUID();
    await ctx.sessions.appendBatch(terminalSessionId, {
      pluginId: "harness-smoke",
      generation: "v1",
    }, [
      { type: "agent/run-state", data: { runId: "old-failure", state: "started" } },
      { type: "agent/assistant-started", data: { messageId: "old-failure-message" } },
      {
        type: "agent/assistant-completed",
        data: { messageId: "old-failure-message", stopReason: "error" },
      },
      { type: "agent/run-state", data: { runId: "old-failure", state: "failed" } },
      { type: "agent/run-state", data: { runId: terminalRunId, state: "started" } },
      {
        type: "agent/run-state",
        data: { runId: terminalRunId, state: "failed", error: "Current run failed before response" },
      },
    ] satisfies AgentSessionRecord[]);
  }

  const recovered = await ctx.agentRuns.recoverAll();
  await runtime.openSession(sessionId);
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
    const history = await ctx.hostActions.execute("history/list", {});
    const historySessions = history && !Array.isArray(history) && typeof history === "object"
      ? history.sessions
      : undefined;
    const recoveryEvents = await ctx.sessions.load(recoverySessionId!);
    const recoveryTypes = recoveryEvents.map((event) => event.record.type);
    const recoveryState = recoveryEvents
      .map((event) => event.record as AgentSessionRecord)
      .findLast((record) => record.type === "agent/run-state");
    const recoveryConversation = (await ctx.projections.build(ctx.sessions, recoverySessionId!))
      .find(({ key }) => key === "conversation")?.value;
    const recoveryItems = recoveryConversation &&
      !Array.isArray(recoveryConversation) &&
      typeof recoveryConversation === "object" &&
      Array.isArray(recoveryConversation.items)
      ? recoveryConversation.items
      : [];
    const terminalConversation = (await ctx.projections.build(ctx.sessions, terminalSessionId!))
      .find(({ key }) => key === "conversation")?.value;
    const terminalItems = terminalConversation &&
      !Array.isArray(terminalConversation) &&
      typeof terminalConversation === "object" &&
      Array.isArray(terminalConversation.items)
      ? terminalConversation.items
      : [];
    if (
      result.status !== "completed" ||
      !recovered.includes(recoveryRunId!) ||
      recoveryState?.type !== "agent/run-state" ||
      recoveryState.data.state !== "interrupted" ||
      !recoveryItems.some((item) =>
        item &&
        !Array.isArray(item) &&
        typeof item === "object" &&
        item.kind === "message" &&
        item.id === "interrupted-message" &&
        item.status === "interrupted") ||
      !recoveryItems.some((item) =>
        item &&
        !Array.isArray(item) &&
        typeof item === "object" &&
        item.kind === "tool" &&
        item.status === "interrupted") ||
      !terminalItems.some((item) =>
        item &&
        !Array.isArray(item) &&
        typeof item === "object" &&
        item.id === `run:${terminalRunId}:failed` &&
        item.status === "failed") ||
      recoveredContext?.messages.length !== 1 ||
      recoveredContext.messages[0]?.role !== "user" ||
      recoveredContext.messages[0].content !== "Keep this committed user message." ||
      recoveryTypes.includes("agent/tool-result") ||
      recoveryTypes.includes("agent/assistant-completed") ||
      !recordTypes.includes("agent/assistant-tool-call") ||
      !recordTypes.includes("agent/tool-result") ||
      !Array.isArray(historySessions) ||
      historySessions.length !== 1 ||
      !historySessions[0] ||
      typeof historySessions[0] !== "object" ||
      Array.isArray(historySessions[0]) ||
      historySessions[0].id !== sessionId ||
      !inspection.services.includes("sessions") ||
      !inspection.tools.includes("read") ||
      inspection.fibers.some(({ pending }) => pending.length) ||
      projectionFrame.lastDurableSeq !== events.at(-1)?.seq ||
      !projections.some(({ key }) => key === "conversation") ||
      !projections.some(({ key, value }) =>
        key === "history/entry" &&
        value &&
        !Array.isArray(value) &&
        typeof value === "object" &&
        value.title === prompt &&
        value.running === false)
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
