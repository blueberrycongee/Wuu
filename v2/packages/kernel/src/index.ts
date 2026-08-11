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
  snapshot(sessionId: string, signal: AbortSignal): Promise<{
    messages: import("@wuu-v2/contracts").ModelMessage[];
    sourceSeq: number;
  }>;
}

export interface ModelRoutingService {
  resolve(sessionId: string): Promise<string>;
  initialize(sessionId: string, sourceSessionId?: string): Promise<void>;
}

export interface ToolPolicyService {
  allowedTools(sessionId: string, available: readonly string[]): Promise<ReadonlySet<string>>;
  initialize(sessionId: string, preset: string): Promise<void>;
}

export interface RuntimeFiberSnapshot {
  uid: number | null;
  name: string;
  state: "pending" | "loading" | "active" | "failed" | "disposed" | "unloading";
  inject: string[];
  pending: string[];
  provides: string[];
  effects: string[];
}

export interface RuntimeInspectionSnapshot {
  services: string[];
  tools: string[];
  providers: string[];
  agents: string[];
  prompts: string[];
  projections: string[];
  hostActions: string[];
  fibers: RuntimeFiberSnapshot[];
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

interface ProjectionUnit {
  fold: ProjectionFold;
  initial?: () => JsonValue | undefined;
}

class UniqueRegistry<T> extends Service {
  private readonly values = new Map<string, T>();
  private readonly listeners = new Set<() => void>();

  private changed(): void {
    for (const listener of this.listeners) {
      try {
        listener();
      } catch (error) {
        this.ctx.logger.error(error);
      }
    }
  }

