import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const workspaceCSS = readFileSync(resolve(__dirname, "styles/workspace.css"), "utf-8");

function cssRule(selector: string): string {
  const escapedSelector = selector.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const match = workspaceCSS.match(new RegExp(`^${escapedSelector}\\s*\\{([\\s\\S]*?)\\n\\}`, "m"));
  if (!match) throw new Error(`missing CSS rule for ${selector}`);
  return match[1] ?? "";
}

describe("plugin settings layout", () => {
  it("stacks each setting instead of inheriting the generic two-column row grid", () => {
    expect(cssRule(".plugin-setting")).toMatch(/display:\s*flex;/);
    expect(cssRule(".plugin-setting")).toMatch(/flex-direction:\s*column;/);
    expect(cssRule(".plugin-setting-heading")).toMatch(/flex-wrap:\s*wrap;/);
    expect(workspaceCSS).toMatch(
      /^\.plugin-setting-status\s*\{[\s\S]*?overflow-wrap:\s*anywhere;[\s\S]*?^\}/m,
    );
  });
});
