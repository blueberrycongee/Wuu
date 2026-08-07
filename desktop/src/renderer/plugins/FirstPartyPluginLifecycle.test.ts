import { readFileSync } from "node:fs";
import { basename, resolve } from "node:path";

import * as React from "react";
import { afterEach, describe, expect, it } from "vitest";

import {
  PluginHost,
  type PluginContributionDeclarations,
  type PluginGenerationApi,
} from "./PluginHost";

interface FirstPartyManifest {
  id: string;
  desktop: { entry: string };
  contributes?: {
    slots?: PluginContributionDeclarations["slots"];
    surfaces?: PluginContributionDeclarations["surfaces"];
    presenters?: PluginContributionDeclarations["presenters"];
    navigation?: PluginContributionDeclarations["navigation"];
    workspaceTools?: PluginContributionDeclarations["workspace_tools"];
    settingsPages?: PluginContributionDeclarations["settings_pages"];
  };
}

interface FirstPartyDesktopModule {
  activate(api: PluginGenerationApi): void | Promise<void>;
}

const repositoryRoot = basename(process.cwd()) === "desktop"
  ? resolve(process.cwd(), "..")
  : process.cwd();
const bundledRoot = resolve(repositoryRoot, "internal/plugin/bundled");
const firstPartyPluginIds = ["goal", "subagent", "automation", "memory", "dream"] as const;

afterEach(() => {
  for (const style of document.head.querySelectorAll("style[data-wuu-plugin-id]")) {
    style.remove();
  }
});

describe("first-party desktop plugin lifecycle", () => {
  it("activates real bundled modules and removes every contribution on disable", async () => {
    const host = new PluginHost({ react: React });

    for (const pluginId of firstPartyPluginIds) {
      const { manifest, module } = await loadFirstPartyPlugin(pluginId);
      await host.activateGeneration({
        pluginId,
        generation: "generation-one",
        contributions: contributionDeclarations(manifest),
        register: module.activate,
      });
    }

    expect(host.getSlotSnapshot("composer.above").map((item) => item.pluginId)).toEqual([
      "goal",
      "subagent",
    ]);
    expect(host.getSlotSnapshot("settings.plugin").map((item) => item.pluginId)).toEqual([
      "subagent",
    ]);
    expect(host.getSlotSnapshot("composer.toolbar").map((item) => item.pluginId)).toEqual([
      "subagent",
    ]);
    expect(host.getViewTypes().map((item) => `${item.pluginId}:${item.id}`)).toEqual([
      "automation:automation.catalog",
      "dream:dream.settings",
      "memory:memory.settings",
    ]);
    expect(host.getNavigationEntries().map((item) => `${item.pluginId}:${item.view}`)).toEqual([
      "automation:automation.catalog",
    ]);
    expect(host.getSettingsPages().map((item) => `${item.pluginId}:${item.view}`)).toEqual([
      "memory:memory.settings",
      "dream:dream.settings",
    ]);
    expect(document.head.querySelectorAll("style[data-wuu-plugin-id]")).toHaveLength(5);

    for (const pluginId of firstPartyPluginIds) {
      host.disable(pluginId);
    }

    expect(host.getSlotSnapshot("composer.above")).toEqual([]);
    expect(host.getSlotSnapshot("settings.plugin")).toEqual([]);
    expect(host.getSlotSnapshot("composer.toolbar")).toEqual([]);
    expect(host.getViewTypes()).toEqual([]);
    expect(host.getNavigationEntries()).toEqual([]);
    expect(host.getSettingsPages()).toEqual([]);
    expect(document.head.querySelector("style[data-wuu-plugin-id]")).toBeNull();
  });

  it.each(firstPartyPluginIds)("atomically replaces the %s desktop generation", async (pluginId) => {
    const host = new PluginHost({ react: React });
    const { manifest, module } = await loadFirstPartyPlugin(pluginId);
    const contributions = contributionDeclarations(manifest);

    await host.activateGeneration({
      pluginId,
      generation: "old-generation",
      contributions,
      register: module.activate,
    });
    await host.activateGeneration({
      pluginId,
      generation: "new-generation",
      contributions,
      register: module.activate,
    });

    expect(host.isGenerationActive(pluginId, "old-generation")).toBe(false);
    expect(host.isGenerationActive(pluginId, "new-generation")).toBe(true);
    expect(document.head.querySelector(`style[data-wuu-plugin-id="${pluginId}"][data-wuu-plugin-generation="old-generation"]`)).toBeNull();
    expect(document.head.querySelectorAll(`style[data-wuu-plugin-id="${pluginId}"][data-wuu-plugin-generation="new-generation"]`)).toHaveLength(1);
    expect([
      ...host.getSlotSnapshot("composer.above"),
      ...host.getSlotSnapshot("composer.toolbar"),
      ...host.getSlotSnapshot("settings.plugin"),
      ...host.getViewTypes(),
    ].filter((item) => item.pluginId === pluginId).every((item) => item.generation === "new-generation"))
      .toBe(true);
  });
});

async function loadFirstPartyPlugin(
  pluginId: typeof firstPartyPluginIds[number],
): Promise<{ manifest: FirstPartyManifest; module: FirstPartyDesktopModule }> {
  const pluginRoot = resolve(bundledRoot, pluginId);
  const manifest = JSON.parse(readFileSync(resolve(pluginRoot, "plugin.json"), "utf8")) as FirstPartyManifest;
  const source = readFileSync(resolve(pluginRoot, manifest.desktop.entry), "utf8");
  const executableSource = source.replace(
    "export async function activate(api)",
    "return async function activate(api)",
  );
  if (executableSource === source) {
    throw new Error(`First-party plugin ${pluginId} does not export activate(api)`);
  }
  const activate: unknown = Function(executableSource)();
  if (typeof activate !== "function") {
    throw new Error(`First-party plugin ${pluginId} has an invalid activate export`);
  }
  return {
    manifest,
    module: { activate: (api) => Reflect.apply(activate, undefined, [api]) as void | Promise<void> },
  };
}

function contributionDeclarations(manifest: FirstPartyManifest): PluginContributionDeclarations {
  const contributions = manifest.contributes ?? {};
  return {
    slots: contributions.slots ?? [],
    surfaces: contributions.surfaces ?? [],
    presenters: contributions.presenters ?? [],
    navigation: contributions.navigation ?? [],
    workspace_tools: contributions.workspaceTools ?? [],
    settings_pages: contributions.settingsPages ?? [],
  };
}
