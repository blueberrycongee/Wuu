import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";
import {
  LEGACY_THEME_TOKEN_ALIASES,
  PUBLIC_THEME_TOKEN_NAMES,
} from "../../shared/themeContract.generated";

const baseCSS = readFileSync(resolve(__dirname, "base.css"), "utf8");
const workspaceCSS = readFileSync(resolve(__dirname, "workspace.css"), "utf8");
const hostCSS = [
  baseCSS,
  workspaceCSS,
  readFileSync(resolve(__dirname, "environment.css"), "utf8"),
  readFileSync(resolve(__dirname, "composer-context-meter.css"), "utf8"),
  readFileSync(resolve(__dirname, "theme.css"), "utf8"),
].join("\n");

describe("plugin theme semantic token consumers", () => {
  it("maps every public token family into host-owned Desktop surfaces", () => {
    const legacyTokens = new Set(Object.keys(LEGACY_THEME_TOKEN_ALIASES));
    for (const token of PUBLIC_THEME_TOKEN_NAMES) {
      if (legacyTokens.has(token)) continue;
      expect(hostCSS, `${token} host consumer`).toMatch(new RegExp(`var\\(\\s*${token}`));
    }
    expect(workspaceCSS).toContain("var(--wuu-border-subtle");
    for (const token of ["keyword", "function", "string", "comment"]) {
      expect(workspaceCSS).toContain(`var(--wuu-syntax-${token}`);
    }
  });
});
