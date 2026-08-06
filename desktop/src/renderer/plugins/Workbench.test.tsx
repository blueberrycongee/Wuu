import * as React from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { ExtensionInventoryRecord } from "../../shared/protocol";
import { RichContent } from "../RichContent";
import { desktopPluginHost } from "./DesktopPluginRuntime";
import { DesktopWorkbench, WorkbenchController } from "./Workbench";
import { PluginHost, type PluginGenerationApi } from "./PluginHost";

describe("WorkbenchController", () => {
  beforeEach(() => {
    window.localStorage.clear();
    document.documentElement.dataset.theme = "dark";
  });

  afterEach(() => {
    window.localStorage.clear();
    document.documentElement.removeAttribute("style");
    document.documentElement.removeAttribute("data-theme");
  });

  it("maps views to every workbench pane and persists only durable state", async () => {
    const host = new PluginHost({ react: React });
    await host.activateGeneration({
      pluginId: "user:views",
      generation: "one",
      register(api) {
        api.registerViewType({
          id: "views.dashboard",
          title: "Dashboard",
          defaultPane: "main",
          persistence: "durable",
          render: () => <div>Dashboard</div>,
        });
        api.registerLayoutContribution({
          id: "default-dashboard",
          parentId: "root",
          pane: "sidebar",
          defaultView: "views.dashboard",
        });
      },
    });
    const controller = new WorkbenchController(host);
    controller.setAvailablePluginIds(new Set(["user:views"]));

    for (const pane of ["main", "sidebar", "auxiliary", "tab", "pane", "overlay"] as const) {
      await controller.openView("views.dashboard", { pane, context: { pane } });
    }

    expect(new Set(controller.getSnapshot().views.map((view) => view.pane))).toEqual(
      new Set(["main", "sidebar", "auxiliary", "tab", "pane", "overlay"]),
    );
    expect(controller.getSnapshot().views).toEqual(expect.arrayContaining([
      expect.objectContaining({ id: "layout:user:views:default-dashboard", pane: "sidebar" }),
    ]));

    const restored = new WorkbenchController(host);
    restored.setAvailablePluginIds(new Set(["user:views"]));
    expect(restored.getSnapshot().views.every((view) => view.persistence === "durable")).toBe(true);
    expect(restored.getSnapshot().views.map((view) => view.pane)).toEqual(
      expect.arrayContaining(["main", "sidebar", "auxiliary", "tab", "pane", "overlay"]),
    );
    controller.dispose();
    restored.dispose();
  });

  it("exposes controlled commands, settings, and plugin-namespaced storage", async () => {
    const execute = vi.fn((input?: unknown) => ({ accepted: input }));
    const getSetting = vi.fn((_pluginId: string, _generation: string, key: string) => key === "density" ? "compact" : null);
    const stored = new Map<string, string>();
    const getStorage = vi.fn(async (pluginId: string, _generation: string, key: string, scope: string) => stored.get(`${pluginId}:${scope}:${key}`) ?? null);
    const setStorage = vi.fn(async (pluginId: string, _generation: string, key: string, value: string, scope: string) => { stored.set(`${pluginId}:${scope}:${key}`, value); });
    const host = new PluginHost({ react: React });
    await host.activateGeneration({
      pluginId: "user:actions",
      generation: "one",
      register(api) {
        api.registerViewType({ id: "actions.view", title: "Actions", render: () => null });
        api.registerCommand({ id: "actions.run", title: "Run", execute });
        api.registerRenderer({
          id: "actions.low",
          category: "document",
          match: "text/plain",
          priority: 1,
          render: () => null,
        });
        api.registerRenderer({
          id: "actions.high",
          category: "document",
          match: "text/plain",
          priority: 10,
          render: () => null,
        });
      },
    });
    const controller = new WorkbenchController(host, { getSetting, getStorage, setStorage });
    const instanceId = await controller.openView("actions.view");
    const view = controller.getSnapshot().views.find((candidate) => candidate.id === instanceId);
    expect(view).toBeDefined();
    if (!view) return;
    const api = controller.createViewHostAPI(view);

    await api.setStorage("panel.mode", "focused");
    expect(await api.getStorage("panel.mode")).toBe("focused");
    expect(await api.getSetting("density")).toBe("compact");
    expect(getStorage).toHaveBeenCalledWith("user:actions", "one", "panel.mode", "workspace");
    expect(await api.executeCommand("actions.run", 7)).toEqual({ accepted: 7 });
    expect(execute).toHaveBeenCalledWith(7);
    expect(controller.getRenderer("document", "text/plain")?.id).toBe("actions.high");
    await expect(api.getStorage("../private")).rejects.toThrow("storage key is invalid");

    await host.activateGeneration({
      pluginId: "user:other",
      generation: "one",
      register(api) {
        api.registerViewType({ id: "other.view", title: "Other", render: () => null });
      },
    });
    const otherId = await controller.openView("other.view");
    const other = controller.getSnapshot().views.find((candidate) => candidate.id === otherId);
    expect(other).toBeDefined();
    if (other) expect(await controller.createViewHostAPI(other).getStorage("panel.mode")).toBeNull();
    host.unload("user:actions");
    await expect(api.getSetting("density")).rejects.toThrow("no longer active");
    controller.dispose();
  });

  it("keeps colliding view IDs bound to their plugin across layouts, persistence, and reloads", async () => {
    const host = new PluginHost({ react: React });
    await host.activateGeneration({
      pluginId: "user:alpha",
      generation: "one",
      register: (api) => registerCollidingGeneration(api, "alpha", "one"),
    });
    await host.activateGeneration({
      pluginId: "user:beta",
      generation: "one",
      register: (api) => registerCollidingGeneration(api, "beta", "one"),
    });
    const availablePlugins = new Set(["user:alpha", "user:beta"]);
    const controller = new WorkbenchController(host);
    controller.setAvailablePluginIds(availablePlugins);

    expect(controller.getSnapshot().views).toEqual(expect.arrayContaining([
      expect.objectContaining({
        id: "layout:user:alpha:default-shared",
        pluginId: "user:alpha",
        generation: "one",
      }),
      expect.objectContaining({
        id: "layout:user:beta:default-shared",
        pluginId: "user:beta",
        generation: "one",
      }),
    ]));
    await expect(controller.openView("shared.view")).rejects.toThrow("view type is ambiguous");

    const alphaLauncherId = await controller.openView("alpha.launcher");
    const alphaLauncher = controller.getSnapshot().views.find((view) => view.id === alphaLauncherId);
    expect(alphaLauncher).toBeDefined();
    if (!alphaLauncher) return;
    const alphaHost = controller.createViewHostAPI(alphaLauncher);
    await alphaHost.openView("shared.view", { pane: "main" });
    await alphaHost.openView("beta.unique", { pane: "pane" });
    expect(controller.getSnapshot().views).toEqual(expect.arrayContaining([
      expect.objectContaining({ viewTypeId: "shared.view", pluginId: "user:alpha", pane: "main" }),
      expect.objectContaining({ viewTypeId: "beta.unique", pluginId: "user:beta", pane: "pane" }),
    ]));

    const restored = new WorkbenchController(host);
    restored.setAvailablePluginIds(availablePlugins);
    expect(restored.getSnapshot().views.filter((view) => view.viewTypeId === "shared.view")).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ pluginId: "user:alpha", generation: "one" }),
        expect.objectContaining({ pluginId: "user:beta", generation: "one" }),
      ]),
    );

    await host.activateGeneration({
      pluginId: "user:alpha",
      generation: "two",
      register: (api) => registerCollidingGeneration(api, "alpha", "two"),
    });
    const reloadedSharedViews = restored.getSnapshot().views.filter((view) =>
      view.viewTypeId === "shared.view");
    expect(new Set(reloadedSharedViews
      .filter((view) => view.pluginId === "user:alpha")
      .map((view) => view.generation))).toEqual(new Set(["two"]));
    expect(new Set(reloadedSharedViews
      .filter((view) => view.pluginId === "user:beta")
      .map((view) => view.generation))).toEqual(new Set(["one"]));

    controller.dispose();
    restored.dispose();
  });

  it("reconciles generation resources without retaining stale views, renderers, tokens, or status", async () => {
    const host = new PluginHost({ react: React });
    await host.activateGeneration({
      pluginId: "user:reload",
      generation: "one",
      register: (api) => registerGeneration(api, "one"),
    });
    const controller = new WorkbenchController(host);
    controller.setAvailablePluginIds(new Set(["user:reload"]));
    const instanceId = await controller.openView("reload.view", { pane: "main" });
    expect(document.documentElement.style.getPropertyValue("--wuu-plugin-accent")).toBe("one");
    expect(document.documentElement.style.getPropertyValue("--wuu-font-family-ui")).toBe("one-ui");
    expect(document.documentElement.style.getPropertyValue("--wuu-syntax-keyword")).toBe("one-keyword");
    expect(controller.getRenderer("document", "text/demo")?.generation).toBe("one");
    expect(host.getStatusItems().map((item) => item.label)).toEqual(["one"]);

    await host.activateGeneration({
      pluginId: "user:reload",
      generation: "two",
      register: (api) => registerGeneration(api, "two"),
    });
    expect(controller.getSnapshot().views.find((view) => view.id === instanceId)?.generation).toBe("two");
    expect(controller.getRenderer("document", "text/demo")?.generation).toBe("two");
    expect(document.documentElement.style.getPropertyValue("--wuu-plugin-accent")).toBe("two");
    expect(document.documentElement.style.getPropertyValue("--wuu-font-family-ui")).toBe("two-ui");
    expect(host.getStatusItems().map((item) => item.label)).toEqual(["two"]);

    host.unload("user:reload");
    expect(controller.getSnapshot().views).toEqual([]);
    expect(controller.getRenderer("document", "text/demo")).toBeUndefined();
    expect(document.documentElement.style.getPropertyValue("--wuu-plugin-accent")).toBe("");
    expect(document.documentElement.style.getPropertyValue("--wuu-font-family-ui")).toBe("");
    expect(document.documentElement.style.getPropertyValue("--wuu-syntax-keyword")).toBe("");
    expect(host.getStatusItems()).toEqual([]);
    controller.dispose();
  });
});

