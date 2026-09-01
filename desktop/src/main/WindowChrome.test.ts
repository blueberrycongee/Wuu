import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const read = (path: string): string =>
  readFileSync(resolve(process.cwd(), path), "utf8");

const mainSource = read("src/main/index.ts");
const preloadSource = read("src/main/preload.ts");

describe("window chrome platform branches", () => {
  it("keeps hiddenInset and the traffic lights exclusive to the darwin branch", () => {
    // Exactly one declaration each — inside windowFrameOptions' darwin
    // branch. A second occurrence means a window factory regressed to a
    // hardcoded mac-only chrome. (The bare identifier also appears in the
    // options Pick<>, so match the value assignments.)
    expect(mainSource.match(/titleBarStyle: "hiddenInset"/g)).toHaveLength(1);
    expect(mainSource.match(/trafficLightPosition: \{/g)).toHaveLength(1);
    expect(mainSource).toContain("trafficLightPosition: { x: 18, y: 15 }");
  });

  it("gives Windows a hidden titlebar with a themed controls overlay", () => {
    expect(mainSource).toContain('titleBarStyle: "hidden"');
    expect(mainSource).toContain("titleBarOverlay: windowsTitleBarOverlay()");
    // Theme flips re-push overlay colors to every open overlay window.
    expect(mainSource).toContain('nativeTheme.on("updated", syncThemeAcrossWindows)');
  });

  it("stamps data-platform before first paint, like data-theme", () => {
    expect(preloadSource).toContain("document.documentElement.dataset.platform");
  });
});