  register(id: string, value: T): () => void {
    if (this.values.has(id)) {
      throw new Error(`duplicate registration: ${id}`);
    }
    this.values.set(id, value);
    this.changed();
    return this.ctx.effect(() => () => {
      if (this.values.delete(id)) this.changed();
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

  subscribe(listener: () => void): () => void {
    this.listeners.add(listener);
    return this.ctx.effect(() => () => {
      this.listeners.delete(listener);
    }, `unsubscribe ${this.name}`);
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

export type PromptRenderer = (
  sessionId: string,
) => string | undefined | Promise<string | undefined>;

export class PromptRegistry extends UniqueRegistry<PromptRenderer> {
  constructor(ctx: Context) {
    super(ctx, "prompts");
  }

  async render(sessionId: string): Promise<{ text: string; sources: string[] }> {
    const entries = [...this.entries()].sort(([left], [right]) => left.localeCompare(right));
    const sections: Array<{ id: string; text: string }> = [];
    for (const [id, render] of entries) {
      const text = (await render(sessionId))?.trim();
      if (text) sections.push({ id, text });
    }
    return {
      text: sections.map(({ text }) => text).join("\n\n"),
      sources: sections.map(({ id }) => id),
    };
  }
}

export class ProjectionRegistry extends Service {
  private readonly folds = new Map<string, ProjectionUnit>();
  private readonly listeners = new Set<() => void>();

  constructor(ctx: Context) {
    super(ctx, "projections");
  }

  register(
    key: string,
    fold: ProjectionFold,
    initial?: () => JsonValue | undefined,
  ): () => void {
    if (this.folds.has(key)) throw new Error(`duplicate projection: ${key}`);
    this.folds.set(key, { fold, ...(initial ? { initial } : {}) });
    this.invalidate();
    return this.ctx.effect(() => () => {
      if (this.folds.delete(key)) this.invalidate();
    }, `unregister projection:${key}`);
  }

  async build(
    sessions: SessionService,
    sessionId: string,
  ): Promise<ProjectionSnapshot[]> {
    return this.buildEvents(await sessions.load(sessionId));
  }

  buildEvents(events: readonly SessionEvent[]): ProjectionSnapshot[] {
    return [...this.folds.entries()].map(([key, unit]) => {
      let value = unit.initial?.();
      for (const event of events) value = unit.fold(value, event);
      return { key, seq: events.at(-1)?.seq ?? 0, value };
    });
  }

  keys(): string[] {
    return [...this.folds.keys()];
  }

  invalidate(): void {
    for (const listener of this.listeners) {
      try {
        listener();
      } catch (error) {
        this.ctx.logger.error(error);
      }
    }
  }

  subscribe(listener: () => void): () => void {
    this.listeners.add(listener);
    return this.ctx.effect(() => () => {
      this.listeners.delete(listener);
    }, "unsubscribe projections");
  }
}

const fiberStateNames: RuntimeFiberSnapshot["state"][] = [
  "pending",
  "loading",
  "active",
  "failed",
  "disposed",
  "unloading",
];

function fiberStateName(state: number): RuntimeFiberSnapshot["state"] {
  const name = fiberStateNames[state];
  if (!name) throw new Error(`unknown Fiber state: ${state}`);
  return name;
}

export class RuntimeInspectionService extends Service {
  constructor(ctx: Context) {
    super(ctx, "runtimeInspection");
  }

  snapshot(): RuntimeInspectionSnapshot {
    const serviceNames = Object.entries(this.ctx.reflect.props)
      .filter(([name, property]) =>
        property.type === "service" && this.ctx.get(name) !== undefined)
      .map(([name]) => name)
      .sort();
    const fibers = [...this.ctx.registry.values()]
      .flatMap((runtime) => [...runtime.fibers])
      .map((fiber): RuntimeFiberSnapshot => {
        const inject = Object.keys(fiber.inject).sort();
        return {
          uid: fiber.uid,
          name: fiber.name,
          state: fiberStateName(fiber.state),
          inject,
          pending: inject.filter((name) => fiber.ctx.get(name) === undefined),
          provides: Object.keys(fiber.store ?? {}).sort(),
          effects: fiber.getEffects().map(({ label }) => label),
        };
      })
      .sort((left, right) =>
        (left.uid ?? Number.MAX_SAFE_INTEGER) - (right.uid ?? Number.MAX_SAFE_INTEGER));
    const keys = <T>(registry: { entries(): ReadonlyArray<readonly [string, T]> }) =>
      registry.entries().map(([id]) => id).sort();
    return {
      services: serviceNames,
      tools: keys(this.ctx.tools),
      providers: keys(this.ctx.providers),
      agents: keys(this.ctx.agents),
      prompts: keys(this.ctx.prompts),
      projections: this.ctx.projections.keys().sort(),
      hostActions: keys(this.ctx.hostActions),
      fibers,
    };
  }

  assertReady(): void {
    const stalled = this.snapshot().fibers.filter(
      ({ state, pending }) => state === "pending" && pending.length,
    );
    if (!stalled.length) return;
    throw new Error(`host startup dependency audit failed; ${stalled
      .map(({ name, pending }) => `${name} -> ${pending.join(",")}`)
      .join("; ")}`);
  }
}

declare module "cordis" {
  interface Context {
    agents: AgentRegistry;
    hostActions: HostActionRegistry;
    modelContext: ModelContextService;
    modelRouting: ModelRoutingService;
    projections: ProjectionRegistry;
    prompts: PromptRegistry;
    providers: ProviderRegistry;
    runtimeInspection: RuntimeInspectionService;
    sessions: SessionService;
    tools: ToolRegistry;
    toolPolicy: ToolPolicyService;
  }
}

export const kernelPlugin: Plugin = function kernel(ctx: Context) {
  new AgentRegistry(ctx);
  new HostActionRegistry(ctx);
  new ProjectionRegistry(ctx);
  new PromptRegistry(ctx);
  new ProviderRegistry(ctx);
  new RuntimeInspectionService(ctx);
  new ToolRegistry(ctx);
};

kernelPlugin.provide = [
  "agents",
  "hostActions",
  "projections",
  "prompts",
  "providers",
  "runtimeInspection",
  "tools",
];

export function createKernelContext(): Context {
  return new Context();
}

export { Context, Service } from "cordis";
export type { Plugin } from "cordis";
