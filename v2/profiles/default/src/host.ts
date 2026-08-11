import type { Fiber } from "cordis";
import { agentRuntimePlugin } from "@wuu-v2/plugin-agent-runtime";
import { contextProjectionPlugin } from "@wuu-v2/plugin-context-projection";
import { conversationProjectionPlugin } from "@wuu-v2/plugin-conversation";
import { defaultAgentLoopPlugin } from "@wuu-v2/plugin-default-agent-loop";
import modelSessionHost from "@wuu-v2/plugin-model-session/host";
import permissionSessionHost, {
  type PermissionMode,
} from "@wuu-v2/plugin-permission-session/host";
import { projectionFeedPlugin } from "@wuu-v2/plugin-projection-feed";
import { corePromptPlugin } from "@wuu-v2/plugin-prompt-core";
import {
  openAIProviderPlugin,
  type OpenAIProviderConfig,
} from "@wuu-v2/plugin-provider-openai";
import {
  scriptedProviderPlugin,
  type ScriptedProviderConfig,
} from "@wuu-v2/plugin-provider-scripted";
import { jsonlSessionPlugin } from "@wuu-v2/plugin-session-jsonl";
import sideHost from "@wuu-v2/plugin-side/host";
import { basicToolsPlugin } from "@wuu-v2/plugin-tools-basic";
import { createKernelContext, kernelPlugin, type Context } from "@wuu-v2/kernel";

export type DefaultProfileProvider =
  | { kind: "openai"; config: OpenAIProviderConfig }
  | { kind: "scripted"; config: ScriptedProviderConfig };

export interface DefaultHostProfileConfig {
  cwd: string;
  dataDirectory: string;
  provider: DefaultProfileProvider;
  defaultPermission?: PermissionMode;
}

export interface DefaultHostProfile {
  ctx: Context;
  providerId: string;
  dispose(): Promise<void>;
}

export async function createDefaultHostProfile(
  config: DefaultHostProfileConfig,
): Promise<DefaultHostProfile> {
  const ctx = createKernelContext();
  const fibers: Fiber[] = [];
  const install = async (fiber: Fiber) => {
    fibers.push(fiber);
    await fiber.await();
  };
  let disposed = false;
  const dispose = async () => {
    if (disposed) return;
    disposed = true;
    for (const fiber of [...fibers].reverse()) await fiber.dispose();
    await ctx.fiber.dispose();
  };

  try {
    await install(ctx.plugin(kernelPlugin));
    await install(ctx.plugin(jsonlSessionPlugin, { directory: config.dataDirectory }));
    await install(ctx.plugin(corePromptPlugin, { cwd: config.cwd }));
    await install(ctx.plugin(basicToolsPlugin, { cwd: config.cwd }));
    await install(ctx.plugin(contextProjectionPlugin));
    await install(ctx.plugin(conversationProjectionPlugin));
    await install(ctx.plugin(projectionFeedPlugin));

    let providerId: string;
    if (config.provider.kind === "openai") {
      providerId = config.provider.config.id ?? "openai";
      await install(ctx.plugin(openAIProviderPlugin, config.provider.config));
    } else {
      providerId = "scripted";
      await install(ctx.plugin(scriptedProviderPlugin, config.provider.config));
    }

    await install(ctx.plugin(agentRuntimePlugin, { agentId: "default" }));
    await install(ctx.plugin(modelSessionHost, { defaultProviderId: providerId }));
    await install(ctx.plugin(permissionSessionHost, {
      defaultMode: config.defaultPermission ?? "full-access",
    }));
    await install(ctx.plugin(defaultAgentLoopPlugin, {}));
    await install(ctx.plugin(sideHost, { agentId: "default" }));

    return { ctx, providerId, dispose };
  } catch (error) {
    await dispose();
    throw error;
  }
}