describe("DesktopWorkbench product path", () => {
  let root: Root;
  let container: HTMLDivElement;

  beforeEach(() => {
    window.localStorage.clear();
    container = document.createElement("div");
    container.innerHTML = '<aside class="sidebar"></aside><main class="conversation-pane"></main>';
    document.body.appendChild(container);
    const workbenchRoot = document.createElement("div");
    workbenchRoot.dataset.workbenchRoot = "true";
    container.appendChild(workbenchRoot);
    root = createRoot(workbenchRoot);
  });

  afterEach(() => {
    act(() => root.unmount());
    container.remove();
    document.querySelectorAll(".plugin-workbench-status").forEach((item) => item.remove());
  });

  it("renders layout defaults through the host workbench and restores built-in UI after unload", async () => {
    const host = new PluginHost({ react: React });
    await host.activateGeneration({
      pluginId: "user:product",
      generation: "one",
      register(api) {
        api.registerViewType({ id: "product.view", title: "Product", render: () => <div>Plugin product view</div> });
        api.registerLayoutContribution({
          id: "product-main",
          parentId: "root",
          pane: "main",
          defaultView: "product.view",
        });
        api.registerStatusItem({ id: "ready", label: "Plugin ready" });
      },
    });

    await act(async () => root.render(
      <DesktopWorkbench host={host} inventory={[inventoryPlugin("user:product")]} />,
    ));
    expect(container.querySelector(".conversation-pane")?.textContent).toContain("Plugin product view");
    expect(document.body.textContent).toContain("Plugin ready");

    await act(async () => host.unload("user:product"));
    expect(container.querySelector(".conversation-pane")?.textContent).not.toContain("Plugin product view");
    expect(document.body.textContent).not.toContain("Plugin ready");
  });

  it("renders colliding layout view IDs with each plugin's own definition", async () => {
    const host = new PluginHost({ react: React });
    await host.activateGeneration({
      pluginId: "user:alpha",
      generation: "one",
      register: (api) => registerCollidingGeneration(api, "alpha", "one"),
    });
    await host.activateGeneration({
      pluginId: "user:beta",
      generation: "one",
      register: (api) => registerCollidingGeneration(api, "beta", "one"),
    });

    await act(async () => root.render(
      <DesktopWorkbench
        host={host}
        inventory={[inventoryPlugin("user:alpha"), inventoryPlugin("user:beta")]}
      />,
    ));
    expect(container.querySelector(".sidebar")?.textContent).toContain("alpha");
    expect(container.querySelector(".conversation-pane")?.textContent).toContain("beta");
  });

  it("keeps settings, disable, and built-in UI escape actions available after a render failure", async () => {
    const host = new PluginHost({ react: React });
    const openSettings = vi.fn();
    const disablePlugin = vi.fn();
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => undefined);
    await host.activateGeneration({
      pluginId: "user:broken",
      generation: "one",
      register(api) {
        api.registerViewType({
          id: "broken.view",
          title: "Broken",
          render: () => { throw new Error("render failed"); },
        });
        api.registerLayoutContribution({
          id: "broken-main",
          parentId: "root",
          pane: "main",
          defaultView: "broken.view",
        });
      },
    });

    await act(async () => root.render(
      <DesktopWorkbench
        host={host}
        inventory={[inventoryPlugin("user:broken")]}
        services={{ openSettings, disablePlugin }}
      />,
    ));
    const buttons = [...container.querySelectorAll<HTMLButtonElement>(".plugin-workbench-error button")];
    expect(buttons.map((button) => button.textContent)).toEqual([
      "Use default UI",
      "Open settings",
      "Disable plugin",
    ]);
    act(() => buttons[1]?.click());
    act(() => buttons[2]?.click());
    expect(openSettings).toHaveBeenCalledOnce();
    expect(disablePlugin).toHaveBeenCalledWith("user:broken");
    act(() => buttons[0]?.click());
    expect(container.querySelector(".plugin-workbench-error")).toBeNull();
    consoleError.mockRestore();
  });

  it("uses registered message renderers and releases them with their generation", async () => {
    await desktopPluginHost.activateGeneration({
      pluginId: "user:renderer",
      generation: "one",
      register(api) {
        api.registerRenderer({
          id: "markdown",
          category: "message",
          match: "text/markdown",
          render: ({ content }) => <div>Plugin markdown: {String(content)}</div>,
        });
      },
    });

    await act(async () => root.render(<RichContent text="hello" />));
    expect(container.textContent).toContain("Plugin markdown: hello");

    await act(async () => desktopPluginHost.unload("user:renderer"));
    expect(container.textContent).not.toContain("Plugin markdown");
    expect(container.textContent).toContain("hello");
  });
});

