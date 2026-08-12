import { createHash } from "node:crypto";
import { mkdir, readFile, readdir } from "node:fs/promises";
import * as nodeModule from "node:module";
import { join } from "node:path";
import { pathToFileURL } from "node:url";
import type { Fiber, Plugin } from "cordis";
import type { JsonValue, ToolDefinition } from "@wuu-v2/contracts";
import { Service, type Context } from "@wuu-v2/kernel";

export interface WorkspaceLoaderConfig {
  directory: string;
}

interface LoadedWorkspacePlugin {
  id: string;
  revision: string;
  plugin: Plugin;
  fiber: Fiber;
}

interface WorkspaceModule {
  default?: unknown;
}

const pluginIdPattern = /^[a-z0-9][a-z0-9._-]{0,63}$/;

function objectInput(input: JsonValue): Record<string, JsonValue> {
  if (!input || typeof input !== "object" || Array.isArray(input)) {
    throw new Error("workspace plugin input must be an object");
  }
  return input;
}

function pluginId(input: JsonValue): string {
  const id = objectInput(input).id;
  if (typeof id !== "string" || !pluginIdPattern.test(id)) {
    throw new Error("workspace plugin id must match [a-z0-9][a-z0-9._-]{0,63}");
  }
  return id;
}

function textResult(text: string) {
  return { content: [{ type: "text" as const, text }] };
}

function revision(source: string): string {
  return `sha256:${createHash("sha256").update(source).digest("hex")}`;
}

function executablePlugin(id: string, candidate: unknown): Plugin {
  if (typeof candidate !== "function") {
    throw new Error(`workspace plugin ${id} must export a default plugin function`);
  }
  const inject = (candidate as { inject?: unknown }).inject;
  if (
    !Array.isArray(inject) &&
    (!inject || typeof inject !== "object")
  ) {
    throw new Error(`workspace plugin ${id} must declare an explicit inject list`);
  }
  if (candidate.constructor.name === "AsyncFunction") {
    throw new Error(`workspace plugin ${id} must register synchronously`);
  }
  const plugin = candidate as (ctx: Context, config: unknown) => unknown;
  const guarded = (ctx: Context, config: unknown) => {
    const result = plugin(ctx, config);
    if (
      result &&
      typeof result !== "function" &&
      typeof (result as PromiseLike<unknown>).then === "function"
    ) {
      throw new Error(`workspace plugin ${id} returned an asynchronous apply result`);
    }
    if (result !== undefined) {
      throw new Error(`workspace plugin ${id} apply must return void; registrations own cleanup`);
    }
  };
  Object.assign(guarded, plugin);
  Object.defineProperty(guarded, "name", { value: `workspace:${id}` });
  return guarded as Plugin;
}

async function readPlugin(entry: string): Promise<{ source: string; revision: string }> {
  const source = await readFile(entry, "utf8");
  if (Buffer.byteLength(source, "utf8") > 1_000_000) {
    throw new Error("workspace plugin exceeds the 1 MB source limit");
  }
  return { source, revision: revision(source) };
}

async function importPlugin(
  id: string,
  entry: string,
  source: string,
  sourceRevision: string,
): Promise<Plugin> {
  if (typeof nodeModule.stripTypeScriptTypes !== "function") {
    throw new Error("workspace TypeScript plugins require Node.js stripTypeScriptTypes support");
  }
  const transformed = nodeModule.stripTypeScriptTypes(source, {
    mode: "strip",
    sourceUrl: pathToFileURL(entry).href,
  });
  const encoded = Buffer.from(transformed, "utf8").toString("base64");
  const module = await import(
    `data:text/javascript;base64,${encoded}#${sourceRevision}`
  ) as WorkspaceModule;
  return executablePlugin(id, module.default);
}

export class WorkspacePluginsService extends Service {
  private readonly loaded = new Map<string, LoadedWorkspacePlugin>();
  private readonly failures = new Map<string, string>();
  private readonly seenRevisions = new Set<string>();
  private closing = false;

