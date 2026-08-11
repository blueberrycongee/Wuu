import {
  createElement,
  type ComponentType,
  type ReactNode,
  useSyncExternalStore,
} from "react";
import { Context, Service, type Fiber, type Plugin } from "cordis";
import type { JsonValue } from "@wuu-v2/contracts";

export type SlotKind = "single" | "list" | "keyed" | "chain";
export type SlotScope = "root" | "session-maybe" | "session";

export interface SlotDeclaration {
  name: string;
  parent?: string;
  kind: SlotKind;
  scope: SlotScope;
}

export interface SlotComponentProps {
  client: Context;
  ownerProps?: unknown;
  sessionId?: string;
}

export interface SlotContribution {
  id: string;
  order?: number;
  component: ComponentType<SlotComponentProps>;
  children?: Array<Omit<SlotDeclaration, "parent">>;
}

interface OwnedDeclaration extends SlotDeclaration {
  epoch: symbol;
}

export interface SlotHandle {
  readonly name: string;
  readonly epoch: symbol;
}

export interface SlotRegistration {
  readonly children: ReadonlyMap<string, SlotHandle>;
  dispose(): void | Promise<void>;
}

export class SlotsService extends Service {
  private readonly declarations = new Map<string, OwnedDeclaration>();
  private readonly contributions = new Map<string, Map<string, SlotContribution>>();
  private readonly listeners = new Set<() => void>();
  private revision = 0;
  readonly root: SlotHandle;

  constructor(ctx: Context) {
    super(ctx, "slots");
    this.root = { name: "root", epoch: Symbol("root") };
    this.declarations.set("root", {
      name: "root",
      kind: "single",
      scope: "root",
      epoch: this.root.epoch,
    });
  }

  private changed(): void {
    this.revision += 1;
    for (const listener of this.listeners) listener();
  }

  subscribe(listener: () => void): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  snapshot = (): number => this.revision;

  private declare(declaration: SlotDeclaration): SlotHandle {
    if (this.declarations.has(declaration.name)) {
      throw new Error(`duplicate slot declaration: ${declaration.name}`);
    }
    const existing = this.contributions.get(declaration.name);
    if (declaration.kind === "single" && (existing?.size ?? 0) > 1) {
      throw new Error(`single slot has multiple pending contributions: ${declaration.name}`);
    }
    const owned = { ...declaration, epoch: Symbol(declaration.name) };
    this.declarations.set(declaration.name, owned);
    this.changed();
    return { name: declaration.name, epoch: owned.epoch };
  }

  private releaseDeclaration(name: string, epoch: symbol): void {
    const current = this.declarations.get(name);
    if (!current || current.epoch !== epoch) return;
    for (const child of [...this.declarations.values()]) {
      if (child.parent === name) this.releaseDeclaration(child.name, child.epoch);
    }
    this.declarations.delete(name);
    this.contributions.delete(name);
    this.changed();
  }

  contribute(name: string, contribution: SlotContribution): SlotRegistration {
    const declaration = this.declarations.get(name);
    const entries = this.contributions.get(name) ?? new Map();
    if (entries.has(contribution.id)) {
      throw new Error(`duplicate slot contribution: ${name}/${contribution.id}`);
    }
    if (declaration?.kind === "single" && entries.size) {
      throw new Error(`single slot is already occupied: ${name}`);
    }
    entries.set(contribution.id, contribution);
    this.contributions.set(name, entries);
    const children = new Map<string, SlotHandle>();
    try {
      for (const child of contribution.children ?? []) {
        const handle = this.declare({ ...child, parent: name });
        children.set(child.name, handle);
      }
    } catch (error) {
      for (const handle of [...children.values()].reverse()) {
        this.releaseDeclaration(handle.name, handle.epoch);
      }
      entries.delete(contribution.id);
      if (!entries.size) this.contributions.delete(name);
      throw error;
    }
    this.changed();
    const dispose = this.ctx.effect(() => () => {
      for (const handle of [...children.values()].reverse()) {
        this.releaseDeclaration(handle.name, handle.epoch);
      }
      if (entries.delete(contribution.id)) {
        if (!entries.size) this.contributions.delete(name);
        this.changed();
      }
    }, `remove slot contribution:${name}/${contribution.id}`);
    return { children, dispose };
  }

  entries(handle: SlotHandle): SlotContribution[] {
    const declaration = this.declarations.get(handle.name);
    if (!declaration || declaration.epoch !== handle.epoch) {
      throw new Error(`stale slot authorization: ${handle.name}`);
    }
    return [...(this.contributions.get(handle.name)?.values() ?? [])]
      .sort((left, right) => (left.order ?? 0) - (right.order ?? 0) || left.id.localeCompare(right.id));
  }
}

interface ProjectionRow {
  seq: number;
  value: JsonValue | undefined;
}

export class ClientProjectionStore extends Service {
  private readonly rows = new Map<string, ProjectionRow>();
  private readonly listeners = new Set<() => void>();
  private revision = 0;

  constructor(ctx: Context) {
    super(ctx, "clientProjections");
  }

  private rowKey(sessionId: string, key: string): string {
    return `${sessionId}\u0000${key}`;
  }

  apply(sessionId: string, key: string, seq: number, value: JsonValue | undefined): void {
    const rowKey = this.rowKey(sessionId, key);
    if ((this.rows.get(rowKey)?.seq ?? -1) >= seq) return;
    this.rows.set(rowKey, { seq, value });
    this.revision += 1;
    for (const listener of this.listeners) listener();
  }

