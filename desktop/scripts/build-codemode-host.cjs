// Dev and release stage the same incrementally rebuilt runtime next to wuu-core.
// Cargo owns freshness; an existing staged binary must never mask source changes.
const { copyFileSync, mkdirSync, existsSync, statSync, chmodSync } = require("node:fs");
const { join, resolve } = require("node:path");
const { spawnSync } = require("node:child_process");

function buildCodeModeHost({
  repoRoot = resolve(__dirname, "../.."),
  platform = process.platform,
  run = spawnSync,
} = {}) {
  const outDir = join(repoRoot, "desktop", "build", "bin");
  const binaryName = platform === "win32" ? "wuu-code-mode-host.exe" : "wuu-code-mode-host";
  const staged = join(outDir, binaryName);
  const source = join(repoRoot, "host", "codemode", "target", "release", binaryName);
  mkdirSync(outDir, { recursive: true });

  if (platform === "darwin" || platform === "linux") {
    const build = run("make", ["codemode-host"], {
      cwd: repoRoot,
      stdio: "inherit",
      env: process.env,
    });
    if (build.error) throw build.error;
    if (build.status !== 0) {
      throw new Error(`make codemode-host failed with status ${build.status}`);
    }
    copyFileSync(source, staged);
  } else if (platform === "win32") {
    // Windows packaging requires a separately built Windows runtime.
    if (!existsSync(staged) || statSync(staged).size === 0) {
      throw new Error("Stage a Windows wuu-code-mode-host.exe in desktop/build/bin before building the desktop app");
    }
  } else {
    throw new Error(`unsupported platform ${platform} for code-mode host build`);
  }
  if (statSync(staged).size === 0) throw new Error(`empty code-mode host: ${staged}`);
  chmodSync(staged, 0o755);
  console.log(`staged code-mode host: ${staged}`);
  return staged;
}

if (require.main === module) buildCodeModeHost();
module.exports = { buildCodeModeHost };
