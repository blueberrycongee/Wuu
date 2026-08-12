import { Service, type Context, type Plugin } from "@wuu-v2/kernel";

class DefaultToolResultProjectionService extends Service {
  readonly generation = "identity-v1";

  constructor(ctx: Context) {
    super(ctx, "toolResultProjection");
  }

  async project(
    _sessionId: string,
    result: Parameters<Context["toolResultProjection"]["project"]>[1],
    signal: AbortSignal,
  ) {
    signal.throwIfAborted();
    return {
      content: result.content.map((item) => ({ ...item })),
      isError: result.isError,
    };
  }
}

export const toolResultProjectionPlugin: Plugin = function toolResultProjection(
  ctx: Context,
) {
  new DefaultToolResultProjectionService(ctx);
};

toolResultProjectionPlugin.provide = "toolResultProjection";

export default toolResultProjectionPlugin;
