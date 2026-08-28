import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ExtensionInventoryRecord, WuuDesktopApi } from "../shared/protocol";
import {
  FirstRunOnboarding,
  bundledOnboardingPlugins,
  hasOnboardingProvider,
} from "./FirstRunOnboarding";
import { I18nProvider } from "./i18n";

function plugin(
  id: string,
  enabled: boolean,
  source: "bundled" | "user" = "bundled",
): ExtensionInventoryRecord {
  return {
    id: `plugin:${source}:${id}`,
    name: id,
    description: `${id} description`,
    kind: "plugin",
    state: "active",
    enabled,
    fingerprint: `${id}-fingerprint`,
    icon: { name: "plug" },
    package_source: source,
    provenance: {
      kind: "plugin",
      source,
      scope: source,
      official: source === "bundled",
      plugin_id: id,
    },
  };
}

describe("FirstRunOnboarding", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
    window.wuu = {
      initialLanguagePreference: "zh-CN",
      initialSystemLocale: "zh-CN",
      initialThemePreference: "dark",
    } as unknown as WuuDesktopApi;
    document.documentElement.dataset.theme = "dark";
  });

  afterEach(() => {
    act(() => root.unmount());
    container.remove();
    vi.restoreAllMocks();
  });

  it("offers only known official plugins distributed with Wuu", () => {
    const inventory = [
      plugin("todo", true),
      plugin("ask-user", false),
      plugin("herbarium", true, "user"),
      plugin("peers", false),
    ];

    expect(bundledOnboardingPlugins(inventory).map((item) => item.id)).toEqual([
      "plugin:bundled:ask-user",
      "plugin:bundled:todo",
    ]);
  });

  it("recognizes configured and locked model providers", () => {
    expect(hasOnboardingProvider([{ name: "a", type: "x", model: "m" }])).toBe(false);
    expect(hasOnboardingProvider([{ name: "a", type: "x", model: "m", api_key_configured: true }])).toBe(true);
    expect(hasOnboardingProvider([{ name: "a", type: "x", model: "m", connection_locked: true }])).toBe(true);
  });

  it("selects the recommended subject IDs when inventory arrives after mount", async () => {
    const props = {
      providers: [],
      onUpdateExtensionPackage: vi.fn(async () => undefined),
      onSaveProvider: vi.fn(async () => undefined),
      onComplete: vi.fn(async () => undefined),
    };

    await act(async () => {
      root.render(
        <I18nProvider>
          <FirstRunOnboarding {...props} />
        </I18nProvider>,
      );
    });
    await clickButton("开始设置");
    expect(container.textContent).toContain("正在准备随包插件");

    await act(async () => {
      root.render(
        <I18nProvider>
          <FirstRunOnboarding
            {...props}
            inventory={[plugin("ask-user", true), plugin("memory", false)]}
          />
        </I18nProvider>,
      );
    });

    expect(container.textContent).not.toContain("正在准备随包插件");
    expect(container.querySelectorAll(".onboarding-plugin.is-selected")).toHaveLength(1);
    expect(container.querySelector(".onboarding-presets .is-selected")?.textContent).toBe("推荐");
  });

  it("loads manifest icon assets with the plugin subject ID", async () => {
    const todo = plugin("todo", true);
    todo.icon = { path: "assets/icon.svg" };
    const loadPluginIcon = vi.fn(async (params) => ({
      ...params,
      url: "data:image/svg+xml;base64,PHN2Zy8+",
    }));
    window.wuu.loadPluginIcon = loadPluginIcon;

    await act(async () => {
      root.render(
        <I18nProvider>
          <FirstRunOnboarding
            inventory={[todo]}
            providers={[]}
            onUpdateExtensionPackage={vi.fn(async () => undefined)}
            onSaveProvider={vi.fn(async () => undefined)}
            onComplete={vi.fn(async () => undefined)}
          />
        </I18nProvider>,
      );
    });
    await clickButton("开始设置");

    expect(loadPluginIcon).toHaveBeenCalledWith({
      id: "plugin:bundled:todo",
      fingerprint: "todo-fingerprint",
      path: "assets/icon.svg",
    });
  });

  it("gates entry until explicit plugin choices are applied and completion is persisted", async () => {
    const update = vi.fn(async () => undefined);
    const complete = vi.fn(async () => undefined);

    await act(async () => {
      root.render(
        <I18nProvider>
          <FirstRunOnboarding
            inventory={[plugin("ask-user", true), plugin("memory", false)]}
            providers={[]}
            onUpdateExtensionPackage={update}
            onSaveProvider={vi.fn(async () => undefined)}
            onComplete={complete}
          />
        </I18nProvider>,
      );
    });

    expect(container.querySelector('[data-testid="first-run-onboarding"]')).not.toBeNull();
    expect(container.textContent).toContain("保持简单，按需生长");
    expect(container.textContent).not.toContain("欢迎来到 Wuu");
    expect(container.textContent).not.toContain("Wuu 的核心负责可靠地运行 Agent");
    expect(document.documentElement.dataset.theme).toBe("light");

    await clickButton("开始设置");
    await clickButton("极简");
    await clickButton("继续");

    expect(update).toHaveBeenCalledTimes(1);
    expect(update).toHaveBeenCalledWith({ id: "plugin:bundled:ask-user", action: "disable" });
    expect(complete).not.toHaveBeenCalled();

    await clickButton("稍后配置");
    expect(container.textContent).toContain("你的 Wuu 已经准备好了");
    expect(complete).not.toHaveBeenCalled();

    await clickButton("开始使用 Wuu");
    expect(complete).toHaveBeenCalledTimes(1);

    act(() => root.unmount());
    expect(document.documentElement.dataset.theme).toBe("dark");
    root = createRoot(container);
  });

  async function clickButton(label: string): Promise<void> {
    const button = [...container.querySelectorAll("button")].find(
      (candidate) => candidate.textContent?.trim() === label,
    );
    expect(button, `missing button ${label}`).toBeDefined();
    await act(async () => {
      button?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    });
  }
});
