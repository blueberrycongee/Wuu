import {
  createElement,
  type ComponentType,
  type ReactNode,
  useCallback,
  useEffect,
  useSyncExternalStore,
} from "react";
import { Context, Service, type Fiber, type Plugin } from "cordis";
import type { JsonValue, ProjectionFrame } from "@wuu-v2/contracts";

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
  businessKey?: string;
  ownerProps?: unknown;
  sessionId?: string;
}

export interface SlotContribution {
  id: string;
  key?: string;
  order?: number;
  priority?: number;
  select?(props: SlotComponentProps): boolean;
  component: ComponentType<SlotComponentProps>;
  children?: Array<Omit<SlotDeclaration, "parent">>;
}

interface OwnedDeclaration extends SlotDeclaration {
  epoch: symbol;
  closing?: boolean;
}

interface OwnedContribution extends SlotContribution {
  fiber: Fiber;
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
  private readonly contributions = new Map<string, Map<string, OwnedContribution>>();
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
    if (declaration.kind === "keyed" && existing) {
      const keys = new Set<string>();
      for (const entry of existing.values()) {
        if (!entry.key) throw new Error(`keyed slot contribution omitted its key: ${declaration.name}/${entry.id}`);
        if (keys.has(entry.key)) throw new Error(`keyed slot has duplicate key: ${declaration.name}/${entry.key}`);
        keys.add(entry.key);
      }
    }
    const owned = { ...declaration, epoch: Symbol(declaration.name) };
    this.declarations.set(declaration.name, owned);
    this.changed();
    return { name: declaration.name, epoch: owned.epoch };
  }

  private drainDeclarationTree(
    name: string,
    epoch: symbol,
    excludedFiber: Fiber,
    fibers: Set<Fiber>,
  ): void {
    const current = this.declarations.get(name);
    if (!current || current.epoch !== epoch) return;
    current.closing = true;
    for (const child of [...this.declarations.values()]) {
      if (child.parent === name) {
        this.drainDeclarationTree(child.name, child.epoch, excludedFiber, fibers);
      }
    }
    const entries = this.contributions.get(name);
    if (entries) {
      for (const entry of entries.values()) {
        if (entry.fiber !== excludedFiber) fibers.add(entry.fiber);
      }
    }
  }

  private async drainDeclaration(name: string, epoch: symbol, excludedFiber: Fiber): Promise<void> {
    const fibers = new Set<Fiber>();
    this.drainDeclarationTree(name, epoch, excludedFiber, fibers);
    for (const fiber of fibers) await fiber.dispose();
  }

  private finalizeDeclarationTree(name: string, epoch: symbol): void {
    const current = this.declarations.get(name);
    if (!current || current.epoch !== epoch) return;
    for (const child of [...this.declarations.values()]) {
      if (child.parent === name) this.finalizeDeclarationTree(child.name, child.epoch);
    }
    this.declarations.delete(name);
    this.contributions.delete(name);
  }

  private rollbackDeclaration(name: string, epoch: symbol): void {
    const current = this.declarations.get(name);
    if (!current || current.epoch !== epoch) return;
    this.declarations.delete(name);
    this.changed();
  }

  contribute(name: string, contribution: SlotContribution): SlotRegistration {
    const declaration = this.declarations.get(name);
    if (declaration?.closing) throw new Error(`slot is being removed: ${name}`);
    const entries = this.contributions.get(name) ?? new Map();
    if (entries.has(contribution.id)) {
      throw new Error(`duplicate slot contribution: ${name}/${contribution.id}`);
    }
    if (declaration?.kind === "single" && entries.size) {
      throw new Error(`single slot is already occupied: ${name}`);
    }
    if (declaration?.kind === "keyed") {
      if (!contribution.key) throw new Error(`keyed slot contribution omitted its key: ${name}/${contribution.id}`);
      if ([...entries.values()].some((entry) => entry.key === contribution.key)) {
        throw new Error(`keyed slot is already occupied: ${name}/${contribution.key}`);
      }
    }
    const ownerFiber = this.ctx.fiber;
    entries.set(contribution.id, { ...contribution, fiber: ownerFiber });
    this.contributions.set(name, entries);
    const children = new Map<string, SlotHandle>();
    try {
      for (const child of contribution.children ?? []) {
        const handle = this.declare({ ...child, parent: name });
        children.set(child.name, handle);
      }
    } catch (error) {
      for (const handle of [...children.values()].reverse()) {
        this.rollbackDeclaration(handle.name, handle.epoch);
      }
      entries.delete(contribution.id);
      if (!entries.size && this.contributions.get(name) === entries) {
        this.contributions.delete(name);
      }
      throw error;
    }
    this.changed();
    const dispose = this.ctx.effect(() => async () => {
      for (const handle of [...children.values()].reverse()) {
        await this.drainDeclaration(handle.name, handle.epoch, ownerFiber);
      }
      if (entries.delete(contribution.id)) {
        if (!entries.size && this.contributions.get(name) === entries) {
          this.contributions.delete(name);
        }
        this.changed();
      }
      for (const handle of [...children.values()].reverse()) {
        this.finalizeDeclarationTree(handle.name, handle.epoch);
      }
      if (children.size) this.changed();
    }, `remove slot contribution:${name}/${contribution.id}`);
    return { children, dispose };
  }

  entries(handle: SlotHandle): SlotContribution[] {
    const declaration = this.declarations.get(handle.name);
    if (!declaration || declaration.epoch !== handle.epoch) {
      throw new Error(`stale slot authorization: ${handle.name}`);
    }
    return [...(this.contributions.get(handle.name)?.values() ?? [])]
      .map(({ id, key, order, priority, select, component, children }) => ({
        id,
        ...(key === undefined ? {} : { key }),
        ...(order === undefined ? {} : { order }),
        ...(priority === undefined ? {} : { priority }),
        ...(select === undefined ? {} : { select }),
        component,
        ...(children === undefined ? {} : { children }),
      }))
      .sort((left, right) => declaration.kind === "chain"
        ? (right.priority ?? 0) - (left.priority ?? 0) || left.id.localeCompare(right.id)
        : (left.order ?? 0) - (right.order ?? 0) || left.id.localeCompare(right.id));
  }

  renderEntries(handle: SlotHandle, props: SlotComponentProps): SlotContribution[] {
    const declaration = this.declarations.get(handle.name);
    if (!declaration || declaration.epoch !== handle.epoch) {
      throw new Error(`stale slot authorization: ${handle.name}`);
    }
    if (declaration.scope === "session" && props.sessionId === undefined) return [];
    const entries = this.entries(handle);
    if (declaration.kind === "keyed") {
      return props.businessKey === undefined
        ? []
        : entries.filter((entry) => entry.key === props.businessKey);
    }
    if (declaration.kind !== "chain") return entries;
    const selected = entries.find((entry) => entry.select?.(props) ?? true);
    return selected ? [selected] : [];
  }

  renderKey(handle: SlotHandle, contributionId: string, sessionId?: string): string {
    const declaration = this.declarations.get(handle.name);
    if (!declaration || declaration.epoch !== handle.epoch) {
      throw new Error(`stale slot authorization: ${handle.name}`);
    }
    return declaration.scope === "session"
      ? `${contributionId}:${sessionId}`
      : contributionId;
  }

  pendingDeclarations(): string[] {
    return [...this.contributions]
      .filter(([name, entries]) => !this.declarations.has(name) && entries.size)
      .map(([name, entries]) => `${name} <- ${[...entries.keys()].sort().join(",")}`)
      .sort();
  }
}

