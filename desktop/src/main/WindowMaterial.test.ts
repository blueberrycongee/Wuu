import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const mainSource = readFileSync(resolve(process.cwd(), "src/main/index.ts"), "utf8");

describe("macOS sidebar material", () => {
  it("uses under-window vibrancy for the sidebar rail", () => {
    expect(mainSource).toContain('vibrancy: "under-window"');
  });
});
