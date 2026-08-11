import { type Context, type Plugin } from "@wuu-v2/kernel";

export interface CorePromptConfig {
  cwd: string;
}

export const corePromptPlugin: Plugin<CorePromptConfig> = function promptCore(
  ctx: Context,
  config: CorePromptConfig,
) {
  ctx.prompts.register("core", () => [
    "You are Wuu, a coding agent working directly in the user's workspace.",
    `Workspace: ${config.cwd}`,
    "Use tools when they are needed, preserve unrelated work, and report concrete outcomes.",
  ].join("\n"));
};

corePromptPlugin.inject = ["prompts"];
