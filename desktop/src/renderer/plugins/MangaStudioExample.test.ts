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

function cssRule(selector: string): string {
  const escapedSelector = selector.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const match = mangaDesktopModule.match(new RegExp(`${escapedSelector}\\s*\\{([^}]*)\\}`));
  expect(match, `missing Manga Studio rule: ${selector}`).not.toBeNull();
  return match?.[1] ?? "";
}

describe("Manga Studio example shell contract", () => {
  it("keeps the app-shell wrapper out of the host layout box tree", () => {
    const shellRule = cssRule(".manga-shell");

    expect(shellRule).toContain("display: contents");
    expect(shellRule).not.toMatch(/\b(?:width|height|overflow)\s*:/);
  });

  it("styles published controls without overriding every host button", () => {
    expect(mangaDesktopModule).not.toContain(".manga-shell button {");
    expect(mangaDesktopModule).not.toContain(".manga-shell button:not(:disabled):hover");
    expect(mangaDesktopModule).toContain(
      '.manga-shell [data-wuu-component="conversation-pane"] > .titlebar',
    );
  });
});
