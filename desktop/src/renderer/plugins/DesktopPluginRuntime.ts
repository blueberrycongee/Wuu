import * as React from "react";
import { useEffect } from "react";

import type { ExtensionInventoryRecord } from "../../shared/protocol";
import { syncExtensionTheme } from "../Theme";
import {
  PluginHost,
  type PluginContributionDeclarations,
  type PluginGenerationApi,
} from "./PluginHost";
import { WorkbenchController } from "./Workbench";

interface DesktopPluginModule {
  activate(api: PluginGenerationApi): void | Promise<void>;
}

type ModuleLoader = (url: string) => Promise<unknown>;

export interface DesktopPluginFailure {
  pluginId: string;
  fingerprint: string;
  error: unknown;
}

export const desktopPluginHost = new PluginHost({
  react: React,
  invokeRuntime: async ({ pluginId, generation, method, input }) => {
    const response = await window.wuu?.requestPluginRuntime?.({
      id: pluginId,
      fingerprint: generation,
      method,
      input,
    });
    if (!response) throw new Error("Plugin runtime requests are unavailable");
    return response.result;
  },
});
export const desktopWorkbenchController = new WorkbenchController(desktopPluginHost);

export class DesktopPluginRuntime {
  private readonly activeGenerations = new Map<string, string>();
  private syncEpoch = 0;

  constructor(
    readonly host: PluginHost,
    private readonly loadModule: ModuleLoader = importDesktopPluginModule,
  ) {}

  async sync(inventory: readonly ExtensionInventoryRecord[]): Promise<readonly DesktopPluginFailure[]> {
    const epoch = ++this.syncEpoch;
    const preferences = await window.wuu?.getPluginConflictPreferences?.();
    if (preferences) this.host.setConflictPreferences(preferences);
    const desired = new Map(
      inventory
        .filter(isLoadableDesktopPlugin)
        .map((plugin) => [plugin.id, plugin] as const),
    );

    for (const pluginId of this.activeGenerations.keys()) {
      if (!desired.has(pluginId)) {
        this.host.unload(pluginId);
        this.activeGenerations.delete(pluginId);
      }
    }

    const failures: DesktopPluginFailure[] = [];
    for (const plugin of desired.values()) {
      const fingerprint = plugin.fingerprint;
      if (!fingerprint || this.activeGenerations.get(plugin.id) === fingerprint) {
        continue;
      }
      try {
        const loaded = await window.wuu?.loadPluginDesktopModule?.({ id: plugin.id, fingerprint });
        if (!loaded || loaded.id !== plugin.id || loaded.fingerprint !== fingerprint) {
          throw new Error("Desktop plugin module identity mismatch");
        }
        await this.host.activateGeneration({
          pluginId: plugin.id,
          generation: fingerprint,
          contributions: (plugin.contributions ?? {}) as PluginContributionDeclarations,
          register: async (api) => {
            const module = requireDesktopPluginModule(await this.loadModule(loaded.url));
            await module.activate(api);
          },
        });
        if (epoch === this.syncEpoch) {
          this.activeGenerations.set(plugin.id, fingerprint);
        }
      } catch (error: unknown) {
        failures.push({ pluginId: plugin.id, fingerprint, error });
      }
    }
    return failures;
  }
}

export const desktopPluginRuntime = new DesktopPluginRuntime(desktopPluginHost);

export function useDesktopPluginRuntime(
  inventory: readonly ExtensionInventoryRecord[] | undefined,
): void {
  useEffect(() => window.wuu?.onServerEvent?.((event) => {
    desktopPluginHost.publishHostEvent(event);
  }), []);
  useEffect(() => {
    syncExtensionTheme(inventory);
    const syncThemeFromOtherWindow = (): void => syncExtensionTheme(inventory);
    window.addEventListener("storage", syncThemeFromOtherWindow);
    void desktopPluginRuntime.sync(inventory ?? []).then((failures) => {
      for (const failure of failures) {
        console.error(`Desktop plugin ${failure.pluginId} failed to activate`, failure.error);
      }
    });
    return () => window.removeEventListener("storage", syncThemeFromOtherWindow);
  }, [inventory]);
}

function isLoadableDesktopPlugin(plugin: ExtensionInventoryRecord): boolean {
  const approved = plugin.approval_state === "granted" || plugin.approval_state === "official";
  const active = plugin.state === "granted" || plugin.state === "active";
  return plugin.kind === "plugin"
    && plugin.desktop !== undefined
    && plugin.enabled !== false
    && approved
    && active
    && typeof plugin.fingerprint === "string"
    && plugin.fingerprint.length > 0;
}

function requireDesktopPluginModule(value: unknown): DesktopPluginModule {
  if (typeof value !== "object" || value === null || !("activate" in value)) {
    throw new Error("Desktop plugin module must export activate(api)");
  }
  const activate = Reflect.get(value, "activate");
  if (typeof activate !== "function") {
    throw new Error("Desktop plugin module must export activate(api)");
  }
  return { activate: (api) => Reflect.apply(activate, value, [api]) as void | Promise<void> };
}

async function importDesktopPluginModule(url: string): Promise<unknown> {
  return import(/* @vite-ignore */ url);
}
