import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const environmentCSS = readFileSync(
  resolve(__dirname, "styles/environment.css"),
  "utf-8",
);

function cssRule(selector: string): string {
  const escapedSelector = selector.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const match = environmentCSS.match(
    new RegExp(`^${escapedSelector}\\s*\\{([\\s\\S]*?)\\n\\}`, "m"),
  );
  if (!match) {
    throw new Error(`missing CSS rule for ${selector}`);
  }
  return match[1] ?? "";
}

describe("environment panel rows", () => {
  it("overlays the close button beside the first content row", () => {
    const header = cssRule(".environment-panel-header.floating");
    expect(header).toMatch(/position:\s*absolute/);
    expect(header).toMatch(/top:\s*15px/);
    expect(header).toMatch(/right:\s*14px/);
    expect(
      cssRule(
        ".environment-panel-header.floating + .environment-panel-body .environment-row:first-child",
      ),
    ).toMatch(/padding-right:\s*40px/);
  });

  it("keeps titled headers (group info, file preview) in normal flow", () => {
    const header = cssRule(".environment-panel-header");
    expect(header).not.toMatch(/position:\s*absolute/);
    expect(header).toMatch(/justify-content:\s*space-between/);
    expect(cssRule(".environment-panel-header h2")).toMatch(/margin:\s*0/);
    expect(cssRule(".environment-panel-title")).toMatch(/min-width:\s*0/);
  });

});
