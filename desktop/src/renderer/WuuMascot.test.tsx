import { act, type ReactElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, describe, expect, it } from "vitest";
import { WuuMascot, WUU_MASCOT_ACTIVITY_PROP_LAYOUT } from "./WuuMascot";
import {
  WUU_MASCOT_ACTIVITY_LOOK,
  WUU_MASCOT_IDENTITY_PERSPECTIVE,
} from "./wuu-mascot-spec";

let container: HTMLDivElement | null = null;
let root: Root | null = null;

function render(node: ReactElement): HTMLDivElement {
  if (container) unmount();
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
  act(() => {
    root!.render(node);
  });
  return container;
}

function rerender(node: ReactElement): void {
  act(() => {
    root!.render(node);
  });
}

function unmount(): void {
  if (root) {
    act(() => {
      root!.unmount();
    });
    root = null;
  }
  if (container) {
    container.remove();
    container = null;
  }
}

afterEach(unmount);

function facePaths(svg: Element): string {
  // Status props (pencil, question mark) portal into a sibling of `.mo-eyes`
  // and are supposed to remount. Compare only the blobatar-owned body and
  // eyes so an activity switch cannot look like a face rebuild.
  const body = svg.querySelector(".mo-bob > g:not(.mo-eyes):not(.wuu-mascot-layer) path");
  return [body, ...svg.querySelectorAll(".mo-eye path")]
    .map((path) => path?.getAttribute("d") ?? "")
    .join("|");
}

describe("WuuMascot activity morph", () => {
  it("keeps the face markup when activity changes and remounts only the status prop", () => {
    const host = render(<WuuMascot activity="thinking" accessory="none" />);
    const svg = host.querySelector("svg")!;
    const thinkingPaths = facePaths(svg);
    const rootGroup = svg.querySelector(".mo-root");

    expect(svg.getAttribute("data-wuu-mascot-activity")).toBe("thinking");
    expect(svg.style.getPropertyValue("--wuu-mascot-look-x")).toBe(
      `${WUU_MASCOT_ACTIVITY_LOOK.thinking.x}px`,
    );
    expect(svg.style.getPropertyValue("--wuu-mascot-look-y")).toBe(
      `${WUU_MASCOT_ACTIVITY_LOOK.thinking.y}px`,
    );
    expect(host.querySelector(".wuu-mascot-activity-prop-thinking")).not.toBeNull();
    const thinkingLayout = WUU_MASCOT_ACTIVITY_PROP_LAYOUT.thinking;
    expect(
      host
        .querySelector(".wuu-mascot-activity-prop-thinking .wuu-mascot-activity-motion > g")
        ?.getAttribute("transform"),
    ).toContain(`translate(${thinkingLayout.x} ${thinkingLayout.y})`);
    expect(thinkingPaths.length).toBeGreaterThan(0);

    rerender(<WuuMascot activity="edit" accessory="none" />);

    const next = host.querySelector("svg")!;
    expect(next).toBe(svg);
    expect(next.querySelector(".mo-root")).toBe(rootGroup);
    expect(facePaths(next)).toBe(thinkingPaths);
    expect(next.getAttribute("data-wuu-mascot-activity")).toBe("edit");
    expect(next.style.getPropertyValue("--wuu-mascot-look-x")).toBe(
      `${WUU_MASCOT_ACTIVITY_LOOK.edit.x}px`,
    );
    expect(next.style.getPropertyValue("--wuu-mascot-look-y")).toBe(
      `${WUU_MASCOT_ACTIVITY_LOOK.edit.y}px`,
    );
    expect(host.querySelector(".wuu-mascot-activity-prop-thinking")).toBeNull();
    expect(host.querySelector(".wuu-mascot-activity-prop-edit")).not.toBeNull();
    const editLayout = WUU_MASCOT_ACTIVITY_PROP_LAYOUT.edit;
    expect(
      host
        .querySelector(".wuu-mascot-activity-prop-edit .wuu-mascot-activity-motion > g")
        ?.getAttribute("transform"),
    ).toContain(`translate(${editLayout.x} ${editLayout.y})`);
  });

  it("bakes only the idle identity perspective into path data", () => {
    const host = render(<WuuMascot activity="search" accessory="none" />);
    const svg = host.querySelector("svg")!;
    const paths = facePaths(svg);
    expect(WUU_MASCOT_IDENTITY_PERSPECTIVE).toEqual({
      yaw: 8,
      pitch: -16,
      strength: 1,
    });
    expect(svg.getAttribute("data-wuu-mascot-activity")).toBe("search");

    rerender(<WuuMascot activity="command" accessory="none" />);
    expect(host.querySelector("svg")).toBe(svg);
    expect(facePaths(svg)).toBe(paths);
    expect(svg.getAttribute("data-wuu-mascot-activity")).toBe("command");
  });
});
