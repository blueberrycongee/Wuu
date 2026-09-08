import { act, createElement, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { useAutoFollowScrollContainer } from "./AutoFollowScroll";

interface StubbedLayout {
  scrollHeight: number;
  clientHeight: number;
  scrollTop: number;
}

function stubLayout(node: HTMLElement): StubbedLayout {
  const layout = {
    scrollHeight: 1200,
    clientHeight: 400,
    scrollTop: 800,
  };
  Object.defineProperties(node, {
    scrollHeight: {
      configurable: true,
      get: () => layout.scrollHeight,
    },
    clientHeight: {
      configurable: true,
      get: () => layout.clientHeight,
    },
    scrollTop: {
      configurable: true,
      get: () => layout.scrollTop,
      set: (value: number) => {
        layout.scrollTop = Math.max(
          0,
          Math.min(value, layout.scrollHeight - layout.clientHeight),
        );
      },
    },
  });
  return layout;
}

type HookHandle = ReturnType<typeof useAutoFollowScrollContainer>;

function Probe({ onReady }: { onReady: (handle: HookHandle) => void }): ReactNode {
  const handle = useAutoFollowScrollContainer();
  onReady(handle);
  return createElement("div", { ref: handle.scrollRef });
}

describe("useAutoFollowScrollContainer", () => {
  let container: HTMLDivElement;
  let root: Root | null = null;
  let handle: HookHandle | null = null;
  let scrollNode: HTMLDivElement | null = null;
  let layout: StubbedLayout | null = null;

  beforeEach(() => {
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
    act(() => {
      root?.render(createElement(Probe, { onReady: (next) => { handle = next; } }));
    });
    scrollNode = container.firstElementChild as HTMLDivElement;
    layout = stubLayout(scrollNode);
    act(() => {
      scrollNode?.dispatchEvent(new Event("scroll"));
    });
  });

  afterEach(() => {
    act(() => root?.unmount());
    document.body.removeChild(container);
  });

  it("stops following as soon as the user wheels upward", () => {
    act(() => {
      scrollNode?.dispatchEvent(new WheelEvent("wheel", { deltaY: -20 }));
    });

    expect(handle?.autoFollowRef.current).toBe(false);
    if (!layout || !handle) throw new Error("probe not mounted");
    layout.scrollHeight = 1400;
    handle.scrollToBottom();
    expect(layout.scrollTop).toBe(800);
  });

  it("keeps automatic viewport following quiet and still reveals user scrolling", () => {
    if (!layout || !handle || !scrollNode) throw new Error("probe not mounted");
    scrollNode.classList.remove("scrollbar-visible");
    for (const height of [300, 200, 300, 400]) {
      layout.clientHeight = height;
      act(() => {
        handle!.scrollToBottom();
        scrollNode!.dispatchEvent(new Event("scroll"));
      });
      expect(layout.scrollTop).toBe(layout.scrollHeight - height);
      expect(handle.autoFollowRef.current).toBe(true);
      expect(scrollNode.classList.contains("scrollbar-visible")).toBe(false);
    }
    act(() => {
      layout!.scrollTop -= 30;
      scrollNode!.dispatchEvent(new Event("scroll"));
    });
    expect(handle.autoFollowRef.current).toBe(false);
    expect(scrollNode.classList.contains("scrollbar-visible")).toBe(true);
  });

  it("stops following when a scrollbar drag moves upward without preflight input", () => {
    if (!layout) throw new Error("probe not mounted");
    layout.scrollTop = 500;
    act(() => {
      scrollNode?.dispatchEvent(new Event("scroll"));
    });

    expect(handle?.autoFollowRef.current).toBe(false);
  });

  it("does not resume following when keyboard dismissal clamps history to the bottom", () => {
    if (!layout || !handle || !scrollNode) throw new Error("probe not mounted");
    act(() => {
      layout!.scrollTop = 600;
      scrollNode!.dispatchEvent(new Event("scroll"));
    });
    expect(handle.autoFollowRef.current).toBe(false);
    scrollNode.classList.remove("scrollbar-visible");
    act(() => {
      layout!.clientHeight = 700;
      // The browser clamps to the new maximum before ResizeObserver runs.
      scrollNode!.scrollTop = layout!.scrollTop;
      scrollNode!.dispatchEvent(new Event("scroll"));
    });
    expect(handle.autoFollowRef.current).toBe(false);
    expect(scrollNode.classList.contains("scrollbar-visible")).toBe(false);
    layout.scrollHeight += 200;
    handle.scrollToBottom();
    expect(layout.scrollTop).toBe(500);
  });

  it("does not restore the bottom after a user-triggered layout expansion", () => {
    if (!layout || !handle || !scrollNode) throw new Error("probe not mounted");

    handle.pauseAutoFollow();
    layout.scrollHeight = 1600;
    act(() => {
      scrollNode?.dispatchEvent(new Event("scroll"));
    });
    handle.scrollToBottom();

    expect(handle.autoFollowRef.current).toBe(false);
    expect(layout.scrollTop).toBe(800);
  });
});
