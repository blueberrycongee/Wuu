import { readFileSync, readdirSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

import {
  BASELINE_FILE,
  formatBaseline,
  type CssFile,
} from "./themeCoverage";

/*
 * Forward token coverage ratchet (complements the reverse literal audit in
 * styleAudit.test.ts).
 *
 * Every color-kind paint declaration must resolve through the public --wuu-*
 * contract: each custom property referenced in its value is either a public
 * token or its resolution chain (through its own definitions' var()
 * references, transitively, union across multiple definitions) reaches one.
 * Violations go into the committed baseline; the comparison is an exact
 * match, so a new unbridged surface and a bridge without a baseline update
 * both fail.
 */

const stylesDir = __dirname;
const cssFiles = readdirSync(stylesDir)
  .filter((name) => name.endsWith(".css"))
  .sort();

function loadHostStyles(): CssFile[] {
  return cssFiles.map((name) => ({
    name,
    source: readFileSync(resolve(stylesDir, name), "utf8"),
  }));
}

function baselineContentLines(): string[] {
  return readFileSync(resolve(stylesDir, BASELINE_FILE), "utf8")
    .split("\n")
    .filter((line) => line.length > 0 && !line.startsWith("#"));
}

const SANCTIONED_BRIDGE = "prop: var(--wuu-slot, var(--private-fallback))";

describe("theme token coverage ratchet", () => {
  it("matches the committed baseline exactly", () => {
    const computed = formatBaseline(loadHostStyles());
    const baseline = baselineContentLines();

    const baselineSet = new Set(baseline);
    const computedSet = new Set(computed);
    const added = computed.filter((line) => !baselineSet.has(line));
    const removed = baseline.filter((line) => !computedSet.has(line));

    const message = [
      "theme coverage drift: computed violations must exactly match the baseline",
      "",
      added.length > 0
        ? `NEW unbridged color surfaces (bridge them, then add the lines):\n  ${added.join("\n  ")}`
        : null,
      removed.length > 0
        ? `STALE baseline entries (a bridge removed them; delete the lines):\n  ${removed.join("\n  ")}`
        : null,
      "",
      `sanctioned bridge form: ${SANCTIONED_BRIDGE}`,
    ]
      .filter((section): section is string => section !== null)
      .join("\n");

    expect(baseline, message).toEqual(computed);
    expect(baseline.length).toBeGreaterThan(0);
  });

  it("flags an unbridged custom property consumed by a color paint declaration", () => {
    const fixture: CssFile[] = [
      {
        name: "fixture.css",
        source: [
          ":root {",
          "  --x: #ffffff;",
          "}",
          ".fixture {",
          "  color: var(--x);",
          "}",
          "",
        ].join("\n"),
      },
    ];
    expect(formatBaseline(fixture)).toEqual(["fixture.css:5 color --x"]);
  });

  it("passes when a paint declaration references a public token directly", () => {
    const fixture: CssFile[] = [
      {
        name: "fixture.css",
        source: [".fixture {", "  color: var(--wuu-color-text, #ffffff);", "}", ""].join("\n"),
      },
    ];
    expect(formatBaseline(fixture)).toEqual([]);
  });

  it("unions multiple definitions: one bridged scope covers the variable", () => {
    const fixture: CssFile[] = [
      {
        name: "fixture.css",
        source: [
          ":root {",
          "  --x: #ffffff;",
          "}",
          ':root[data-theme="dark"] {',
          "  --x: var(--wuu-color-text, #000000);",
          "}",
          ".fixture {",
          "  color: var(--x);",
          "}",
          "",
        ].join("\n"),
      },
    ];
    expect(formatBaseline(fixture)).toEqual([]);
  });

  it("ignores variables that resolve to geometry, even inside color paint properties", () => {
    const fixture: CssFile[] = [
      {
        name: "fixture.css",
        source: [
          ":root {",
          "  --gap: 8px;",
          "  --indent: var(--gap);",
          "}",
          ".fixture {",
          "  color: var(--indent);",
          "}",
          "",
        ].join("\n"),
      },
    ];
    expect(formatBaseline(fixture)).toEqual([]);
  });

  it("treats variables with no definition in the stylesheets as out of scope", () => {
    const fixture: CssFile[] = [
      {
        name: "fixture.css",
        source: [".fixture {", "  color: var(--js-provided-color);", "}", ""].join("\n"),
      },
    ];
    expect(formatBaseline(fixture)).toEqual([]);
  });
});
