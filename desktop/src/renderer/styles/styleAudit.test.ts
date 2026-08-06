import { readFileSync, readdirSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

/*
 * Reverse token audit.
 *
 * Component styles must paint through custom properties so every color
 * resolves through the theme layer and, in turn, the public --wuu-*
 * semantic token contract. This test flags color literals used in
 * ordinary declarations (color, background, box-shadow, gradients...).
 *
 * Allowed literal sites, by design:
 * - custom property declarations (`--slot: <value>`), including
 *   multi-line values — these are the theme layer's declaration sites;
 * - var(...) fallback arguments;
 * - the `transparent` keyword.
 *
 * Baseline counts form a ratchet: a file may only decrease its count.
 * The target is zero everywhere, at which point the baseline should be
 * emptied and this audit becomes strict.
 */

const COLOR_LITERAL =
  /#[0-9a-fA-F]{3,8}\b|(?:rgba?|hsla?)\([^)]*\)|\b(?:white|black)\b(?![-\w])/g;

const BASELINE: Readonly<Record<string, number>> = {};

function stripComments(source: string): string {
  return source.replace(/\/\*[\s\S]*?\*\//g, "");
}

function stripVarCalls(line: string): string {
  let output = "";
  let depth = 0;
  for (let index = 0; index < line.length; index += 1) {
    if (depth === 0 && line.slice(index, index + 4) === "var(") {
      depth = 1;
      index += 3;
      continue;
    }
    if (depth > 0) {
      if (line[index] === "(") depth += 1;
      if (line[index] === ")") depth -= 1;
      continue;
    }
    output += line[index];
  }
  return output;
}

function countColorLiterals(source: string): number {
  let count = 0;
  let inCustomDeclaration = false;
  for (const line of stripComments(source).split("\n")) {
    const trimmed = line.trim();
    if (inCustomDeclaration) {
      if (trimmed.endsWith(";")) inCustomDeclaration = false;
      continue;
    }
    if (/^--[\w-]+\s*:/.test(trimmed)) {
      if (!trimmed.endsWith(";")) inCustomDeclaration = true;
      continue;
    }
    const matches = stripVarCalls(trimmed).match(COLOR_LITERAL);
    if (matches) count += matches.length;
  }
  return count;
}

const stylesDir = __dirname;
const cssFiles = readdirSync(stylesDir).filter((name) => name.endsWith(".css"));

describe("component styles paint through tokens", () => {
  it("has no color literals outside token declaration sites", () => {
    const failures: string[] = [];
    for (const file of cssFiles) {
      const source = readFileSync(resolve(stylesDir, file), "utf8");
      const count = countColorLiterals(source);
      const baseline = BASELINE[file] ?? 0;
      if (count > baseline) {
        failures.push(
          `${file}: ${count} color literals (baseline ${baseline}) — paint through custom properties instead of literals`,
        );
      }
    }
    expect(failures).toEqual([]);
  });
});