function registerGeneration(api: PluginGenerationApi, label: string): void {
  api.registerViewType({
    id: "reload.view",
    title: `Reload ${label}`,
    persistence: "durable",
    render: () => <div>{label}</div>,
  });
  api.registerRenderer({
    id: "reload.renderer",
    category: "document",
    match: "text/demo",
    render: () => <div>{label}</div>,
  });
  api.registerThemeTokens({
    theme: "dark",
    base: "dark",
    tokens: {
      "--wuu-plugin-accent": label,
      "--wuu-font-family-ui": `${label}-ui`,
    },
    syntax: { "--wuu-syntax-keyword": `${label}-keyword` },
  });
  api.registerStatusItem({ id: "reload.status", label });
}

function registerCollidingGeneration(
  api: PluginGenerationApi,
  label: "alpha" | "beta",
  generation: string,
): void {
  api.registerViewType({
    id: "shared.view",
    title: `${label} ${generation}`,
    persistence: "durable",
    render: () => <div>{label}</div>,
  });
  api.registerViewType({
    id: `${label}.launcher`,
    title: `${label} launcher`,
    render: () => null,
  });
  if (label === "beta") {
    api.registerViewType({
      id: "beta.unique",
      title: "Beta unique",
      persistence: "durable",
      render: () => null,
    });
  }
  api.registerLayoutContribution({
    id: "default-shared",
    parentId: "root",
    pane: label === "alpha" ? "sidebar" : "auxiliary",
    defaultView: "shared.view",
  });
}

function inventoryPlugin(id: string): ExtensionInventoryRecord {
  return {
    id,
    name: id,
    kind: "plugin",
    provenance: { kind: "plugin", source: "user", scope: "user" },
    state: "granted",
    approval_state: "granted",
    enabled: true,
  };
}
