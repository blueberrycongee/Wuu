import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, describe, expect, it } from "vitest";
import { EmptyConversationHome, RuntimeLoading, ViewSwitchLoading } from "./LoadingViews";

let root: Root | undefined;
let container: HTMLDivElement | undefined;

function render(view: JSX.Element): HTMLDivElement {
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
  act(() => root?.render(view));
  return container;
}

afterEach(() => {
  act(() => root?.unmount());
  root = undefined;
  container?.remove();
  container = undefined;
});

describe("RuntimeLoading", () => {
  it("centers the wuu blobatar mascot inside one glass app-icon surface", () => {
    const view = render(<RuntimeLoading status="connecting" />);

    const glass = view.querySelector(".wuu-launch-glass");
    const mascot = glass?.querySelector<SVGSVGElement>("svg.wuu-launch-mascot");

    expect(glass).not.toBeNull();
    expect(mascot).not.toBeNull();
    expect(mascot?.querySelector("g.mo-root.mo-always")).not.toBeNull();
    expect(view.querySelector(".wuu-launch-mark")).toBeNull();
    expect(view.querySelector(".wuu-launch-rail")).toBeNull();
  });

  it("keeps the compact legacy loader for view switches", () => {
    const view = render(<ViewSwitchLoading />);

    expect(view.querySelector(".wuu-launch-mark")).not.toBeNull();
    expect(view.querySelector(".wuu-launch-rail")).not.toBeNull();
    expect(view.querySelector(".wuu-launch-glass")).toBeNull();
  });
});

describe("EmptyConversationHome", () => {
  it("greets with an always-animating round blobatar", () => {
    const view = render(
      <EmptyConversationHome title="Hello">
        <div className="hero-composer" />
      </EmptyConversationHome>,
    );

    // The mascot is now an inline-SVG blobatar; "always" animation puts the
    // mo-root/mo-always classes and seeded motion properties on an inner <g>
    // so expression changes transition on the same element (see blobatar/react).
    const mascot = view.querySelector<SVGSVGElement>("svg.empty-home-mascot");
    const motionRoot = mascot?.querySelector<SVGGElement>("g.mo-root.mo-always");
    expect(mascot).not.toBeNull();
    expect(mascot?.getAttribute("aria-hidden")).toBe("true");
    expect(motionRoot).not.toBeNull();
    expect(motionRoot?.getAttribute("style")).toContain("--mo-phase");
  });
});
