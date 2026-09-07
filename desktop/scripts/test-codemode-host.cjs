const assert = require("node:assert/strict");
const { mkdtempSync, mkdirSync, writeFileSync, readFileSync, rmSync } = require("node:fs");
const { tmpdir } = require("node:os");
const { join } = require("node:path");
const vm = require("node:vm");
const { buildCodeModeHost } = require("./build-codemode-host.cjs");

const root = mkdtempSync(join(tmpdir(), "wuu-codemode-build-"));
try {
  const source = join(root, "host/codemode/target/release/wuu-code-mode-host");
  mkdirSync(join(root, "host/codemode/target/release"), { recursive: true });
  let revision = "first-build";
  const run = () => {
    writeFileSync(source, revision);
    return { status: 0 };
  };
  const staged = buildCodeModeHost({ repoRoot: root, platform: "darwin", run });
  assert.equal(readFileSync(staged, "utf8"), "first-build");
  revision = "updated-source";
  buildCodeModeHost({ repoRoot: root, platform: "darwin", run });
  assert.equal(readFileSync(staged, "utf8"), "updated-source");
  assert.throws(() => buildCodeModeHost({
    repoRoot: root, platform: "darwin", run: () => ({ status: 1 }),
  }), /failed/);
} finally {
  rmSync(root, { recursive: true, force: true });
}

// Execute the actual dev launcher with external processes replaced. A failed
// host build must prevent Electron from starting even when a previous build exists.
for (const fail of [false, true]) {
  let hostBuilt = false;
  let launched = false;
  const fakeRequire = (name) => {
    if (name === "node:child_process") return {
      spawnSync: () => ({ status: 0 }),
      spawn: () => {
        assert.ok(hostBuilt);
        launched = true;
        return { on() {} };
      },
    };
    if (name === "node:fs") return {
      readFileSync: () => JSON.stringify({ bin: { "electron-vite": "bin/electron-vite.js" } }),
    };
    if (name === "./build-codemode-host.cjs") return { buildCodeModeHost() {
      if (fail) throw new Error("host build failed");
      hostBuilt = true;
    } };
    if (name.startsWith("./")) return {};
    return require(name);
  };
  fakeRequire.resolve = () => "/fixture/node_modules/electron-vite/package.json";
  const execute = () => vm.runInNewContext(readFileSync(join(__dirname, "dev.cjs"), "utf8"), {
    require: fakeRequire, __dirname,
    process: { platform: "linux", env: {}, execPath: process.execPath, exit() { throw new Error("unexpected exit"); } },
    console,
  });
  if (fail) assert.throws(execute, /host build failed/);
  else execute();
  assert.equal(launched, !fail);
}
console.log("code-mode host build and dev launch checks passed");
