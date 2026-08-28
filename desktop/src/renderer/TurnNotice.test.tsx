/**
 * Tests for `ContextCompactionNotice`'s process-activity row rendering.
 *
 * These tests pin the shared process-surface classes, visible detail,
 * active sweep target, and failed-state semantics.
 */
import { act, type ReactElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { ContextCompactionNotice, TurnNotice } from "./TurnNotice";
import { userFacingErrorForMessage } from "./UserFacingErrors";
import { setActiveLocale } from "./i18n";
import { hoverTooltipText, unhoverTooltip } from "./tooltipTestUtils";

beforeAll(() => {
  // jsdom does not lay out real heights. Stub getBoundingClientRect so
  // React's effects do not crash on layout queries.
  Element.prototype.getBoundingClientRect = function (): DOMRect {
    return {
      x: 0,
      y: 0,
      top: 0,
      left: 0,
      right: 0,
      bottom: 0,
      width: 0,
      height: 0,
      toJSON() {
        return this;
      },
    } as DOMRect;
  };
});

let root: Root | null = null;
let container: HTMLDivElement | null = null;

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  setActiveLocale("zh-CN");
  unhoverTooltip();
  if (root) {
    act(() => {
      root?.unmount();
    });
    root = null;
  }
  if (container) {
    container.remove();
    container = null;
  }
  vi.useRealTimers();
});

function mount(element: ReactElement): HTMLDivElement {
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
  act(() => {
    root?.render(element);
  });
  return container;
}

describe("ContextCompactionNotice", () => {
  it("localizes recognized compaction events", () => {
    setActiveLocale("en-US");
    const host = mount(
      <ContextCompactionNotice
        status="completed"
        text="Compacted history: 18 → 5 messages"
      />,
    );

    expect(host.querySelector(".context-compaction-title")?.textContent).toBe("Context compacted");
    expect(host.querySelector(".context-compaction-detail")?.textContent).toContain("18 messages became 5");
  });
  it("renders the in_progress host with the shimmer-ready label when status is in_progress", () => {
    const host = mount(<ContextCompactionNotice status="in_progress" />);
    const aside = host.querySelector("aside.process-surface.context-compaction-notice");
    expect(aside).not.toBeNull();
    expect(aside?.classList.contains("in_progress")).toBe(true);
    expect(aside?.getAttribute("role")).toBe("status");
    expect(aside?.getAttribute("aria-live")).toBe("polite");

    const label = host.querySelector(".context-compaction-title");
    expect(label).not.toBeNull();
    expect(label?.textContent).toBe("正在自动压缩上下文");
    expect(host.querySelector(".process-surface-row.is-live-gray")).not.toBeNull();
    expect(
      host.querySelector(".process-surface-blobatar")?.getAttribute("data-wuu-mascot-activity"),
    ).toBe("compact");
    expect(host.querySelector(".context-compaction-mascot")).not.toBeNull();
    expect(host.querySelector(".context-compaction-black-hole")).toBeNull();
    expect(host.querySelector(".context-compaction-copy")).not.toBeNull();
    expect(host.querySelector(".process-surface-summary-line")?.getAttribute("aria-label")).toBe(
      "正在自动压缩上下文",
    );
    expect(host.querySelector(".turn-event-notice")).toBeNull();
  });

  it("uses the manual compact progress label for slash compact", () => {
    const host = mount(
      <ContextCompactionNotice
        status="in_progress"
        reason="manual"
        text="Manual context compaction in progress."
      />,
    );

    expect(host.querySelector(".context-compaction-title")?.textContent).toBe(
      "正在压缩上下文",
    );
    expect(host.querySelector(".context-compaction-notice.in_progress .process-surface-row.is-live-gray")).not.toBeNull();
  });

  it("renders the process activity row with visible detail when status is completed", () => {
    const host = mount(
      <ContextCompactionNotice
        status="completed"
        text="✦ Compacted history: 18 → 5 messages (was ~12k tokens)"
      />,
    );
    const aside = host.querySelector("aside.process-surface.context-compaction-notice");
    expect(aside).not.toBeNull();
    expect(aside?.classList.contains("completed")).toBe(true);
    expect(aside?.getAttribute("aria-live")).toBeNull();

    const title = host.querySelector(".context-compaction-title");
    expect(title?.textContent).toBe("上下文已压缩");

    expect(host.querySelector(".context-compaction-detail")?.textContent).toContain("18 条消息整理为 5 条");
    expect(host.querySelector(".process-surface-row.is-live-gray")).toBeNull();
    expect(host.querySelector(".context-compaction-mascot")).toBeNull();
  });

  it("shows both the old and replacement context token estimates", () => {
    const host = mount(
      <ContextCompactionNotice
        status="completed"
        text="✦ Compacted history: 119 → 1 messages (~239k → ~49k tokens)"
      />,
    );

    expect(host.querySelector(".context-compaction-detail")?.textContent).toBe(
      "已压缩较早上下文：119 条消息整理为 1 条，token 约从 239k 降至 49k",
    );
  });

  it("labels manual compact completion as success", () => {
    const host = mount(
      <ContextCompactionNotice
        status="completed"
        reason="manual"
        text="✦ Manually compacted history: 18 → 5 messages (was ~12k tokens)"
      />,
    );

    expect(host.querySelector(".context-compaction-title")?.textContent).toBe(
      "压缩成功",
    );
    expect(host.querySelector(".context-compaction-detail")?.textContent).toContain(
      "18 条消息整理为 5 条",
    );
  });

  it("labels failed manual compact status as failed", () => {
    const host = mount(
      <ContextCompactionNotice
        status="failed"
        reason="manual"
        text="Manual context compaction failed; history is unchanged."
      />,
    );

    expect(host.querySelector(".context-compaction-title")?.textContent).toBe(
      "压缩失败",
    );
    expect(host.querySelector(".context-compaction-detail")?.textContent).toContain(
      "当前对话仍保留原上下文",
    );
    expect(host.querySelector("aside")?.classList.contains("failed")).toBe(true);
    expect(host.querySelector("aside")?.getAttribute("role")).toBe("alert");
  });

  it("falls back to the completed layout when status is omitted", () => {
    const host = mount(<ContextCompactionNotice text="" />);
    const aside = host.querySelector("aside.process-surface.context-compaction-notice");
    expect(aside?.classList.contains("completed")).toBe(true);
    expect(host.querySelector(".context-compaction-title")?.textContent).toBe(
      "上下文已压缩",
    );
  });

  it("uses the same completed title for overflow recovery compaction", () => {
    const host = mount(
      <ContextCompactionNotice
        status="completed"
        text="Recovered from context overflow — compacted history: 18 → 5 messages (was ~12k tokens)"
      />,
    );

    expect(host.querySelector(".context-compaction-title")?.textContent).toBe(
      "上下文已压缩",
    );
  });

  it("does not present failed proactive compaction as a successful compact", () => {
    const failedCompactText =
      "Context compaction failed; continuing without compacting history.";
    const host = mount(
      <ContextCompactionNotice
        status="completed"
        text={failedCompactText}
      />,
    );

    expect(host.querySelector(".context-compaction-title")?.textContent).toBe(
      "压缩失败",
    );
    expect(host.querySelector(".context-compaction-detail")?.textContent).toContain(
      "当前对话仍保留原上下文",
    );
    expect(host.textContent).not.toContain("上下文已压缩");
  });
});

