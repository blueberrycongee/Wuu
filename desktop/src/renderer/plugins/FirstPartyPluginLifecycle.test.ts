import { readFileSync } from "node:fs";
import { basename, resolve } from "node:path";

import * as React from "react";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, describe, expect, it } from "vitest";

import {
  PluginHost,
  type PluginContributionDeclarations,
  type PluginGenerationApi,
} from "./PluginHost";
import { PluginSlot } from "./PluginSlot";
import { PluginViewContent, WorkbenchController } from "./Workbench";

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
const firstPartyPluginIds = ["goal", "subagent", "automation", "memory", "dream", "plan"] as const;

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
    expect(host.getSlotSnapshot("composer.toolbar")).toEqual([]);
    expect(host.getViewTypes().map((item) => `${item.pluginId}:${item.id}`)).toEqual([
      "automation:automation.catalog",
      "dream:dream.settings",
      "memory:memory.settings",
      "subagent:subagent.settings",
    ]);
    expect(host.getNavigationEntries().map((item) => `${item.pluginId}:${item.view}`)).toEqual([
      "automation:automation.catalog",
    ]);
    expect(host.getSettingsPages().map((item) => `${item.pluginId}:${item.view}`)).toEqual([
      "memory:memory.settings",
      "subagent:subagent.settings",
      "dream:dream.settings",
    ]);
    expect(host.getInspectorSections().map((item) => `${item.pluginId}:${item.id}`)).toEqual([
      "plan:current-plan",
    ]);
    expect(host.getPresenters("conversation.tool-activity", "plan").at(-1)?.pluginId).toBe("plan");
    expect(document.head.querySelectorAll("style[data-wuu-plugin-id]")).toHaveLength(6);

    for (const pluginId of firstPartyPluginIds) {
      host.disable(pluginId);
    }

    expect(host.getSlotSnapshot("composer.above")).toEqual([]);
    expect(host.getSlotSnapshot("composer.toolbar")).toEqual([]);
    expect(host.getViewTypes()).toEqual([]);
    expect(host.getNavigationEntries()).toEqual([]);
    expect(host.getSettingsPages()).toEqual([]);
    expect(host.getInspectorSections()).toEqual([]);
    expect(host.getPresenters("conversation.tool-activity", "plan")).toEqual([]);
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
      ...host.getViewTypes(),
    ].filter((item) => item.pluginId === pluginId).every((item) => item.generation === "new-generation"))
      .toBe(true);
  });

  it("hides the previous session's subagent status while the next session loads", async () => {
    const pending = new Map<string, (value: unknown) => void>();
    let runtimeCalls = 0;
    const host = new PluginHost({
      react: React,
      invokeRuntime: ({ method, input }) => {
        if (method !== "status.list") {
          return Promise.reject(new Error(`Unexpected subagent runtime method: ${method}`));
        }
        runtimeCalls += 1;
        const threadId = String((input as { parent_session_id?: string })?.parent_session_id ?? "");
        return new Promise((resolve) => pending.set(threadId, resolve));
      },
    });
    const { manifest, module } = await loadFirstPartyPlugin("subagent");
    await host.activateGeneration({
      pluginId: "subagent",
      generation: "session-status-test",
      contributions: contributionDeclarations(manifest),
      register: module.activate,
    });
    const container = document.createElement("div");
    document.body.appendChild(container);
    const root = createRoot(container);
    try {
      await act(async () => {
        root.render(React.createElement(PluginSlot, {
          host,
          id: "composer.above",
          context: { threadId: "thread-a", mainConversation: true },
        }));
      });
      act(() => {
        host.publishHostEvent({ kind: "notification", message: { method: "turn/event" } });
        host.publishHostEvent({ kind: "notification", message: { method: "turn/usage" } });
      });
      expect(runtimeCalls).toBe(1);
      await act(async () => {
        pending.get("thread-a")?.({ sessions: [{ session_id: "child-a", name: "from-a", state: "running" }] });
      });
      expect(container.textContent).toContain("from-a · running");

      await act(async () => {
        root.render(React.createElement(PluginSlot, {
          host,
          id: "composer.above",
          context: { threadId: "thread-b", mainConversation: true },
        }));
      });

      expect(container.textContent).not.toContain("from-a");
    } finally {
      act(() => root.unmount());
      container.remove();
      host.disable("subagent");
    }
  });

  it.each([
    ["automation", "automation.catalog", "还没有自动化任务"],
    ["memory", "memory.settings", "记忆概览"],
    ["dream", "dream.settings", "后台记忆整合"],
  ] as const)("renders the real %s view with localized host UI", async (pluginId, viewTypeId, expectedText) => {
    const host = new PluginHost({
      react: React,
      invokeRuntime: async ({ method }) => firstPartyRuntimeResponse(method),
    });
    const { manifest, module } = await loadFirstPartyPlugin(pluginId);
    await host.activateGeneration({
      pluginId,
      generation: "render-test",
      contributions: contributionDeclarations(manifest),
      register: module.activate,
    });
    const controller = new WorkbenchController(host);
    const container = document.createElement("div");
    document.body.appendChild(container);
    const root = createRoot(container);
    try {
      await act(async () => {
        root.render(React.createElement(PluginViewContent, {
          controller,
          pluginId,
          viewTypeId,
        }));
      });
      expect(container.querySelector(".plugin-ui-page")).not.toBeNull();
      expect(container.textContent).toContain(expectedText);
      expect(container.textContent).not.toMatch(/\b(?:title|subtitle|overview|loading|chat|raw|enabled|interval|minimum|model|new|empty)\b/);
      expect([...container.querySelectorAll("input, textarea")].every((control) => control.closest("label"))).toBe(true);
    } finally {
      act(() => root.unmount());
      container.remove();
      controller.dispose();
    }
  });
});

function firstPartyRuntimeResponse(method: string): unknown {
  switch (method) {
    case "automation.list":
      return { tasks: [] };
    case "dream.get":
      return {
        settings: { enabled: false, interval_days: 7, min_sessions: 5, model_alias: "" },
        candidates: {},
        last_status: "",
      };
    case "memory.overview.start":
      return { id: "overview-job" };
    case "memory.job.get":
      return { state: "completed", output: "" };
    case "memory.read":
      return { index_raw: "", files: [] };
    default:
      throw new Error(`Unexpected first-party runtime method: ${method}`);
  }
}

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
