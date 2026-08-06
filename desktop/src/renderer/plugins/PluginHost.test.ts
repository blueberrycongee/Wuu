import * as React from "react";
import { afterEach, describe, expect, it } from "vitest";

import {
  PLUGIN_SLOT_IDS,
  PluginHost,
  type Disposable,
  type PluginGenerationApi,
} from "./PluginHost";

afterEach(() => {
  for (const style of document.head.querySelectorAll("style[data-wuu-plugin-id]")) {
    style.remove();
  }
});

describe("PluginHost", () => {
  it("publishes the fixed slots and orders registrations by order, plugin id, and contribution id", async () => {
    expect(PLUGIN_SLOT_IDS).toEqual([
      "sidebar.primary",
      "sidebar.footer",
      "workspace.header",
      "conversation.header",
      "conversation.message.before",
      "conversation.message.after",
      "composer.above",
      "composer.toolbar",
      "settings.plugin",
    ]);

    const host = new PluginHost({ react: React });
    let injectedReact: typeof React | undefined;

    await host.activateGeneration({
      pluginId: "zeta",
      generation: "one",
      register(api) {
        injectedReact = api.react;
        api.registerSlot("composer.toolbar", contribution("z-last", 10));
        api.registerSlot("composer.toolbar", contribution("z-first", 0));
        api.registerCommand(command("z-command", 10));
        api.registerLocale({ id: "base", locale: "en", entries: { shared: "zeta", zeta: "Zeta" } });
        api.registerStyle({ id: "z-style", css: ".zeta {}", order: 10 });
      },
    });
    await host.activateGeneration({
      pluginId: "alpha",
      generation: "one",
      register(api) {
        api.registerSlot("composer.toolbar", contribution("b", 0));
        api.registerSlot("composer.toolbar", contribution("a", 0));
        api.registerCommand(command("alpha-command", 0));
        api.registerLocale({ id: "base", locale: "en", entries: { alpha: "Alpha", shared: "alpha" } });
        api.registerStyle({ id: "alpha-style", css: ".alpha {}", order: 0 });
      },
    });

    expect(injectedReact).toBe(React);
    expect(host.getSlotSnapshot("composer.toolbar").map(qualifiedId)).toEqual([
      "alpha:a",
      "alpha:b",
      "zeta:z-first",
      "zeta:z-last",
    ]);
    expect(host.getCommands().map(qualifiedId)).toEqual(["alpha:alpha-command", "zeta:z-command"]);
    expect(host.getLocaleEntries("en")).toEqual({ alpha: "Alpha", shared: "zeta", zeta: "Zeta" });
    expect([...document.head.querySelectorAll<HTMLStyleElement>("style[data-wuu-plugin-id]")]
      .map((style) => `${style.dataset.wuuPluginId}:${style.dataset.wuuPluginStyle}`))
      .toEqual(["alpha:alpha-style", "zeta:z-style"]);
  });

  it("replaces a generation and makes stale generation handles harmless", async () => {
    const host = new PluginHost({ react: React });
    const cleanup: string[] = [];
    const oldHandle = await host.activateGeneration({
      pluginId: "notes",
      generation: "old",
      register(api) {
        registerCompleteGeneration(api, "old");
        api.registerCleanup(() => cleanup.push("old"));
      },
    });

    const newHandle = await host.activateGeneration({
      pluginId: "notes",
      generation: "new",
      register(api) {
        registerCompleteGeneration(api, "new");
        api.registerCleanup(() => cleanup.push("new"));
      },
    });

    expect(cleanup).toEqual(["old"]);
    expect(host.getSlotSnapshot("workspace.header").map((item) => item.generation)).toEqual(["new"]);
    expect(host.getCommands().map((item) => item.id)).toEqual(["new-command"]);
    expect(host.getLocaleEntries("en")).toEqual({ label: "new" });
    expect(document.head.querySelector("style[data-wuu-plugin-generation=old]")).toBeNull();
    expect(document.head.querySelector("style[data-wuu-plugin-generation=new]")).not.toBeNull();

    oldHandle.dispose();
    expect(host.getCommands()).toHaveLength(1);

    newHandle.dispose();
    expect(cleanup).toEqual(["old", "new"]);
    expect(host.getSlotSnapshot("workspace.header")).toEqual([]);
    expect(host.getCommands()).toEqual([]);
    expect(host.getLocaleEntries("en")).toEqual({});
    expect(document.head.querySelector("style[data-wuu-plugin-id=notes]")).toBeNull();
  });

  it("keeps a staged replacement private until its registration callback completes", async () => {
    const host = new PluginHost({ react: React });
    await host.activateGeneration({
      pluginId: "atomic",
      generation: "old",
      register(api) {
        api.registerCommand(command("old"));
      },
    });

    let finishRegistration: (() => void) | undefined;
    const registrationGate = new Promise<void>((resolve) => {
      finishRegistration = resolve;
    });
    const replacement = host.activateGeneration({
      pluginId: "atomic",
      generation: "new",
      async register(api) {
        api.registerCommand(command("new"));
        await registrationGate;
      },
    });

    expect(host.getCommands().map((item) => item.id)).toEqual(["old"]);
    finishRegistration?.();
    await replacement;
    expect(host.getCommands().map((item) => item.id)).toEqual(["new"]);
  });

  it("rolls back partial registration in reverse order and preserves the active generation", async () => {
    const host = new PluginHost({ react: React });
    const cleanup: string[] = [];
    await host.activateGeneration({
      pluginId: "review",
      generation: "stable",
      register(api) {
        api.registerCommand(command("stable"));
        api.registerCleanup(() => cleanup.push("stable"));
      },
    });

    const activation = host.activateGeneration({
      pluginId: "review",
      generation: "broken",
      register(api) {
        api.registerCleanup(() => cleanup.push("first"));
        api.registerSlot("sidebar.footer", contribution("partial"));
        api.registerCleanup(() => cleanup.push("second"));
        api.registerStyle({ id: "partial", css: ".partial {}" });
        throw new Error("registration stopped");
      },
    });

    await expect(activation).rejects.toThrow("registration stopped");
    expect(cleanup).toEqual(["second", "first"]);
    expect(host.getCommands().map((item) => item.id)).toEqual(["stable"]);
    expect(host.getSlotSnapshot("sidebar.footer")).toEqual([]);
    expect(document.head.querySelector("style[data-wuu-plugin-generation=broken]")).toBeNull();
    expect(host.getGenerationDiagnostics("review", "broken").map((item) => item.kind)).toEqual(["activation"]);
  });

  it("removes individual active registrations through idempotent disposables", async () => {
    const host = new PluginHost({ react: React });
    const disposables: Disposable[] = [];
    const cleanup: string[] = [];
    await host.activateGeneration({
      pluginId: "removable",
      generation: "one",
      register(api) {
        disposables.push(api.registerSlot("settings.plugin", contribution("settings")));
        disposables.push(api.registerCommand(command("action")));
        disposables.push(api.registerStyle({ id: "theme", css: ".theme {}" }));
        disposables.push(api.registerLocale({ id: "copy", locale: "en", entries: { copy: "Copy" } }));
        disposables.push(api.registerCleanup(() => cleanup.push("cleanup")));
      },
    });

    for (const disposable of disposables) {
      disposable.dispose();
      disposable.dispose();
    }

    expect(host.getSlotSnapshot("settings.plugin")).toEqual([]);
    expect(host.getCommands()).toEqual([]);
    expect(host.getLocaleEntries("en")).toEqual({});
    expect(document.head.querySelector("style[data-wuu-plugin-id=removable]")).toBeNull();
    expect(cleanup).toEqual(["cleanup"]);
  });

  it("disables a plugin by removing every host-owned registration", async () => {
    const host = new PluginHost({ react: React });
    const cleanup: string[] = [];
    await host.activateGeneration({
      pluginId: "disable-me",
      generation: "one",
      register(api) {
        registerCompleteGeneration(api, "one");
        api.registerCleanup(() => cleanup.push("disabled"));
      },
    });

    host.disable("disable-me");

    expect(host.getSlotSnapshot("workspace.header")).toEqual([]);
    expect(host.getCommands()).toEqual([]);
    expect(host.getLocaleEntries("en")).toEqual({});
    expect(document.head.querySelector("style[data-wuu-plugin-id=disable-me]")).toBeNull();
    expect(cleanup).toEqual(["disabled"]);
  });

  it("publishes workbench registrations atomically and removes them with their generation", async () => {
    const host = new PluginHost({ react: React });
    const notifications: number[] = [];
    host.subscribe(() => notifications.push(notifications.length + 1));

    await host.activateGeneration({
      pluginId: "workbench",
      generation: "one",
      register(api) {
        api.registerViewType({
          id: "dashboard",
          title: "Dashboard",
          defaultPane: "main",
          persistence: "durable",
          render: () => null,
        });
        api.registerLayoutContribution({
          id: "dashboard-pane",
          parentId: "root",
          pane: "main",
          defaultView: "dashboard",
        });
        api.registerRenderer({
          id: "result",
          category: "tool-result",
          match: "application/example",
          priority: 20,
          render: () => null,
        });
        api.registerThemeTokens({
          theme: "night",
          base: "dark",
          tokens: { "--wuu-paper": "#111" },
        });
        api.registerCSSSnippet({ id: "density", css: ":root { --plugin-density: 1; }" });
        api.registerStatusItem({ id: "ready", label: "Ready", priority: 10 });
      },
    });

    expect(host.getViewTypes().map(qualifiedId)).toEqual(["workbench:dashboard"]);
    expect(host.getLayoutContributions().map(qualifiedId)).toEqual(["workbench:dashboard-pane"]);
    expect(host.getRenderers("tool-result").map(qualifiedId)).toEqual(["workbench:result"]);
    expect(host.getThemeTokens("night").map(qualifiedId)).toEqual(["workbench:theme:night"]);
    expect(host.getStatusItems().map(qualifiedId)).toEqual(["workbench:ready"]);
    expect(document.head.querySelector("style[data-wuu-plugin-css-snippet=density]")?.textContent)
      .toContain("--plugin-density");
    expect(notifications).toHaveLength(1);

    host.disable("workbench");

    expect(host.getViewTypes()).toEqual([]);
    expect(host.getLayoutContributions()).toEqual([]);
    expect(host.getRenderers()).toEqual([]);
    expect(host.getThemeTokens()).toEqual([]);
    expect(host.getStatusItems()).toEqual([]);
    expect(document.head.querySelector("style[data-wuu-plugin-css-snippet]")).toBeNull();
    expect(notifications).toHaveLength(2);
  });
});

function contribution(id: string, order = 0) {
  return {
    id,
    order,
    render: () => React.createElement("span", null, id),
  };
}

function command(id: string, order = 0) {
  return {
    id,
    order,
    title: id,
    execute: () => id,
  };
}

function qualifiedId(item: { pluginId: string; id: string }): string {
  return `${item.pluginId}:${item.id}`;
}

function registerCompleteGeneration(api: PluginGenerationApi, label: string): void {
  api.registerSlot("workspace.header", contribution(`${label}-slot`));
  api.registerCommand(command(`${label}-command`));
  api.registerStyle({ id: `${label}-style`, css: `.${label} {}` });
  api.registerLocale({ id: `${label}-copy`, locale: "en", entries: { label } });
}