interface ProjectionRow {
  seq: number;
  value: JsonValue | undefined;
}

export class ClientProjectionStore extends Service {
  private readonly rows = new Map<string, ProjectionRow>();
  private readonly frameSeq = new Map<string, number>();
  private readonly listeners = new Set<() => void>();
  private revision = 0;
  private transport: ClientProjectionTransport | undefined;
  private readonly followers = new Map<string, { count: number; stop?: () => void }>();

  constructor(ctx: Context) {
    super(ctx, "clientProjections");
  }

  private startFollower(
    sessionId: string,
    follower: { count: number; stop?: () => void },
    transport: ClientProjectionTransport,
  ): void {
    let baseline = true;
    follower.stop = transport.follow(sessionId, (frame) => {
      if (baseline) {
        baseline = false;
        this.frameSeq.delete(sessionId);
        this.truncate(sessionId, frame.lastDurableSeq);
      }
      this.applyFrame(frame);
    });
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

  applyFrame(frame: ProjectionFrame): void {
    if ((this.frameSeq.get(frame.sessionId) ?? -1) > frame.lastDurableSeq) return;
    this.frameSeq.set(frame.sessionId, frame.lastDurableSeq);
    const keys = new Set(frame.projections.map(({ key }) => this.rowKey(frame.sessionId, key)));
    let changed = false;
    for (const key of [...this.rows.keys()]) {
      if (!key.startsWith(`${frame.sessionId}\u0000`) || keys.has(key)) continue;
      if ((this.rows.get(key)?.seq ?? -1) > frame.lastDurableSeq) continue;
      this.rows.delete(key);
      changed = true;
    }
    for (const projection of frame.projections) {
      const key = this.rowKey(frame.sessionId, projection.key);
      const current = this.rows.get(key);
      if ((current?.seq ?? -1) > projection.seq) continue;
      if (current?.seq === projection.seq && current.value === projection.value) continue;
      this.rows.set(key, { seq: projection.seq, value: projection.value });
      changed = true;
    }
    if (!changed) return;
    this.revision += 1;
    for (const listener of this.listeners) listener();
  }

  connect(transport: ClientProjectionTransport): () => void {
    if (this.transport) throw new Error("client projection transport is already connected");
    this.transport = transport;
    for (const [sessionId, follower] of this.followers) {
      this.startFollower(sessionId, follower, transport);
    }
    return this.ctx.effect(() => () => {
      if (this.transport !== transport) return;
      this.transport = undefined;
      for (const follower of this.followers.values()) {
        follower.stop?.();
        delete follower.stop;
      }
    }, "disconnect client projection transport");
  }

  follow(sessionId: string): () => void {
    const existing = this.followers.get(sessionId);
    if (existing) {
      existing.count += 1;
    } else {
      const follower: { count: number; stop?: () => void } = { count: 1 };
      if (this.transport) {
        this.startFollower(sessionId, follower, this.transport);
      }
      this.followers.set(sessionId, follower);
    }
    return () => {
      const follower = this.followers.get(sessionId);
      if (!follower) return;
      follower.count -= 1;
      if (follower.count > 0) return;
      follower.stop?.();
      this.followers.delete(sessionId);
    };
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

export interface ClientProjectionTransport {
  follow(sessionId: string, listener: (frame: ProjectionFrame) => void): () => void;
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

export class ActiveSessionService extends Service {
  private value: string | undefined;
  private readonly listeners = new Set<() => void>();

  constructor(ctx: Context) {
    super(ctx, "activeSession");
    ctx.effect(() => () => this.listeners.clear(), "clear active Session listeners");
  }

  current(): string | undefined {
    return this.value;
  }

  select(sessionId: string): void {
    if (!sessionId) throw new Error("active Session id must not be empty");
    if (this.value === sessionId) return;
    this.value = sessionId;
    for (const listener of this.listeners) listener();
  }

  subscribe(listener: () => void): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  snapshot = (): string | undefined => this.value;
}

export type ScopedStoreUpdate<T> = T | ((current: T) => T);

export class ScopedStoreSeat<T> {
  private readonly values = new Map<string, T>();
  private readonly listeners = new Map<string, Set<() => void>>();
  private active = true;

  constructor(
    readonly id: string,
    private readonly initial: () => T,
  ) {}

  private assertActive(): void {
    if (!this.active) throw new Error(`stale scoped store seat: ${this.id}`);
  }

  get(sessionId: string): T {
    this.assertActive();
    if (!this.values.has(sessionId)) this.values.set(sessionId, this.initial());
    return this.values.get(sessionId)!;
  }

  set(sessionId: string, update: ScopedStoreUpdate<T>): void {
    this.assertActive();
    const current = this.get(sessionId);
    const next = typeof update === "function"
      ? (update as (current: T) => T)(current)
      : update;
    if (Object.is(current, next)) return;
    this.values.set(sessionId, next);
    for (const listener of this.listeners.get(sessionId) ?? []) listener();
  }

  subscribe(sessionId: string, listener: () => void): () => void {
    this.assertActive();
    const listeners = this.listeners.get(sessionId) ?? new Set();
    listeners.add(listener);
    this.listeners.set(sessionId, listeners);
    return () => {
      listeners.delete(listener);
      if (!listeners.size) this.listeners.delete(sessionId);
    };
  }

  close(): void {
    this.active = false;
    this.values.clear();
    this.listeners.clear();
  }
}

export class ScopedStoresService extends Service {
  private readonly seats = new Map<string, ScopedStoreSeat<unknown>>();

  constructor(ctx: Context) {
    super(ctx, "scopedStores");
  }

  define<T>(id: string, initial: () => T): ScopedStoreSeat<T> {
    if (this.seats.has(id)) throw new Error(`duplicate scoped store seat: ${id}`);
    const seat = new ScopedStoreSeat(id, initial);
    this.seats.set(id, seat as ScopedStoreSeat<unknown>);
    this.ctx.effect(() => () => {
      if (this.seats.get(id) !== seat) return;
      this.seats.delete(id);
      seat.close();
    }, `remove scoped store seat:${id}`);
    return seat;
  }
}

export interface ClientPluginModule {
  default: Plugin;
}

export type ClientModuleFactory = () => Promise<ClientPluginModule>;

function pluginInject(plugin: Plugin): string[] {
  if (!plugin.inject) return [];
  return Array.isArray(plugin.inject) ? plugin.inject.map(String) : Object.keys(plugin.inject);
}

function pluginProvide(plugin: Plugin): string[] {
  if (!plugin.provide) return [];
  return Array.isArray(plugin.provide) ? plugin.provide : [plugin.provide];
}

function dependencyCycle(graph: Map<string, Set<string>>): string[] | undefined {
  const visiting = new Set<string>();
  const visited = new Set<string>();
  const path: string[] = [];

  function visit(id: string): string[] | undefined {
    if (visiting.has(id)) {
      const start = path.indexOf(id);
      return [...path.slice(start), id];
    }
    if (visited.has(id)) return;
    visiting.add(id);
    path.push(id);
    for (const dependency of graph.get(id) ?? []) {
      const cycle = visit(dependency);
      if (cycle) return cycle;
    }
    path.pop();
    visiting.delete(id);
    visited.add(id);
  }

  for (const id of graph.keys()) {
    const cycle = visit(id);
    if (cycle) return cycle;
  }
}

export class ClientModuleSystem {
  private readonly arrivals = new Map<string, { revision: string; factory: ClientModuleFactory }>();
  private readonly modules = new Map<string, ClientPluginModule>();
  private readonly materializing = new Map<string, Promise<ClientPluginModule>>();
  private readonly fibers = new Map<string, Fiber>();
  private transition: Promise<void> = Promise.resolve();
  private closing = false;

  constructor(private readonly ctx: Context) {}

  private enqueue<T>(operation: () => Promise<T>): Promise<T> {
    const task = this.transition.then(operation, operation);
    this.transition = task.then(() => undefined, () => undefined);
    return task;
  }

  arrive(id: string, revision: string, factory: ClientModuleFactory): void {
    if (this.closing) throw new Error("client module system is stopping");
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
      if (this.arrivals.get(id) === arrival) this.modules.set(id, module);
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
    if (this.closing) throw new Error("client module system is stopping");
    return this.enqueue(() => this.activateAllNow(ids));
  }

  private async activateAllNow(ids: string[]): Promise<void> {
    const pending: Array<{ id: string; fiber: Fiber; module: ClientPluginModule }> = [];
    try {
      for (const id of ids) {
        const active = this.fibers.get(id);
        if (active?.uid === null) this.fibers.delete(id);
        else if (active) continue;
        const module = await this.materialize(id);
        const materialized = this.fibers.get(id);
        if (materialized?.uid === null) this.fibers.delete(id);
        else if (materialized) continue;
        const fiber = this.ctx.plugin(module.default);
        this.fibers.set(id, fiber);
        pending.push({ id, fiber, module });
      }
      await Promise.all(pending.map(({ fiber }) => fiber.await()));
      this.auditActivation(pending);
    } catch (error) {
      for (const { id, fiber } of pending.reverse()) {
        await fiber.dispose();
        this.fibers.delete(id);
        this.modules.delete(id);
      }
      throw error;
    }
  }

  private auditActivation(pending: Array<{
    id: string;
    fiber: Fiber;
    module: ClientPluginModule;
  }>): void {
    const unresolved = new Map<string, string[]>();
    for (const { id, module } of pending) {
      const missing = pluginInject(module.default)
        .filter((name) => this.ctx.reflect.get(name) === undefined);
      if (missing.length) unresolved.set(id, missing);
    }
    if (!unresolved.size) return;

    const providers = new Map<string, string[]>();
    for (const { id, module } of pending) {
      for (const service of pluginProvide(module.default)) {
        const ids = providers.get(service) ?? [];
        ids.push(id);
        providers.set(service, ids);
      }
    }
    const graph = new Map<string, Set<string>>();
    const missing: string[] = [];
    for (const [id, services] of unresolved) {
      const dependencies = new Set<string>();
      graph.set(id, dependencies);
      for (const service of services) {
        const candidates = providers.get(service) ?? [];
        if (!candidates.length) missing.push(`${id} -> ${service}`);
        for (const candidate of candidates) dependencies.add(candidate);
      }
    }
    const cycle = dependencyCycle(graph);
    const reasons = [
      ...(missing.length ? [`missing services: ${missing.join(", ")}`] : []),
      ...(cycle ? [`dependency cycle: ${cycle.join(" -> ")}`] : []),
      ...(unresolved.size
        ? [`unresolved modules: ${[...unresolved.keys()].join(", ")}`]
        : []),
    ];
    throw new Error(`client startup dependency audit failed; ${reasons.join("; ")}`);
  }

  auditReady(): void {
    const pendingSlots = this.ctx.slots.pendingDeclarations();
    if (!pendingSlots.length) return;
    throw new Error(`client startup slot audit failed; undeclared slots: ${pendingSlots.join("; ")}`);
  }

  async invalidate(id: string): Promise<void> {
    if (this.closing) throw new Error("client module system is stopping");
    return this.enqueue(() => this.invalidateNow(id));
  }

  private async invalidateNow(id: string): Promise<void> {
    this.arrivals.delete(id);
    try {
      await this.materializing.get(id);
    } catch {
      // A failed candidate still needs its arrival state invalidated.
    }
    await this.fibers.get(id)?.dispose();
    this.fibers.delete(id);
    this.modules.delete(id);
  }

  async dispose(): Promise<void> {
    if (this.closing) return this.transition;
    this.closing = true;
    return this.enqueue(async () => {
      for (const id of [...this.arrivals.keys()].reverse()) await this.invalidateNow(id);
    });
  }
}

declare module "cordis" {
  interface Context {
    activeSession: ActiveSessionService;
    clientActions: ClientActionsService;
    clientProjections: ClientProjectionStore;
    slots: SlotsService;
    scopedStores: ScopedStoresService;
  }
}

export const clientKernelPlugin: Plugin = function clientKernel(ctx: Context) {
  new ActiveSessionService(ctx);
  new ClientActionsService(ctx);
  new ClientProjectionStore(ctx);
  new ScopedStoresService(ctx);
  new SlotsService(ctx);
};

clientKernelPlugin.provide = [
  "activeSession",
  "clientActions",
  "clientProjections",
  "scopedStores",
  "slots",
];

export function useScopedStore<T>(
  seat: ScopedStoreSeat<T>,
  sessionId: string,
): readonly [T, (update: ScopedStoreUpdate<T>) => void] {
  const subscribe = useCallback(
    (listener: () => void) => seat.subscribe(sessionId, listener),
    [seat, sessionId],
  );
  const snapshot = useCallback(() => seat.get(sessionId), [seat, sessionId]);
  const value = useSyncExternalStore(subscribe, snapshot, snapshot);
  const set = useCallback(
    (update: ScopedStoreUpdate<T>) => seat.set(sessionId, update),
    [seat, sessionId],
  );
  return [value, set] as const;
}

export function useActiveSession(client: Context): string | undefined {
  return useSyncExternalStore(
    client.activeSession.subscribe.bind(client.activeSession),
    client.activeSession.snapshot,
    client.activeSession.snapshot,
  );
}

export function SlotOutlet(props: {
  client: Context;
  slot: SlotHandle;
  ownerProps?: unknown;
  businessKey?: string;
  sessionId?: string;
  empty?: ReactNode;
}): ReactNode {
  useSyncExternalStore(
    props.client.slots.subscribe.bind(props.client.slots),
    props.client.slots.snapshot,
    props.client.slots.snapshot,
  );
  const componentProps: SlotComponentProps = {
    client: props.client,
    ...(props.businessKey === undefined ? {} : { businessKey: props.businessKey }),
    ...(props.ownerProps === undefined ? {} : { ownerProps: props.ownerProps }),
    ...(props.sessionId === undefined ? {} : { sessionId: props.sessionId }),
  };
  const entries = props.client.slots.renderEntries(props.slot, componentProps);
  if (!entries.length) return props.empty ?? null;
  return entries.map((entry) =>
    createElement(entry.component, {
      key: props.client.slots.renderKey(props.slot, entry.id, props.sessionId),
      ...componentProps,
    }),
  );
}

export function useProjection<T extends JsonValue>(
  client: Context,
  sessionId: string,
  key: string,
): T | undefined {
  useEffect(() => sessionId ? client.clientProjections.follow(sessionId) : undefined, [client, sessionId]);
  useSyncExternalStore(
    client.clientProjections.subscribe.bind(client.clientProjections),
    client.clientProjections.snapshot,
    client.clientProjections.snapshot,
  );
  return client.clientProjections.get(sessionId, key)?.value as T | undefined;
}

export { Context, Service } from "cordis";
export type { Plugin } from "cordis";
