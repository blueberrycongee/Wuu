import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";

const manifest = JSON.parse(await readFile(new URL("../plugin.json", import.meta.url), "utf8"));
const packageJSON = JSON.parse(await readFile(new URL("../package.json", import.meta.url), "utf8"));
const source = await readFile(new URL("../src/renderer.ts", import.meta.url), "utf8");
const output = await readFile(new URL("../dist/renderer.js", import.meta.url), "utf8");

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

console.log("developer-loop public SDK contract ok");
