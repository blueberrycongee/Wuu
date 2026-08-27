import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const channelsCSS = readFileSync(
  resolve(__dirname, "styles/channels.css"),
  "utf8",
);

function cssRule(selector: string): string {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const match = channelsCSS.match(new RegExp(`${escaped}\\s*\\{([^}]*)\\}`));
  if (!match) throw new Error(`missing CSS rule for ${selector}`);
  return match[1] ?? "";
}

describe("channel room layout", () => {
  it("keeps the composer in flow so resizing shrinks the message stream synchronously", () => {
    expect(cssRule(".channel-room-main")).toMatch(
      /grid-template-rows:\s*auto auto minmax\(0, 1fr\) auto;/,
    );

    const streamRule = cssRule(".channel-message-stream");
    expect(streamRule).toContain("grid-row: 3;");
    expect(streamRule).not.toContain("--channel-composer-height");

    const footerRule = cssRule(".channel-conversation-footer");
    expect(footerRule).toContain("grid-row: 4;");
    expect(footerRule).not.toMatch(/position:\s*absolute;/);
  });
});
