import { Context, Service, type Plugin } from "cordis";
import type {
  AgentLoopFactory,
  EventSource,
  JsonValue,
  ModelProvider,
  SessionEvent,
  SessionRecord,
  ToolDefinition,
} from "@wuu-v2/contracts";

export interface SessionService {
  append<R extends SessionRecord>(
    sessionId: string,
    source: EventSource,
    record: R,
  ): Promise<SessionEvent<R>>;
  appendBatch<R extends SessionRecord>(
    sessionId: string,
    source: EventSource,
    records: readonly R[],
  ): Promise<Array<SessionEvent<R>>>;
  list(): Promise<string[]>;
  load(sessionId: string): Promise<SessionEvent[]>;
  subscribe(
    sessionId: string,
    listener: (event: SessionEvent) => void,
  ): () => void;
}

export interface ModelContextService {
  build(sessionId: string, signal: AbortSignal): Promise<{
    messages: import("@wuu-v2/contracts").ModelMessage[];
    tools: import("@wuu-v2/contracts").ModelTool[];
    systemPrompt: string;
    generation: string;
    sources: string[];
  }>;
}

export type HostActionHandler = (
  input: JsonValue,
) => Promise<JsonValue | undefined> | JsonValue | undefined;

export interface ProjectionSnapshot<T extends JsonValue = JsonValue> {
  key: string;
  seq: number;
  value: T | undefined;
}

export type ProjectionFold<T extends JsonValue = JsonValue> = (
  current: T | undefined,
  event: SessionEvent,
) => T | undefined;

class UniqueRegistry<T> extends Service {
  private readonly values = new Map<string, T>();

  register(id: string, value: T): () => void {
    if (this.values.has(id)) {
      throw new Error(`duplicate registration: ${id}`);
    }
    this.values.set(id, value);
    return this.ctx.effect(() => () => {
      this.values.delete(id);
    }, `unregister ${this.name}:${id}`);
  }

  get(id: string): T | undefined {
    return this.values.get(id);
  }

  require(id: string): T {
    const value = this.values.get(id);
    if (!value) throw new Error(`missing registration: ${id}`);
    return value;
  }

  entries(): ReadonlyArray<readonly [string, T]> {
    return [...this.values.entries()];
  }
}

export class ToolRegistry extends UniqueRegistry<ToolDefinition> {
  constructor(ctx: Context) {
    super(ctx, "tools");
  }
}

export class ProviderRegistry extends UniqueRegistry<ModelProvider> {
  constructor(ctx: Context) {
    super(ctx, "providers");
  }
}

export class AgentRegistry extends UniqueRegistry<AgentLoopFactory> {
  constructor(ctx: Context) {
    super(ctx, "agents");
  }
}

export class HostActionRegistry extends UniqueRegistry<HostActionHandler> {
  constructor(ctx: Context) {
    super(ctx, "hostActions");
  }

  async execute(action: string, input: JsonValue): Promise<JsonValue | undefined> {
    return this.require(action)(input);
  }
}

export class PromptRegistry extends UniqueRegistry<() => string> {
  constructor(ctx: Context) {
    super(ctx, "prompts");
  }

  render(): { text: string; sources: string[] } {
    const entries = this.entries();
    return {
      text: entries.map(([, render]) => render()).filter(Boolean).join("\n\n"),
      sources: entries.map(([id]) => id),
    };
  }
}

export class ProjectionRegistry extends Service {
  private readonly folds = new Map<string, ProjectionFold>();

  constructor(ctx: Context) {
    super(ctx, "projections");
  }

  register(key: string, fold: ProjectionFold): () => void {
    if (this.folds.has(key)) throw new Error(`duplicate projection: ${key}`);
    this.folds.set(key, fold);
    return this.ctx.effect(() => () => {
      this.folds.delete(key);
    }, `unregister projection:${key}`);
  }

  async build(
    sessions: SessionService,
    sessionId: string,
  ): Promise<ProjectionSnapshot[]> {
    const events = await sessions.load(sessionId);
    return [...this.folds.entries()].map(([key, fold]) => {
      let value: JsonValue | undefined;
      for (const event of events) value = fold(value, event);
      return { key, seq: events.at(-1)?.seq ?? 0, value };
    });
  }
}

declare module "cordis" {
  interface Context {
    agents: AgentRegistry;
    hostActions: HostActionRegistry;
    modelContext: ModelContextService;
    projections: ProjectionRegistry;
    prompts: PromptRegistry;
    providers: ProviderRegistry;
    sessions: SessionService;
    tools: ToolRegistry;
  }
}

export const kernelPlugin: Plugin = function kernel(ctx: Context) {
  new AgentRegistry(ctx);
  new HostActionRegistry(ctx);
  new ProjectionRegistry(ctx);
  new PromptRegistry(ctx);
  new ProviderRegistry(ctx);
  new ToolRegistry(ctx);
};

kernelPlugin.provide = ["agents", "hostActions", "projections", "prompts", "providers", "tools"];

export function createKernelContext(): Context {
  return new Context();
}

export { Context, Service } from "cordis";
export type { Plugin } from "cordis";
