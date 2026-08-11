import type { EventSource, JsonValue, SessionRecord } from "@wuu-v2/contracts";
import {
  Service,
  type Context,
  type ModelRoutingService,
  type Plugin,
} from "@wuu-v2/kernel";

export interface ModelSessionConfig {
  defaultModelId: string;
}

type ModelSelectedRecord = SessionRecord<"model/selected", { modelId: string }>;
const source: EventSource = { pluginId: "model-session", generation: "v1" };

function objectInput(input: JsonValue): Record<string, JsonValue> {
  if (!input || typeof input !== "object" || Array.isArray(input)) {
    throw new Error("action input must be an object");
  }
  return input;
}

function stringField(input: Record<string, JsonValue>, field: string): string {
  const value = input[field];
  if (typeof value !== "string" || !value) throw new Error(`missing string field: ${field}`);
  return value;
}

export class ModelSessionService extends Service implements ModelRoutingService {
  constructor(ctx: Context, private readonly defaultModelId: string) {
    super(ctx, "modelRouting");
    ctx.providers.require(defaultModelId);
    ctx.projections.register("model", (current, event) => {
      if (event.record.type !== "model/selected") return current;
      const record = event.record as ModelSelectedRecord;
      return this.projection(record.data.modelId);
    }, () => this.projection(defaultModelId));
    ctx.providers.subscribe(() => ctx.projections.invalidate());
    ctx.hostActions.register("model/select", async (input) => {
      const value = objectInput(input);
      const sessionId = stringField(value, "sessionId");
      const modelId = stringField(value, "modelId");
      if (ctx.agentRuns.isActive(sessionId)) throw new Error("cannot change model during an active run");
      ctx.providers.require(modelId);
      const event = await ctx.sessions.append(sessionId, source, {
        type: "model/selected",
        data: { modelId },
      } satisfies ModelSelectedRecord);
      return { modelId, acceptedSeq: event.seq };
    });
  }

  private projection(modelId: string): JsonValue {
    return {
      selected: modelId,
      options: this.ctx.providers.entries()
        .map(([id, provider]) => ({ id, label: provider.displayName ?? id }))
        .sort((left, right) => left.id.localeCompare(right.id)),
    };
  }

  async resolve(sessionId: string): Promise<string> {
    let selected = this.defaultModelId;
    for (const event of await this.ctx.sessions.load(sessionId)) {
      if (event.record.type === "model/selected") {
        selected = (event.record as ModelSelectedRecord).data.modelId;
      }
    }
    this.ctx.providers.require(selected);
    return selected;
  }

  async initialize(sessionId: string, sourceSessionId?: string): Promise<void> {
    const target = await this.ctx.sessions.load(sessionId);
    if (target.some((event) => event.record.type === "model/selected")) return;
    await this.ctx.sessions.append(sessionId, source, {
      type: "model/selected",
      data: {
        modelId: sourceSessionId
          ? await this.resolve(sourceSessionId)
          : this.defaultModelId,
      },
    } satisfies ModelSelectedRecord);
  }
}

const modelSessionHost: Plugin<ModelSessionConfig> = function modelSession(ctx, config) {
  new ModelSessionService(ctx, config.defaultModelId);
};

modelSessionHost.inject = ["agentRuns", "hostActions", "projections", "providers", "sessions"];
modelSessionHost.provide = "modelRouting";
export default modelSessionHost;
