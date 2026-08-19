#!/usr/bin/env node
// Cross-shell replacement for the POSIX `KEY=value command` prefix, which
// cmd.exe and PowerShell do not understand. Usage:
//   node scripts/env-run.cjs KEY=value [KEY2=value2 ...] command [args...]
// Leading KEY=value tokens land in the child's environment; the rest is the
// command line. npm has already prepended node_modules/.bin to PATH, so bare
// tool names (vitest, electron-vite) resolve on every platform.
const { spawn } = require("node:child_process");

const args = process.argv.slice(2);
const env = { ...process.env };
let index = 0;
while (index < args.length && /^[A-Za-z_][A-Za-z0-9_]*=/.test(args[index])) {
  const separator = args[index].indexOf("=");
  env[args[index].slice(0, separator)] = args[index].slice(separator + 1);
  index += 1;
}

const command = args[index];
if (!command) {
  console.error("usage: env-run.cjs KEY=value ... command [args...]");
  process.exit(2);
}

const child = spawn(command, args.slice(index + 1), {
  stdio: "inherit",
  env,
  // Windows tool shims and bare npm-script commands may resolve to .cmd
  // files, which require cmd.exe. Native .exe/.com programs must be spawned
  // directly so arguments such as `node -e "..."` are not re-parsed or
  // corrupted by an extra shell (and do not trigger Node's shell+args warning).
  shell:
    process.platform === "win32" &&
    !/\.(?:exe|com)$/i.test(command),
});
child.on("error", (error) => {
  console.error(`env-run: ${error.message}`);
  process.exit(1);
});
child.on("exit", (code, signal) => {
  process.exit(code ?? (signal ? 1 : 0));
});
