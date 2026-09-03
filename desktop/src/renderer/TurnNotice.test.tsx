/**
 * Tests for `ContextCompactionNotice`'s process-activity row rendering.
 *
 * These tests pin the shared process-surface classes, visible detail,
 * active sweep target, and failed-state semantics.
 */
import { act, type ReactElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { ContextCompactionNotice, StreamStatusNotice, TurnNotice } from "./TurnNotice";
import { userFacingErrorForMessage } from "./UserFacingErrors";
import { setActiveLocale } from "./i18n";

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

  it("does not render a completed compaction attempt that changed nothing", () => {
    const host = mount(
      <ContextCompactionNotice
        status="completed"
        reason="proactive"
        text="Nothing to compact yet; history is unchanged."
      />,
    );

    expect(host.childElementCount).toBe(0);
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

describe("TurnNotice process row", () => {
  it("uses the shared process surface and keeps the full detail expandable", () => {
    const display = userFacingErrorForMessage("connection reset by peer", "turn");
    const host = mount(<TurnNotice display={display} />);
    const aside = host.querySelector("aside.system-event-notice");
    expect(aside).not.toBeNull();
    expect(aside?.querySelector(".process-surface-row")).not.toBeNull();
    expect(aside?.querySelector(".system-event-title")?.textContent).toBe(display.title);
    expect(aside?.querySelector(".system-event-detail")?.textContent).toBe(display.detail);
    expect(aside?.querySelector(".system-event-expanded-detail")?.textContent).toBe(display.detail);
    expect(aside?.getAttribute("aria-label")).toContain(display.title);
    expect(aside?.getAttribute("aria-label")).toContain(display.detail);
    expect(aside?.querySelector(".process-surface-chevron")).not.toBeNull();
  });

  it("renders cancellation in the same neutral process row", () => {
    const display = userFacingErrorForMessage("context canceled", "turn");
    const host = mount(<TurnNotice display={display} />);
    const aside = host.querySelector("aside.system-event-notice");
    expect(aside?.querySelector(".system-event-title")?.textContent).toBe(display.title);
    expect(aside?.classList.contains("neutral")).toBe(false);
  });

  it("keeps alert semantics without applying a colored tone class", () => {
    const display = userFacingErrorForMessage("401 unauthorized", "turn");
    const host = mount(<TurnNotice display={display} />);
    const aside = host.querySelector("aside.system-event-notice");
    expect(aside?.getAttribute("role")).toBe("alert");
    expect(aside?.classList.contains("auth")).toBe(false);
  });

  it("shows retry-only progress and counts down in one live process row", () => {
    const retryAtMs = Date.now() + 2_000;
    const host = mount(
      <StreamStatusNotice
        status={{
          text: "429 触发限流 · 第 2/5 次重试 · 2 秒后重试",
          liveProgress: true,
          event: {
            label: "429 触发限流",
            retryCount: 2,
            maxRetries: 5,
            retryAtMs,
          },
        }}
      />,
    );

    const notice = host.querySelector("aside.stream-status-notice");
    expect(notice?.querySelectorAll(".process-surface-row")).toHaveLength(1);
    expect(notice?.textContent).toContain("429 触发限流");
    expect(notice?.textContent).toContain("第 2/5 次重试");
    expect(notice?.textContent).toContain("2 秒后重试");
    expect(notice?.textContent).not.toContain("尝试");
    expect(notice?.textContent).not.toContain("已发送");

    act(() => {
      vi.advanceTimersByTime(1_000);
    });
    expect(notice?.textContent).toContain("1 秒后重试");

    act(() => {
      vi.advanceTimersByTime(1_000);
    });
    expect(notice?.textContent).toContain("正在重试");
    expect(notice?.querySelector(".process-surface-chevron")).toBeNull();
  });
});
