import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

import {
  LEGACY_THEME_TOKEN_ALIASES,
  PUBLIC_SYNTAX_TOKEN_NAMES,
  PUBLIC_THEME_TOKEN_NAMES,
} from "../../shared/themeContract.generated";

import {
  parsePinnedAnchors,
  type SurfaceMatrix,
} from "./themeCoverage";

/*
 * Cross-consistency for the generated surface matrix (U2):
 * the committed matrix must equal a fresh analysis, its unbridged rows must
 * be exactly the U1 baseline entries, and every current public color token
 * must reach a host paint declaration.
 */

const stylesDir = __dirname;
const rendererDir = resolve(__dirname, "..");
const repoRoot = resolve(__dirname, "../../../../");
const configDir = resolve(repoRoot, "config");

function committedMatrix(): SurfaceMatrix {
  return JSON.parse(
    readFileSync(resolve(configDir, "desktop-theme-surface-matrix.json"), "utf8"),
  );
}

function baselineEntries(): Set<string> {
  return new Set(
    readFileSync(resolve(stylesDir, "themeCoverage.baseline.txt"), "utf8")
      .split("\n")
      .filter((line) => line.length > 0 && !line.startsWith("#")),
  );
}

describe("theme surface matrix consistency", () => {
  it("maps the U1 baseline to unbridged rows bijectively", () => {
    const matrix = committedMatrix();
    const unbridged = new Set(
      matrix.rows
        .filter((row) => row.status === "unbridged")
        .map((row) => `${row.file} ${row.selector} ${row.prop} ${row.variable}`),
    );
    expect(unbridged).toEqual(baselineEntries());
    for (const row of matrix.rows) {
      if (row.status === "bridged") {
        expect(row.tokens.length).toBeGreaterThan(0);
      } else {
        expect(row.tokens).toEqual([]);
      }
    }
  });

  it("keeps every current color token reachable from a host paint declaration", () => {
    const contract: {
      tokens: Array<{ name: string; category: string }>;
      syntax: string[];
    } = JSON.parse(readFileSync(resolve(configDir, "desktop-theme-contract.json"), "utf8"));
    const categories = new Map(contract.tokens.map((token) => [token.name, token.category]));
    const legacy = new Set(Object.keys(LEGACY_THEME_TOKEN_ALIASES));
    const contractTokens = new Set<string>([...PUBLIC_THEME_TOKEN_NAMES, ...PUBLIC_SYNTAX_TOKEN_NAMES]);
    const matrix = committedMatrix();
    const tokenSet = new Set(matrix.tokenSet);

    for (const token of tokenSet) {
      expect(contractTokens.has(token), `${token} is a contract token`).toBe(true);
    }
    for (const token of PUBLIC_THEME_TOKEN_NAMES) {
      if (legacy.has(token)) continue; // the consumer audit skips legacy aliases too
      if (categories.get(token) !== "color") continue; // typography/spacing/etc. reach no paint
      expect(tokenSet.has(token), `${token} reaches a host paint declaration`).toBe(true);
    }
  });

  it("keeps every pinned anchor in the anchor inventory", () => {
    const pinned = parsePinnedAnchors(
      readFileSync(resolve(rendererDir, "plugins/ProductionSemanticAnchors.test.ts"), "utf8"),
    );
    const anchors = new Set(committedMatrix().anchors.map((anchor) => anchor.name));
    for (const anchor of pinned) {
      expect(anchors.has(anchor), `${anchor} anchor inventory`).toBe(true);
    }
  });
});
