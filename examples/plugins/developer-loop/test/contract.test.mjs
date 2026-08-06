import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";

const manifest = JSON.parse(await readFile(new URL("../plugin.json", import.meta.url), "utf8"));
const packageJSON = JSON.parse(await readFile(new URL("../package.json", import.meta.url), "utf8"));
const source = await readFile(new URL("../src/renderer.ts", import.meta.url), "utf8");
const output = await readFile(new URL("../dist/renderer.js", import.meta.url), "utf8");
const renderer = await import(new URL("../dist/renderer.js", import.meta.url));

assert.equal(packageJSON.devDependencies["@wuu/plugin-sdk"], "^0.1.0");
assert.match(source, /from "@wuu\/plugin-sdk"/);
assert.doesNotMatch(source, /(?:desktop|internal)\/src|\.\.\/\.\.\/\.\.\/packages/);
assert.doesNotMatch(source, /WorkbenchController|PluginHost|RegistryImpl/);
assert.doesNotMatch(output, /react(?:-dom)?["'/]|node_modules\/react/);

for (const registration of [
  "registerViewType",
  "registerViewPlacement",
  "registerThemeTokens",
  "registerCSSSnippet",
  "registerCommand",
  "registerStatusItem",
  "registerLocale",
  "registerSlot",
  "registerPresenter",
  "registerToolActivityPresenter",
]) {
  assert.match(source, new RegExp(`api\\.${registration}\\(`), `missing ${registration}`);
}

for (const family of [
  "color",
  "font",
  "space",
  "radius",
  "border",
  "elevation",
  "motion",
  "syntax",
  "content",
]) {
  assert.match(source, new RegExp(`--wuu-${family}-`), `missing --wuu-${family}- token`);
}

assert.match(source, /persistence: "durable"/);
assert.match(source, /host\.getSetting\(/);
assert.match(source, /host\.getStorage\(STORAGE_KEY\)/);
assert.match(source, /host\.setStorage\(STORAGE_KEY/);

for (const type of ["boolean", "string", "number", "enum"]) {
  assert.ok(
    Object.values(manifest.contributes.settings).some((setting) => setting.type === type),
    `missing ${type} setting`,
  );
}

const registrations = new Map();
const disposables = [];
const register = (kind) => (value) => {
  const values = registrations.get(kind) ?? [];
  values.push(value);
  registrations.set(kind, values);
  const disposable = {
    dispose() {
      const index = values.indexOf(value);
      if (index >= 0) values.splice(index, 1);
    },
  };
  disposables.push(disposable);
  return disposable;
};
const createElement = (type, props, ...children) => ({ type, props: props ?? {}, children });
const api = {
  pluginId: manifest.id,
  generation: "acceptance-generation",
  react: { Fragment: Symbol("Fragment"), createElement },
  registerViewType: register("views"),
  registerViewPlacement: register("viewPlacements"),
  // Kept in the stub so old bundles can still be exercised by this host.
  registerLayoutContribution: register("layouts"),
  registerThemeTokens: register("themes"),
  registerCSSSnippet: register("css"),
  registerCommand: register("commands"),
  registerStatusItem: register("status"),
  registerLocale: register("locales"),
  registerSlot: (_slot, contribution) => register("slots")(contribution),
  registerPresenter: register("presenters"),
  registerToolActivityPresenter: register("toolActivityPresenters"),
};

const activateGeneration = (activate) => {
  const generationStart = disposables.length;
  try {
    activate(api);
  } catch (error) {
    for (const disposable of disposables.splice(generationStart).reverse()) disposable.dispose();
    throw error;
  }
  const owned = disposables.slice(generationStart);
  return () => {
    for (const disposable of owned.reverse()) disposable.dispose();
  };
};

const unloadGeneration = activateGeneration(renderer.activate);
for (const [kind, expected] of Object.entries({
  views: 1,
  viewPlacements: 1,
  themes: 1,
  css: 1,
  commands: 1,
  status: 1,
  locales: 2,
  slots: 1,
  presenters: 6,
  toolActivityPresenters: 1,
})) {
  assert.equal(registrations.get(kind)?.length, expected, `${kind} did not activate`);
}

const presenter = registrations.get("toolActivityPresenters")[0];
assert.equal(presenter.key, "developer-loop.tool.echo");
const presented = presenter.render({
  activity: {
    id: "call-1",
    toolName: "developer-loop-echo",
    capability: presenter.key,
    status: "completed",
    argumentsText: "{}",
    resultText: "developer-loop tool ok",
  },
  host: {},
  fallback: createElement("span", null, "native"),
});
assert.equal(presented.type, "section");
assert.equal(presented.props["data-developer-loop-tool"], "completed");
assert.equal(presented.props["data-tool-id"], "call-1");
assert.equal(presented.children[1].children[0], "developer-loop tool ok");

const presenterByTarget = new Map(registrations.get("presenters").map((definition) => [definition.target, definition]));
assert.deepEqual(
  registrations.get("presenters").map(({ target, key, mode }) => ({ target, key, mode })),
  [
    { target: "conversation.item", key: "assistant-message", mode: "wrap" },
    { target: "conversation.composer", key: undefined, mode: "wrap" },
    { target: "navigation.primary", key: undefined, mode: "replace" },
    { target: "content.preview", key: "text/markdown", mode: "wrap" },
    { target: "app.status", key: undefined, mode: "replace" },
    { target: "header.conversation", key: undefined, mode: "wrap" },
  ],
);

const invokedActions = [];
const presentationHost = {
  actions: ["conversation.composer.submit", "navigation.activate-node", "status.activate-item"],
  invoke: async (action, input) => {
    invokedActions.push([action, input]);
    return { accepted: true };
  },
};
const fallback = createElement("span", { "data-native-fallback": "" }, "native");
const renderPresenter = (target, snapshot, key) => {
  const definition = presenterByTarget.get(target);
  assert.equal(definition.key, key);
  return definition.render({ contractVersion: 1, target, key, snapshot, host: presentationHost, fallback });
};

const itemOutput = renderPresenter("conversation.item", {
  contractVersion: 1,
  id: "message-1",
  kind: "assistant-message",
  status: "completed",
  text: "Accepted answer",
}, "assistant-message");
assert.equal(itemOutput.props["data-item-id"], "message-1");
assert.equal(itemOutput.props["data-item-status"], "completed");
assert.equal(itemOutput.children[1], fallback);

const composerOutput = renderPresenter("conversation.composer", {
  contractVersion: 1,
  threadId: "thread-1",
  draftText: "Ship it",
  activeSubmissionMode: "send",
  running: false,
});
assert.equal(composerOutput.props["data-thread-id"], "thread-1");
assert.equal(composerOutput.children[0], fallback);
await composerOutput.children[1].props.onClick();

const navigationOutput = renderPresenter("navigation.primary", {
  contractVersion: 1,
  activeNodeId: "thread-1",
  nodes: [{ id: "thread-1", kind: "thread", label: "Acceptance thread", active: true }],
});
assert.equal(navigationOutput.props["data-node-count"], "1");
await navigationOutput.children[0].props.onClick();

const previewOutput = renderPresenter("content.preview", {
  contractVersion: 1,
  resourceId: "resource-1",
  workspaceRelativePath: "acceptance.md",
  contentType: "text/markdown",
  text: "# Accepted",
  readOnly: true,
}, "text/markdown");
assert.equal(previewOutput.props["data-content-type"], "text/markdown");
assert.equal(previewOutput.children[2], fallback);

const statusOutput = renderPresenter("app.status", {
  contractVersion: 1,
  items: [{ id: "ready", label: "Ready", kind: "success", actionId: "open" }],
});
assert.equal(statusOutput.children[0].props["data-status-kind"], "success");
await statusOutput.children[0].props.onClick();

const headerOutput = renderPresenter("header.conversation", {
  contractVersion: 1,
  scope: "conversation",
  title: "Acceptance conversation",
  activeTabId: "tab-1",
});
assert.equal(headerOutput.props["data-active-tab"], "tab-1");
assert.equal(headerOutput.children[1], fallback);
assert.deepEqual(invokedActions, [
  ["conversation.composer.submit", undefined],
  ["navigation.activate-node", { id: "thread-1" }],
  ["status.activate-item", { id: "ready" }],
]);
assert.equal(await renderer.invokePresentationAction(presentationHost, "unsupported.action"), undefined);

const storedWrites = [];
const host = {
  getStorage: async (key) => {
    assert.equal(key, "counter");
    return "4";
  },
  setStorage: async (key, value) => storedWrites.push([key, value]),
  getSetting: async (key) => ({ enabled: true, label: "Verified", step: 3, density: "compact" })[key],
  executeCommand: async () => undefined,
  openView: async () => undefined,
};
const nodes = new Map([
  ["[data-counter-value]", { textContent: "" }],
  ["[data-counter-label]", { textContent: "" }],
  ["[data-counter-button]", { textContent: "", disabled: false }],
]);
const root = { dataset: {}, querySelector: (selector) => nodes.get(selector) ?? null };
const view = registrations.get("views")[0].render({ host });
view.props.ref(root);
await new Promise((resolve) => setTimeout(resolve, 0));
assert.equal(nodes.get("[data-counter-value]").textContent, "4");
assert.equal(nodes.get("[data-counter-label]").textContent, "Verified");
assert.equal(root.dataset.density, "compact");
const button = view.children.find((child) => child?.type === "button");
await button.props.onClick();
assert.deepEqual(storedWrites, [["counter", "7"]]);
assert.equal(nodes.get("[data-counter-value]").textContent, "7");

const countsBeforeFailure = new Map([...registrations].map(([kind, values]) => [kind, values.length]));
assert.throws(
  () => activateGeneration((failedApi) => {
    failedApi.registerPresenter({ id: "failed", target: "app.status", render: () => null });
    throw new Error("candidate activation failed");
  }),
  /candidate activation failed/,
);
for (const [kind, values] of registrations) {
  assert.equal(values.length, countsBeforeFailure.get(kind), `${kind} changed after failed generation rollback`);
}

unloadGeneration();
for (const [kind, values] of registrations) {
  assert.equal(values.length, 0, `${kind} leaked after generation disposal`);
}

console.log("developer-loop public SDK contract ok");
