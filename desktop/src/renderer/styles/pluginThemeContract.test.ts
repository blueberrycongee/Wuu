import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const baseCSS = readFileSync(resolve(__dirname, "base.css"), "utf8");
const workspaceCSS = readFileSync(resolve(__dirname, "workspace.css"), "utf8");

describe("plugin theme semantic token consumers", () => {
  it("maps every public token family into host-owned Desktop surfaces", () => {
    for (const token of [
      "--wuu-color-canvas",
      "--wuu-color-text",
      "--wuu-font-family-ui",
      "--wuu-font-size-body",
      "--wuu-space-unit",
      "--wuu-space-density",
      "--wuu-radius-control",
      "--wuu-elevation-panel",
      "--wuu-motion-duration-fast",
      "--wuu-motion-easing-standard",
      "--wuu-content-max-width",
    ]) {
      expect(baseCSS).toMatch(new RegExp(`var\\(\\s*${token}`));
    }
    expect(workspaceCSS).toContain("var(--wuu-border-subtle");
    for (const token of ["keyword", "function", "string", "comment"]) {
      expect(workspaceCSS).toContain(`var(--wuu-syntax-${token}`);
    }
  });
});
