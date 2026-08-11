import { readFileSync } from "node:fs";
import { basename, resolve } from "node:path";

import { describe, expect, it } from "vitest";

const repositoryRoot = basename(process.cwd()) === "desktop"
  ? resolve(process.cwd(), "..")
  : process.cwd();
const mangaDesktopModule = readFileSync(
  resolve(repositoryRoot, "examples/plugins/manga-studio/desktop.js"),
  "utf8",
);

describe("Manga Studio example shell contract", () => {
  it("no longer wraps the host app shell with a bespoke surface", () => {
    expect(mangaDesktopModule).not.toContain(".manga-shell");
    expect(mangaDesktopModule).not.toContain("body:has(");
  });

  it("styles published plugin components without overriding host buttons", () => {
    expect(mangaDesktopModule).not.toContain("button {");
    expect(mangaDesktopModule).toContain('[data-wuu-component="plugin-ui-panel"]');
    expect(mangaDesktopModule).toContain('[data-wuu-component="plugin-ui-card"]');
    expect(mangaDesktopModule).toContain('[data-wuu-component="jump-to-latest"]');
    expect(mangaDesktopModule).toContain("border-radius: var(--wuu-radius-control)");
  });
});
