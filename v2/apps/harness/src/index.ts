import { randomUUID } from "node:crypto";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { contextProjectionPlugin } from "@wuu-v2/plugin-context-projection";
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

const shutdown = async () => {
  for (const fiber of fibers.reverse()) await fiber.dispose();
  await ctx.fiber.dispose();
};
process.once("SIGINT", () => void shutdown().finally(() => process.exit(130)));
process.once("SIGTERM", () => void shutdown().finally(() => process.exit(143)));

try {
  const result = await ctx.agents.require("default")().run({ sessionId, text: prompt });
  const events = await ctx.sessions.load(sessionId);
  const recordTypes = events.map((event) => event.record.type);
  if (smoke) {
    if (
      result.status !== "completed" ||
      !recordTypes.includes("agent/assistant-tool-call") ||
      !recordTypes.includes("agent/tool-result")
    ) {
      throw new Error("smoke run did not complete the model-tool-result loop");
    }
  }
  console.log(JSON.stringify({
    runtime: "wuu-v2",
    sessionId,
    status: result.status,
    lastSeq: events.at(-1)?.seq ?? 0,
    recordTypes,
  }));
} finally {
  await shutdown();
}
