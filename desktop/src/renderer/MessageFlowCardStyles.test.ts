import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const turnsCSS = readFileSync(
  resolve(process.cwd(), "src/renderer/styles/turns.css"),
  "utf8",
);

function ruleFor(selector: string): string {
  const escapedSelector = selector.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  return turnsCSS.match(new RegExp(`${escapedSelector}\\s*\\{[\\s\\S]*?\\}`))?.[0] ?? "";
}

describe("message-flow card styles", () => {
  it("keeps code blocks and rich artifact summaries on one surface recipe", () => {
    const codeCard = ruleFor(".rich-code-block");
    const outputCard = ruleFor(".turn-output-summary-card");

    for (const declaration of [
      "background: var(--message-flow-card-bg)",
      "border: 1px solid var(--message-flow-card-border)",
      "border-radius: var(--message-flow-card-radius)",
      "box-shadow: var(--message-flow-card-shadow)",
    ]) {
      expect(codeCard).toContain(declaration);
      expect(outputCard).toContain(declaration);
    }
  });

  it("uses the same clean header surface and divider for both card types", () => {
    const codeHeader = ruleFor(".rich-code-header");
    const outputHeader = ruleFor(".turn-output-summary-header");

    for (const declaration of [
      "background: var(--message-flow-card-header-bg)",
      "border-bottom: 1px solid var(--message-flow-card-divider)",
    ]) {
      expect(codeHeader).toContain(declaration);
      expect(outputHeader).toContain(declaration);
    }
  });

  it("keeps the file-change object flat and border-defined", () => {
    const editCard = ruleFor(".turn-edit-summary-card");
    const editOverview = ruleFor(".turn-edit-summary-overview");

    expect(editCard).toContain("background: var(--paper)");
    expect(editCard).toContain("border: 1px solid var(--message-flow-card-border)");
    expect(editCard).toContain("border-radius: var(--message-flow-card-radius)");
    expect(editCard).toContain("box-shadow: none");
    expect(editOverview).toContain("min-height: 68px");
  });
});
