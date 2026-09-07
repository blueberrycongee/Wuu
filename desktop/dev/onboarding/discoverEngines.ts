import { accessSync, constants, statSync } from "node:fs";
import { delimiter, isAbsolute, join } from "node:path";
import type { EngineListResult } from "../../src/shared/protocol";

// The standalone preview has no app-server or saved settings. Resolve CLI
// availability from the launch environment without starting either agent.
export function discoverPreviewEngines(env: NodeJS.ProcessEnv = process.env): EngineListResult {
  const engines = ["codex", "claude"].map((id) => {
    const override = env[`WUU_${id.toUpperCase()}_BINARY`]?.trim();
    const extensions = process.platform === "win32"
      ? (env.PATHEXT ?? ".COM;.EXE;.BAT;.CMD").split(";")
      : [""];
    const candidates = override ? [override] : (env.PATH ?? "").split(delimiter)
      .filter(isAbsolute)
      .flatMap((directory) => extensions.map((extension) => join(directory, id + extension)));
    const binary = candidates.find((candidate) => {
      try {
        if (!isAbsolute(candidate) || !statSync(candidate).isFile()) return false;
        accessSync(candidate, constants.X_OK);
        return true;
      } catch {
        return false;
      }
    });
    return { id, enabled: Boolean(binary), binary_ok: Boolean(binary) };
  });
  return { engines: [{ id: "wuu", enabled: true, binary_ok: true }, ...engines] };
}
