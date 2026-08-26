import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const mainSource = readFileSync(resolve(process.cwd(), "src/main/index.ts"), "utf8");
const sidebarCSS = readFileSync(
  resolve(process.cwd(), "src/renderer/styles/sidebar.css"),
  "utf8",
);
const themeCSS = readFileSync(
  resolve(process.cwd(), "src/renderer/styles/theme.css"),
  "utf8",
);

describe("macOS sidebar material", () => {
  it("frosts the under-window material into a bright rail with a dark hairline edge", () => {
    expect(mainSource).toContain('vibrancy: "under-window"');
    expect(sidebarCSS).toMatch(
      /--sidebar-material-fill:\s*rgba\(255, 255, 255, 0\.62\);/,
    );
    expect(sidebarCSS).toMatch(
      /--sidebar-material-edge:\s*rgba\(17, 19, 21, 0\.07\);/,
    );
    expect(sidebarCSS).toMatch(
      /\.sidebar,\s*\n\.settings-sidebar\s*\{[\s\S]*?background:\s*var\(--sidebar-material-fill\);/,
    );
    expect(sidebarCSS).toMatch(
      /\.sidebar::before,\s*\n\.settings-sidebar::before\s*\{[\s\S]*?opacity:\s*0\.5;/,
    );
    expect(sidebarCSS).toMatch(
      /\.sidebar,\s*\n\.settings-sidebar\s*\{[\s\S]*?backdrop-filter:\s*none;/,
    );
  });

  it("uses a stable dark surface above the wallpaper material", () => {
    expect(themeCSS).toMatch(
      /:root\[data-theme="dark"\][\s\S]*?--sidebar-material-fill:\s*rgba\(16, 18, 21, 0\.88\);/,
    );
  });
});
