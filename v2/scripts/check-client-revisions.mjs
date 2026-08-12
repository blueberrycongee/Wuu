import { createHash } from "node:crypto";
import { readFile, readdir } from "node:fs/promises";
import { dirname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";

const root = dirname(dirname(fileURLToPath(import.meta.url)));
const generatedPath = join(root, "profiles", "default", "src", "client-revisions.json");
const packages = {
  "theme-default": "plugins/theme-default/src",
  workbench: "plugins/workbench/src",
  history: "plugins/history/src",
  layout: "plugins/layout/src",
  conversation: "plugins/conversation/src",
  composer: "plugins/composer/src",
  slash: "plugins/slash/src",
  "model-session": "plugins/model-session/src",
  "permission-session": "plugins/permission-session/src",
  side: "plugins/side/src",
};

async function files(directory) {
  const result = [];
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) result.push(...await files(path));
    else if (entry.isFile()) result.push(path);
  }
  return result.sort();
}

async function digest(directory) {
  const hash = createHash("sha256");
  for (const path of await files(directory)) {
    hash.update(relative(directory, path));
    hash.update("\0");
    hash.update(await readFile(path));
    hash.update("\0");
  }
  return `sha256:${hash.digest("hex")}`;
}

const actual = Object.fromEntries(await Promise.all(
  Object.entries(packages).map(async ([id, path]) => [id, await digest(join(root, path))]),
));
if (process.argv.includes("--print")) {
  process.stdout.write(`${JSON.stringify(actual, null, 2)}\n`);
} else {
  const expected = JSON.parse(await readFile(generatedPath, "utf8"));
  if (JSON.stringify(actual) !== JSON.stringify(expected)) {
    throw new Error("default client revisions are stale; regenerate profiles/default/src/client-revisions.json");
  }
}