  truncate(sessionId: string, lastDurableSeq: number): void {
    let changed = false;
    for (const [key, row] of this.rows) {
      if (!key.startsWith(`${sessionId}\u0000`) || row.seq <= lastDurableSeq) continue;
      this.rows.delete(key);
      changed = true;
    }
    if (!changed) return;
    this.revision += 1;
    for (const listener of this.listeners) listener();
  }

  get(sessionId: string, key: string): ProjectionRow | undefined {
    return this.rows.get(this.rowKey(sessionId, key));
  }

  subscribe(listener: () => void): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  snapshot = (): number => this.revision;
}

export type ClientActionHandler = (
  action: string,
  input: JsonValue,
) => Promise<JsonValue | undefined>;

export class ClientActionsService extends Service {
  private handler: ClientActionHandler | undefined;

  constructor(ctx: Context) {
    super(ctx, "clientActions");
  }

  connect(handler: ClientActionHandler): () => void {
    if (this.handler) throw new Error("client action bridge is already connected");
    this.handler = handler;
    return this.ctx.effect(() => () => {
      if (this.handler === handler) this.handler = undefined;
    }, "disconnect client action bridge");
  }

  execute(action: string, input: JsonValue): Promise<JsonValue | undefined> {
    if (!this.handler) throw new Error("client action bridge is not connected");
    return this.handler(action, input);
  }
}

export interface ClientPluginModule {
  default: Plugin;
}

export type ClientModuleFactory = () => Promise<ClientPluginModule>;

export class ClientModuleSystem {
  private readonly arrivals = new Map<string, { revision: string; factory: ClientModuleFactory }>();
  private readonly modules = new Map<string, ClientPluginModule>();
  private readonly materializing = new Map<string, Promise<ClientPluginModule>>();
  private readonly fibers = new Map<string, Fiber>();

  constructor(private readonly ctx: Context) {}

  arrive(id: string, revision: string, factory: ClientModuleFactory): void {
    if (this.arrivals.has(id)) throw new Error(`client module already arrived: ${id}`);
    this.arrivals.set(id, { revision, factory });
  }

  async materialize(id: string): Promise<ClientPluginModule> {
    const cached = this.modules.get(id);
    if (cached) return cached;
    const inFlight = this.materializing.get(id);
    if (inFlight) return inFlight;
    const arrival = this.arrivals.get(id);
    if (!arrival) throw new Error(`client module has not arrived: ${id}`);
    const task = arrival.factory().then((module) => {
      this.modules.set(id, module);
      return module;
    });
    this.materializing.set(id, task);
    try {
      return await task;
    } finally {
      if (this.materializing.get(id) === task) this.materializing.delete(id);
    }
  }

  async activate(id: string): Promise<void> {
    await this.activateAll([id]);
  }

  async activateAll(ids: string[]): Promise<void> {
    const pending: Array<{ id: string; fiber: Fiber }> = [];
    for (const id of ids) {
      if (this.fibers.has(id)) continue;
      const module = await this.materialize(id);
      if (this.fibers.has(id)) continue;
      const fiber = this.ctx.plugin(module.default);
      this.fibers.set(id, fiber);
      pending.push({ id, fiber });
    }
    try {
      await Promise.all(pending.map(({ fiber }) => fiber.await()));
    } catch (error) {
      for (const { id, fiber } of pending.reverse()) {
        await fiber.dispose();
        this.fibers.delete(id);
        this.modules.delete(id);
      }
      throw error;
    }
  }

  async invalidate(id: string): Promise<void> {
    try {
      await this.materializing.get(id);
    } catch {
      // A failed candidate still needs its arrival state invalidated.
    }
    await this.fibers.get(id)?.dispose();
    this.fibers.delete(id);
    this.modules.delete(id);
    this.arrivals.delete(id);
  }

  async dispose(): Promise<void> {
    for (const id of [...this.arrivals.keys()].reverse()) await this.invalidate(id);
  }
}

declare module "cordis" {
  interface Context {
    clientActions: ClientActionsService;
    clientProjections: ClientProjectionStore;
    slots: SlotsService;
  }
}

export const clientKernelPlugin: Plugin = function clientKernel(ctx: Context) {
  new ClientActionsService(ctx);
  new ClientProjectionStore(ctx);
  new SlotsService(ctx);
};

clientKernelPlugin.provide = ["clientActions", "clientProjections", "slots"];

export function SlotOutlet(props: {
  client: Context;
  slot: SlotHandle;
  ownerProps?: unknown;
  sessionId?: string;
}): ReactNode {
  useSyncExternalStore(
    props.client.slots.subscribe.bind(props.client.slots),
    props.client.slots.snapshot,
    props.client.slots.snapshot,
  );
  return props.client.slots.entries(props.slot).map((entry) =>
    createElement(entry.component, {
      key: entry.id,
      client: props.client,
      ...(props.ownerProps === undefined ? {} : { ownerProps: props.ownerProps }),
      ...(props.sessionId === undefined ? {} : { sessionId: props.sessionId }),
    }),
  );
}

export function useProjection<T extends JsonValue>(
  client: Context,
  sessionId: string,
  key: string,
): T | undefined {
  useSyncExternalStore(
    client.clientProjections.subscribe.bind(client.clientProjections),
    client.clientProjections.snapshot,
    client.clientProjections.snapshot,
  );
  return client.clientProjections.get(sessionId, key)?.value as T | undefined;
}

export { Context, Service } from "cordis";
export type { Plugin } from "cordis";
