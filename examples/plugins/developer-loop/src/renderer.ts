import type { PluginGenerationApi, ViewHostAPI } from "@wuu/plugin-sdk";

const VIEW_ID = "acceptance-counter";
const COMMAND_ID = "open-acceptance-counter";
const STORAGE_KEY = "counter";
let activeViewHost: ViewHostAPI | undefined;

type ViewElement = {
  querySelector(selector: string): ViewElement | null;
  textContent: string | null;
  disabled?: boolean;
  dataset?: Record<string, string | undefined>;
};

interface AcceptanceSettings {
  enabled: boolean;
  label: string;
  step: number;
  density: string;
}

function acceptanceView(api: PluginGenerationApi, props: Readonly<Record<string, unknown>>): unknown {
  const host = (props as unknown as { host: ViewHostAPI }).host;
  activeViewHost = host;
  let count = 0;
  let settings: AcceptanceSettings = {
    enabled: true,
    label: "Accepted runs",
    step: 1,
    density: "comfortable",
  };

  const update = (root: ViewElement): void => {
    const value = root.querySelector("[data-counter-value]");
    const label = root.querySelector("[data-counter-label]");
    const button = root.querySelector("[data-counter-button]");
    if (value) value.textContent = String(count);
    if (label) label.textContent = settings.label;
    if (button) button.disabled = !settings.enabled;
    if (root.dataset) root.dataset.density = settings.density;
  };

  const restore = async (root: ViewElement): Promise<void> => {
    const [stored, enabled, label, step, density] = await Promise.all([
      host.getStorage(STORAGE_KEY),
      host.getSetting("enabled"),
      host.getSetting("label"),
      host.getSetting("step"),
      host.getSetting("density"),
    ]);
    const restored = Number.parseInt(stored ?? "0", 10);
    count = Number.isFinite(restored) ? restored : 0;
    settings = {
      enabled: typeof enabled === "boolean" ? enabled : true,
      label: typeof label === "string" ? label : "Accepted runs",
      step: typeof step === "number" && Number.isFinite(step) ? step : 1,
      density: typeof density === "string" ? density : "comfortable",
    };
    update(root);
  };

  let root: ViewElement | null = null;
  return api.react.createElement(
    "section",
    {
      className: "developer-loop-card",
      ref: (node: ViewElement | null) => {
        root = node;
        if (node) void restore(node);
      },
    },
    api.react.createElement("p", { className: "developer-loop-eyebrow" }, "SDK v2 acceptance"),
    api.react.createElement("h2", { "data-counter-label": "" }, settings.label),
    api.react.createElement("strong", { "data-counter-value": "" }, "0"),
    api.react.createElement(
      "button",
      {
        type: "button",
        "data-counter-button": "",
        onClick: async () => {
          if (!settings.enabled) return;
          count += settings.step;
          await host.setStorage(STORAGE_KEY, String(count));
          if (root) update(root);
        },
      },
      "Increment and save",
    ),
  );
}

export function activate(api: PluginGenerationApi): void {
  api.registerViewType({
    id: VIEW_ID,
    title: "Acceptance Counter",
    icon: "check-circle",
    defaultPane: "auxiliary",
    persistence: "durable",
    render: (props: Readonly<Record<string, unknown>>) => acceptanceView(api, props),
  });
  api.registerLayoutContribution({
    id: "acceptance-tools",
    parentId: "root",
    pane: "auxiliary",
    size: 320,
    minSize: 240,
    defaultView: VIEW_ID,
  });
  api.registerThemeTokens({
    theme: "developer-focus-complete",
    base: "dark",
    tokens: {
      "--wuu-color-canvas": "#111827",
      "--wuu-color-text": "#f9fafb",
      "--wuu-font-family-ui": "ui-sans-serif, system-ui, sans-serif",
      "--wuu-font-size-body": "14px",
      "--wuu-space-density": "0.875",
      "--wuu-space-unit": "4px",
      "--wuu-radius-control": "8px",
      "--wuu-border-subtle": "1px solid #374151",
      "--wuu-elevation-panel": "0 12px 32px rgb(0 0 0 / 0.28)",
      "--wuu-motion-duration-fast": "120ms",
      "--wuu-motion-easing-standard": "cubic-bezier(0.2, 0, 0, 1)",
      "--wuu-content-max-width": "72rem",
    },
    syntax: {
      "--wuu-syntax-keyword": "#c084fc",
      "--wuu-syntax-function": "#67e8f9",
      "--wuu-syntax-string": "#86efac",
      "--wuu-syntax-comment": "#9ca3af",
    },
  });
  api.registerCSSSnippet({
    id: "acceptance-counter-card",
    priority: 10,
    css: `.developer-loop-card {
  max-width: var(--wuu-content-max-width, 72rem);
  padding: calc(var(--wuu-space-unit, 4px) * 4);
  color: var(--wuu-color-text, inherit);
  border: var(--wuu-border-subtle, 1px solid currentColor);
  border-radius: var(--wuu-radius-control, 8px);
  box-shadow: var(--wuu-elevation-panel, none);
  font-family: var(--wuu-font-family-ui, sans-serif);
}
.developer-loop-card[data-density="compact"] { padding: calc(var(--wuu-space-unit, 4px) * 2); }
.developer-loop-eyebrow { color: var(--wuu-ink-soft, currentColor); }`,
  });
  api.registerCommand({
    id: COMMAND_ID,
    title: "Open Acceptance Counter",
    async execute() {
      if (!activeViewHost) {
        return { viewTypeId: VIEW_ID, pane: "auxiliary", persistence: "durable" };
      }
      await activeViewHost.openView(VIEW_ID, {
        pane: "auxiliary",
        persistence: "durable",
        reveal: true,
      });
    },
  });
  api.registerStatusItem({
    id: "acceptance-ready",
    label: "Acceptance ready",
    icon: "check",
    tooltip: "Open the persistent SDK acceptance counter",
    command: COMMAND_ID,
    priority: 50,
  });
  api.registerLocale({
    id: "acceptance-en",
    locale: "en",
    entries: {
      "developerLoop.counter.title": "Acceptance Counter",
      "developerLoop.counter.increment": "Increment and save",
    },
  });
  api.registerLocale({
    id: "acceptance-zh-cn",
    locale: "zh-CN",
    entries: {
      "developerLoop.counter.title": "验收计数器",
      "developerLoop.counter.increment": "增加并保存",
    },
  });
  api.registerSlot("conversation.header", {
    id: "developer-loop-status",
    render() {
      return api.react.createElement("span", null, `Plugin generation ${api.generation}`);
    },
  });
}
