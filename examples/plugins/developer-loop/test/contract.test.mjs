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
  "registerLayoutContribution",
  "registerThemeTokens",
  "registerCSSSnippet",
  "registerCommand",
  "registerStatusItem",
  "registerLocale",
  "registerSlot",
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
  react: { createElement },
  registerViewType: register("views"),
  registerLayoutContribution: register("layouts"),
  registerThemeTokens: register("themes"),
  registerCSSSnippet: register("css"),
  registerCommand: register("commands"),
  registerStatusItem: register("status"),
  registerLocale: register("locales"),
  registerSlot: (_slot, contribution) => register("slots")(contribution),
};

renderer.activate(api);
for (const [kind, expected] of Object.entries({
  views: 1,
  layouts: 1,
  themes: 1,
  css: 1,
  commands: 1,
  status: 1,
  locales: 2,
  slots: 1,
})) {
  assert.equal(registrations.get(kind)?.length, expected, `${kind} did not activate`);
}

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

for (const disposable of disposables.reverse()) disposable.dispose();
for (const [kind, values] of registrations) {
  assert.equal(values.length, 0, `${kind} leaked after generation disposal`);
}

console.log("developer-loop public SDK contract ok");
