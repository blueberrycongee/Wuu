// Builds (or reuses) the code-mode host binary and stages it into
// desktop/build/bin so electron-builder bundles it next to wuu-core.
//
// The host is a pinned, from-source V8 sandbox build. The first build takes
// 30-60 minutes; a binary already present in build/bin is reused without
// rebuilding. The workspace Makefile owns the reproducible build steps
// (seed-icu-data.sh pins the mksnapshot ICU data, .cargo/config.toml pins the
// GN args); this script only coordinates and copies.
const { copyFileSync, mkdirSync, existsSync, statSync, chmodSync } = require("node:fs");
const { join, resolve } = require("node:path");
const { spawnSync } = require("node:child_process");

function main() {
  const desktopRoot = resolve(__dirname, "..");
  const repoRoot = resolve(desktopRoot, "..");
  const outDir = join(desktopRoot, "build", "bin");
  const staged = join(outDir, process.platform === "win32" ? "wuu-code-mode-host.exe" : "wuu-code-mode-host");
  const source = join(
    repoRoot,
    "host",
    "codemode",
    "target",
    "release",
    process.platform === "win32" ? "wuu-code-mode-host.exe" : "wuu-code-mode-host",
  );

  mkdirSync(outDir, { recursive: true });

  if (!existsSync(staged) || statSync(staged).size === 0) {
    if (existsSync(source) && statSync(source).size > 0) {
      copyFileSync(source, staged);
    } else {
      if (process.platform === "win32") {
        throw new Error(
          "wuu-code-mode-host must be built on macOS/Linux and staged into desktop/build/bin before Windows packaging",
        );
      }
      const make = process.platform === "darwin" || process.platform === "linux" ? "make" : null;
      if (!make) {
        throw new Error(`unsupported platform ${process.platform} for code-mode host build`);
      }
      const build = spawnSync(make, ["codemode-host"], {
        cwd: repoRoot,
        stdio: "inherit",
        env: process.env,
      });
      if (build.status !== 0) {
        throw new Error(`make codemode-host failed with status ${build.status}`);
      }
      copyFileSync(source, staged);
    }
  }
  chmodSync(staged, 0o755);
  console.log(`staged code-mode host: ${staged}`);
}

main();