  constructor(ctx: Context, private readonly directory: string) {
    super(ctx, "workspacePlugins");
    ctx.hostActions.register("workspace-plugin/inspect", () =>
      ctx.runtimeInspection.snapshot() as unknown as JsonValue);
    ctx.hostActions.register("workspace-plugin/list", () => this.status());
    ctx.hostActions.register("workspace-plugin/load", async (input) => ({
      plugin: await this.load(pluginId(input)),
    }));
    ctx.hostActions.register("workspace-plugin/unload", async (input) => ({
      unloaded: await this.unload(pluginId(input)),
      modelChangesApplyFrom: "next-run",
    }));
    for (const tool of this.tools()) ctx.tools.register(tool.name, tool);
    ctx.effect(() => async () => {
      this.closing = true;
      await this.ctx.composition.run(async () => {
        for (const entry of [...this.loaded.values()].reverse()) await entry.fiber.dispose();
        this.loaded.clear();
      });
    }, "unload workspace plugins");
  }

  private assertOpen(): void {
    if (this.closing) throw new Error("workspace plugin loader is stopping");
  }

  private enqueue<T>(operation: () => Promise<T>): Promise<T> {
    this.assertOpen();
    return this.ctx.composition.run(operation);
  }

  private entry(id: string): string {
    return join(this.directory, id, "index.ts");
  }

  private assertSupportedFiber(id: string, fiber: Fiber): void {
    if (fiber.state !== 2) {
      const snapshot = this.ctx.runtimeInspection.snapshot().fibers.find(
        ({ uid }) => uid === fiber.uid,
      );
      throw new Error(snapshot?.pending.length
        ? `workspace plugin ${id} is missing services: ${snapshot.pending.join(", ")}`
        : `workspace plugin ${id} did not become active`);
    }
    const unsupported = fiber.getEffects()
      .map(({ label }) => label)
      .filter((label) =>
        !label.startsWith("unregister tools:") &&
        !label.startsWith("unregister prompts:") &&
        !label.startsWith("unregister projection:"));
    if (unsupported.length) {
      throw new Error(`workspace plugin ${id} uses unsupported setup effects: ${unsupported.join(", ")}`);
    }
  }

  list(): Array<{ id: string; revision: string }> {
    return [...this.loaded.values()]
      .map(({ id, revision }) => ({ id, revision }))
      .sort((left, right) => left.id.localeCompare(right.id));
  }

  status(): {
    plugins: Array<{ id: string; revision: string }>;
    failures: Array<{ id: string; error: string }>;
  } {
    return {
      plugins: this.list(),
      failures: [...this.failures]
        .map(([id, error]) => ({ id, error }))
        .sort((left, right) => left.id.localeCompare(right.id)),
    };
  }

  load(id: string): Promise<{
    id: string;
    revision: string;
    modelChangesApplyFrom: "next-run";
  }> {
    if (!pluginIdPattern.test(id)) throw new Error(`invalid workspace plugin id: ${id}`);
    return this.enqueue(() => this.replace(id)).then((result) => {
      this.failures.delete(id);
      return result;
    }, (error) => {
      this.failures.set(id, error instanceof Error ? error.message : String(error));
      throw error;
    });
  }

