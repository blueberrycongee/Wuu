import { describe, expect, it } from "vitest";

import {
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

describe("theme token coverage ratchet", () => {
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
    expect(formatBaseline(fixture)).toEqual(["fixture.css .fixture color --x"]);
  });

  it("keeps baseline identities stable when unrelated lines move", () => {
    const source = [
      ":root {",
      "  --x: #ffffff;",
      "}",
      ".fixture {",
      "  color: var(--x);",
      "}",
      "",
    ].join("\n");

    expect(formatBaseline([{ name: "fixture.css", source: `/* unrelated */\n\n${source}` }]))
      .toEqual(formatBaseline([{ name: "fixture.css", source }]));
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
