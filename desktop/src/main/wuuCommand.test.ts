import { describe, expect, it } from "vitest";
import { mkdirSync, mkdtempSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { resolveWuuCommand } from "./wuuCommand";

describe("resolveWuuCommand (issue #8: never run a workspace-local core)", () => {
  it("ignores the active workspace and never falls back to wuu on PATH", () => {
    // Even if /repo/wuu contains bin/wuu or wuu (e.g. an ignored `make build`
    // artifact built for another OS), the resolver never inspects the
    // workspace, so a stale/foreign binary can't be spawned as the core.
    expect(() => resolveWuuCommand({}, "/repo/wuu", undefined)).toThrow(
      "wuu desktop core is missing",
    );
    // A discovered source root does NOT enable workspace/go-run on its own.
    expect(() => resolveWuuCommand({}, "/repo/wuu", "/src/wuu")).toThrow(
      "wuu desktop core is missing",
    );
  });

  it("honors an explicit desktop-core override", () => {
    expect(
      resolveWuuCommand(
        { WUU_DESKTOP_CORE: "/opt/wuu/bin/wuu-core" },
        "/repo/wuu",
        "/src/wuu",
      ),
    ).toEqual({ command: "/opt/wuu/bin/wuu-core", args: [], cwd: "/repo/wuu" });
  });

  it("uses go run only when opted in AND a source root is found", () => {
    expect(
      resolveWuuCommand(
        { WUU_DESKTOP_USE_GO_RUN: "1" },
        "/repo/wuu",
        "/src/wuu",
      ),
    ).toEqual({ command: "go", args: ["run", "./cmd/wuu"], cwd: "/src/wuu" });
  });

  it("fails when go-run is opted in but no source root or packaged core exists", () => {
    expect(() =>
      resolveWuuCommand({ WUU_DESKTOP_USE_GO_RUN: "1" }, "/repo/wuu", undefined),
    ).toThrow("wuu desktop core is missing");
  });

  it("prefers the packaged app-owned binary over PATH", () => {
    const resourcesPath = mkdtempSync(join(tmpdir(), "wuu-resources-"));
    const binaryName = process.platform === "win32" ? "wuu-core.exe" : "wuu-core";
    const binary = join(resourcesPath, "bin", binaryName);
    mkdirSync(join(resourcesPath, "bin"), { recursive: true });
    writeFileSync(binary, "#!/bin/sh\n");
    expect(resolveWuuCommand({}, "/repo/wuu", undefined, resourcesPath)).toEqual({
      command: binary,
      args: [],
      cwd: "/repo/wuu",
    });
  });

  it("only selects a core binary for the current platform", () => {
    const resourcesPath = mkdtempSync(join(tmpdir(), "wuu-resources-platform-"));
    const bin = join(resourcesPath, "bin");
    mkdirSync(bin, { recursive: true });
    const unixBinary = join(bin, "wuu-core");
    const windowsBinary = join(bin, "wuu-core.exe");
    writeFileSync(unixBinary, "unix");
    writeFileSync(windowsBinary, "windows");

    expect(
      resolveWuuCommand({}, "/repo/wuu", undefined, resourcesPath, "win32"),
    ).toMatchObject({ command: windowsBinary });
    expect(
      resolveWuuCommand({}, "/repo/wuu", undefined, resourcesPath, "linux"),
    ).toMatchObject({ command: unixBinary });
  });

  it("prefers the explicit desktop-core override over go run", () => {
    expect(
      resolveWuuCommand(
        { WUU_DESKTOP_CORE: "/opt/wuu-core", WUU_DESKTOP_USE_GO_RUN: "1" },
        "/repo/wuu",
        "/src/wuu",
      ),
    ).toEqual({ command: "/opt/wuu-core", args: [], cwd: "/repo/wuu" });
  });
});