  private async replace(id: string): Promise<{
    id: string;
    revision: string;
    modelChangesApplyFrom: "next-run";
  }> {
    this.assertOpen();
    const entry = this.entry(id);
    const candidateSource = await readPlugin(entry);
    this.assertOpen();
    const previous = this.loaded.get(id);
    if (previous?.revision === candidateSource.revision) {
      return { id, revision: candidateSource.revision, modelChangesApplyFrom: "next-run" };
    }
    const revisionKey = `${id}:${candidateSource.revision}`;
    if (!this.seenRevisions.has(revisionKey) && this.seenRevisions.size >= 128) {
      throw new Error("workspace plugin generation limit reached; restart the Harness before loading more revisions");
    }
    this.seenRevisions.add(revisionKey);
    const candidate = await importPlugin(
      id,
      entry,
      candidateSource.source,
      candidateSource.revision,
    );
    this.assertOpen();

    if (previous) {
      await previous.fiber.dispose();
      this.loaded.delete(id);
    }
    this.assertOpen();

    let fiber: Fiber | undefined;
    try {
      fiber = this.ctx.root.plugin(candidate);
      await fiber.await();
      this.assertSupportedFiber(id, fiber);
      this.assertOpen();
      this.loaded.set(id, {
        id,
        revision: candidateSource.revision,
        plugin: candidate,
        fiber,
      });
      return { id, revision: candidateSource.revision, modelChangesApplyFrom: "next-run" };
    } catch (error) {
      await fiber?.dispose();
      if (!previous || this.closing) throw error;
      let rollback: Fiber | undefined;
      try {
        rollback = this.ctx.root.plugin(previous.plugin);
        await rollback.await();
        this.assertSupportedFiber(id, rollback);
        this.loaded.set(id, { ...previous, fiber: rollback });
      } catch (rollbackError) {
        await rollback?.dispose();
        throw new AggregateError(
          [error, rollbackError],
          `workspace plugin ${id} failed and its previous generation could not be restored`,
        );
      }
      throw error;
    }
  }

  unload(id: string): Promise<boolean> {
    if (!pluginIdPattern.test(id)) throw new Error(`invalid workspace plugin id: ${id}`);
    return this.enqueue(async () => {
      const current = this.loaded.get(id);
      if (!current) return false;
      await current.fiber.dispose();
      this.loaded.delete(id);
      this.failures.delete(id);
      return true;
    });
  }

  async discover(): Promise<void> {
    await mkdir(this.directory, { recursive: true });
    const entries = await readdir(this.directory, { withFileTypes: true });
    for (const entry of entries.filter((item) => item.isDirectory()).sort((a, b) => a.name.localeCompare(b.name))) {
      if (!pluginIdPattern.test(entry.name)) continue;
      try {
        await this.load(entry.name);
      } catch (error) {
        this.failures.set(entry.name, error instanceof Error ? error.message : String(error));
        this.ctx.logger.error(error);
      }
    }
  }

  private tools(): ToolDefinition[] {
    return [{
      name: "workspace_runtime_inspect",
      description: "Inspect active Services, Tools, Prompts, projections, actions, and Cordis Fibers.",
      access: "read",
      inputSchema: { type: "object", properties: {}, additionalProperties: false },
      execute: async () => textResult(JSON.stringify(this.ctx.runtimeInspection.snapshot())),
    }, {
      name: "workspace_plugin_list",
      description: "List active trusted TypeScript plugins from .wuu-v2/plugins.",
      access: "read",
      inputSchema: { type: "object", properties: {}, additionalProperties: false },
      execute: async () => textResult(JSON.stringify(this.status())),
    }, {
      name: "workspace_plugin_load",
      description: "Load or reload one trusted TypeScript plugin. Model-visible changes apply from the next Agent run.",
      access: "execute",
      inputSchema: {
        type: "object",
        properties: { id: { type: "string" } },
        required: ["id"],
        additionalProperties: false,
      },
      execute: async (input) => textResult(JSON.stringify(await this.load(pluginId(input)))),
    }, {
      name: "workspace_plugin_unload",
      description: "Unload one runtime generation until restart. The current Agent run keeps its frozen Tool surface.",
      access: "execute",
      inputSchema: {
        type: "object",
        properties: { id: { type: "string" } },
        required: ["id"],
        additionalProperties: false,
      },
      execute: async (input) => textResult(JSON.stringify({
        unloaded: await this.unload(pluginId(input)),
        modelChangesApplyFrom: "next-run",
      })),
    }];
  }
}

declare module "cordis" {
  interface Context {
    workspacePlugins: WorkspacePluginsService;
  }
}

export const workspaceLoaderPlugin: Plugin<WorkspaceLoaderConfig> = async function workspaceLoader(
  ctx,
  config,
) {
  const service = new WorkspacePluginsService(ctx, config.directory);
  await service.discover();
};

workspaceLoaderPlugin.inject = ["composition", "hostActions", "runtimeInspection", "tools"];
workspaceLoaderPlugin.provide = "workspacePlugins";