describe("TurnNotice compact chip", () => {
  it("renders the title as the only visible label and moves the full detail into a hover tooltip", async () => {
    const display = userFacingErrorForMessage("connection reset by peer", "turn");
    const host = mount(<TurnNotice display={display} />);
    const aside = host.querySelector("aside.turn-event-notice");
    expect(aside).not.toBeNull();
    // Tone class drives the visual treatment.
    expect(aside?.classList.contains(display.tone)).toBe(true);
    // The title element shows only the short title (single-line).
    expect(host.querySelector(".turn-event-title")?.textContent).toBe(display.title);
    // The full detail is reachable via a hover tooltip (no native `title`
    // dump) and mirrored on `aria-label` for assistive tech.
    expect(aside?.getAttribute("title")).toBeNull();
    const tooltip = await hoverTooltipText(aside);
    expect(tooltip).toContain(display.title);
    expect(tooltip).toContain(display.detail);
    expect(aside?.getAttribute("aria-label")).toContain(display.title);
    expect(aside?.getAttribute("aria-label")).toContain(display.detail);
    expect(host.querySelectorAll("button, a")).toHaveLength(0);
    expect(host.querySelectorAll(".turn-event-title")).toHaveLength(1);
  });

  it("renders cancellation as a read-only event", async () => {
    const display = userFacingErrorForMessage("context canceled", "turn");
    const host = mount(<TurnNotice display={display} />);
    expect(host.querySelectorAll("button, a")).toHaveLength(0);
    // The host still carries the hover text and title element.
    const aside = host.querySelector("aside.turn-event-notice");
    expect(await hoverTooltipText(aside)).toContain(display.title);
    expect(host.querySelector(".turn-event-title")?.textContent).toBe(display.title);
  });

  it("applies the auth tone to the host aside", () => {
    const display = userFacingErrorForMessage("401 unauthorized", "turn");
    const host = mount(<TurnNotice display={display} />);
    const aside = host.querySelector("aside.turn-event-notice");
    expect(aside?.classList.contains("auth")).toBe(true);
    // Auth is the only category that resolves to the "auth" tone today.
    expect(display.tone).toBe("auth");
  });
});
