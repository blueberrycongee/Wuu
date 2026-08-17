import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { ThreadItem, Turn } from "../shared/protocol";
import { streamTextKey, streamTextStore } from "./StreamText";
import { ThreadItemView } from "./ThreadItemView";
import { clearToasts, ToastViewport } from "./Toast";
import { desktopPluginHost, desktopWorkbenchController } from "./plugins/DesktopPluginRuntime";
import { setOpenThreadInSplitHandler } from "./ConversationSplitBridge";
import { WuuUIRoot } from "./ui/layers/UILayerHost";

let container: HTMLDivElement | undefined;
let root: Root | undefined;

function makeFinalAnswer(status: ThreadItem["status"]): ThreadItem {
  return {
    id: "final-1",
    type: "agent_message",
    status,
    terminal: true,
    text: "Final answer text.",
  };
}

function makeUserMessage(text: string, id = "user-1"): ThreadItem {
  return {
    id,
    type: "user_message",
    status: "completed",
    text,
  };
}

function render({
  item,
  turnStatus,
  actionableAgentMessageID,
  latestAgentMessageID,
  streaming,
  onEditMessage,
}: {
  item: ThreadItem;
  turnStatus: Turn["status"];
  actionableAgentMessageID?: string;
  latestAgentMessageID?: string;
  streaming: boolean;
  onEditMessage?: (turnID: string, item: ThreadItem) => void;
}): void {
  if (!container) {
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
  }
  act(() => {
    root!.render(
      <WuuUIRoot>
        <ThreadItemView
          turnID="turn-1"
          turnStatus={turnStatus}
          item={item}
          streaming={streaming}
          actionableAgentMessageID={actionableAgentMessageID}
          latestAgentMessageID={latestAgentMessageID}
          onStreamFrame={() => {}}
          onEditMessage={onEditMessage}
        />
        <ToastViewport />
      </WuuUIRoot>,
    );
  });
}

function actionBar(): HTMLElement {
  const node = container?.querySelector<HTMLElement>(".agent-message-actions");
  if (!node) {
    throw new Error("expected agent action bar");
  }
  return node;
}

afterEach(() => {
  act(() => {
    root?.unmount();
  });
  container?.remove();
  streamTextStore.clearItem("turn-1", "final-1");
  clearToasts();
  desktopPluginHost.unload("thread-item-production-test");
  desktopPluginHost.setActiveConversationThread(undefined);
  vi.restoreAllMocks();
  root = undefined;
  container = undefined;
});

describe("ThreadItemView", () => {
  it("mounts sanitized frozen context before and after each conversation message", async () => {
    const contexts: Array<Readonly<Record<string, unknown>>> = [];
    await desktopPluginHost.activateGeneration({
      pluginId: "thread-item-production-test",
      generation: "one",
      register(api) {
        for (const slotId of ["conversation.message.before", "conversation.message.after"] as const) {
          api.registerSlot(slotId, {
            id: slotId,
            render(context) {
              contexts.push(context);
              return <span data-testid={slotId}>{slotId}</span>;
            },
          });
        }
      },
    });

    render({
      item: makeFinalAnswer("completed"),
      turnStatus: "completed",
      streaming: false,
    });

    expect(container?.querySelector('[data-testid="conversation.message.before"]')).not.toBeNull();
    expect(container?.querySelector('[data-testid="conversation.message.after"]')).not.toBeNull();
    expect(contexts).toHaveLength(2);
    expect(contexts[0]).toEqual({
      kind: "agent_message",
      turnStatus: "completed",
      streaming: false,
      editing: false,
      locale: "zh-CN",
      translate: expect.any(Function),
    });
    expect(Object.isFrozen(contexts[0])).toBe(true);
    expect(contexts[0]).not.toHaveProperty("item");
    expect(contexts[0]).not.toHaveProperty("text");
  });

  it("uses a conversation item presenter for the complete production item root", async () => {
    await desktopPluginHost.activateGeneration({
      pluginId: "thread-item-production-test",
      generation: "gen-1",
      register: (api) => {
        api.registerPresenter({
          id: "assistant-root",
          target: "conversation.item",
          key: "assistant-message",
          render: ({ snapshot }) => (
            <section data-production-presenter>
              {(snapshot as { text?: string }).text}
            </section>
          ),
        });
      },
    });
    render({
      item: makeFinalAnswer("completed"),
      turnStatus: "completed",
      streaming: false,
    });

    expect(container?.querySelector("[data-production-presenter]")?.textContent).toBe("Final answer text.");
    expect(container?.querySelector(".agent-block")).toBeNull();
  });

  it("uses the legacy message surface as the exact native fallback with a sanitized context", async () => {
    let received: Readonly<Record<string, unknown>> | undefined;
    await desktopPluginHost.activateGeneration({
      pluginId: "thread-item-production-test",
      generation: "gen-1",
      register: (api) => {
        api.registerSurface("conversation.message", {
          id: "legacy-message",
          mode: "replace",
          render: (context) => {
            received = context;
            return <section data-legacy-message>{String(context.kind)}</section>;
          },
        });
      },
    });

    const item = makeFinalAnswer("completed");
    (item as ThreadItem & { privateValue: string }).privateValue = "must-not-leak";
    render({ item, turnStatus: "completed", streaming: false });

    expect(container?.querySelector("[data-legacy-message]")?.textContent).toBe("assistant-message");
    expect(container?.querySelector(".agent-block")).toBeNull();
    expect(received).toEqual({
      version: 1,
      messageId: "final-1",
      turnId: "turn-1",
      kind: "assistant-message",
      status: "completed",
      phase: "final_answer",
      streaming: false,
      attachmentCount: 0,
      actions: { edit: undefined, fork: undefined },
    });
    expect(received).not.toHaveProperty("item");
    expect(received).not.toHaveProperty("text");
    expect(received).not.toHaveProperty("privateValue");

    act(() => desktopPluginHost.unload("thread-item-production-test"));
    expect(container?.querySelector(".agent-block")).not.toBeNull();
  });

  it("exposes the active thread id on the conversation message surface", async () => {
    let received: Readonly<Record<string, unknown>> | undefined;
    await desktopPluginHost.activateGeneration({
      pluginId: "thread-item-production-test",
      generation: "gen-thread-id",
      register: (api) => {
        api.registerSurface("conversation.message", {
          id: "thread-aware-message",
          mode: "replace",
          render: (context) => {
            received = context;
            return <section data-thread-aware>{String(context.threadId)}</section>;
          },
        });
      },
    });

    desktopPluginHost.setActiveConversationThread("thread-aware-1");
    render({ item: makeFinalAnswer("completed"), turnStatus: "completed", streaming: false });

    expect(container?.querySelector("[data-thread-aware]")?.textContent).toBe("thread-aware-1");
    expect(received).toHaveProperty("threadId", "thread-aware-1");
    desktopPluginHost.setActiveConversationThread(undefined);
  });

  it("lets the keyed presenter replace the legacy surface and native fallback coherently", async () => {
    const legacyRender = vi.fn(() => <section data-legacy-message />);
    await desktopPluginHost.activateGeneration({
      pluginId: "thread-item-production-test",
      generation: "gen-1",
      register: (api) => {
        api.registerSurface("conversation.message", {
          id: "legacy-message",
          mode: "replace",
          render: legacyRender,
        });
        api.registerPresenter({
          id: "assistant-root",
          target: "conversation.item",
          key: "assistant-message",
          render: () => <section data-keyed-presenter />,
        });
      },
    });

    render({
      item: makeFinalAnswer("completed"),
      turnStatus: "completed",
      streaming: false,
    });

    expect(container?.querySelector("[data-keyed-presenter]")).not.toBeNull();
    expect(container?.querySelector("[data-legacy-message]")).toBeNull();
    expect(container?.querySelector(".agent-block")).toBeNull();
    expect(legacyRender).not.toHaveBeenCalled();
  });

  it("shows short user messages in full without a collapse control", () => {
    render({
      item: makeUserMessage("Short query."),
      turnStatus: "completed",
      streaming: false,
    });

    expect(container?.querySelector(".user-message-expand-toggle")).toBeNull();
    expect(container?.querySelector(".user-message-long-card")).toBeNull();
    expect(container?.textContent).toContain("Short query.");
  });

  it("collapses long wrapped user messages without explicit line breaks", () => {
    const longSingleParagraph = "pasted query ".repeat(150);

    render({
      item: makeUserMessage(longSingleParagraph),
      turnStatus: "completed",
      streaming: false,
    });

    const bubble = container?.querySelector<HTMLElement>(".user-message");
    const rawQuery = container?.querySelector<HTMLElement>(
      ".user-message-raw-query",
    );
    const toggle = container?.querySelector<HTMLButtonElement>(
      ".user-message-expand-toggle",
    );
    expect(bubble?.classList.contains("user-message-long-card")).toBe(true);
    expect(bubble?.contains(toggle ?? null)).toBe(true);
    expect(rawQuery?.textContent?.endsWith("...")).toBe(true);
    expect((rawQuery?.textContent?.length ?? 0)).toBeLessThan(
      longSingleParagraph.length,
    );
    expect(toggle?.textContent).toContain("显示更多");
  });

  it("shows long query previews as raw pasted text", () => {
    const markdownQuery = [
      "# Final plan",
      "",
      "## 0. Metadata",
      "",
      "- **Goal**: keep DefaultSystemPrompt() readable",
      "- **Reference**: default.md / gpt_5_1_prompt.md / prompt_with_apply_patch_instructions.md",
      "- **Scope**: internal/config/config.go / internal/runtime/session.go",
      "- **Step**: preserve the copied query shape",
      "- **Step**: avoid rendered markdown changing the preview",
      "- **Step**: keep a full rounded bubble",
      "- **Step**: wrap long paths and identifiers",
      "- **Step**: expose a clear show more control",
      "- **Step**: keep short queries unchanged",
      "- **Step**: keep copy and edit using the full original text",
      "- **Step**: each query starts collapsed",
      "- **Step**: expanded queries can collapse again",
    ].join("\n");

    render({
      item: makeUserMessage(markdownQuery),
      turnStatus: "completed",
      streaming: false,
    });

    const rawQuery = container?.querySelector<HTMLElement>(
      ".user-message-raw-query",
    );
    expect(rawQuery?.textContent).toContain("# Final plan");
    expect(rawQuery?.textContent).toContain("- **Goal**");
    expect(container?.querySelector(".rich-heading")).toBeNull();
  });

  it("collapses long user messages and toggles the full text", () => {
    const longText = Array.from(
      { length: 20 },
      (_, index) => `line ${index + 1}`,
    ).join("\n");

    render({
      item: makeUserMessage(longText),
      turnStatus: "completed",
      streaming: false,
    });

    const toggle = container?.querySelector<HTMLButtonElement>(
      ".user-message-expand-toggle",
    );
    const rawQuery = container?.querySelector<HTMLElement>(
      ".user-message-raw-query",
    );

    expect(rawQuery?.textContent).not.toContain("line 20");
    expect(rawQuery?.textContent?.endsWith("...")).toBe(true);
    expect(toggle?.textContent).toContain("显示更多");
    expect(toggle?.getAttribute("aria-expanded")).toBe("false");

    act(() => {
      toggle?.click();
    });

    expect(rawQuery?.textContent).toContain("line 20");
    expect(toggle?.textContent).toContain("收起");
    expect(toggle?.getAttribute("aria-expanded")).toBe("true");

    act(() => {
      toggle?.click();
    });

    expect(rawQuery?.textContent).not.toContain("line 20");
    expect(toggle?.textContent).toContain("显示更多");
  });

  it("defaults a different long user query back to collapsed", () => {
    const firstLongText = Array.from(
      { length: 20 },
      (_, index) => `first query line ${index + 1}`,
    ).join("\n");
    const secondLongText = Array.from(
      { length: 20 },
      (_, index) => `second query line ${index + 1}`,
    ).join("\n");

    render({
      item: makeUserMessage(firstLongText, "user-1"),
      turnStatus: "completed",
      streaming: false,
    });

    const firstToggle = container?.querySelector<HTMLButtonElement>(
      ".user-message-expand-toggle",
    );
    act(() => {
      firstToggle?.click();
    });

    expect(
      container?.querySelector(".user-message-raw-query")?.textContent,
    ).toContain("first query line 20");

    render({
      item: makeUserMessage(secondLongText, "user-2"),
      turnStatus: "completed",
      streaming: false,
    });

    const secondToggle = container?.querySelector<HTMLButtonElement>(
      ".user-message-expand-toggle",
    );
    expect(
      container?.querySelector(".user-message-raw-query")?.textContent,
    ).not.toContain("second query line 20");
    expect(secondToggle?.getAttribute("aria-expanded")).toBe("false");
  });

  it("reserves no action bar while streaming and mounts the persistent bar on completion", () => {
    render({
      item: makeFinalAnswer("in_progress"),
      turnStatus: "in_progress",
      streaming: true,
    });

    // Streaming answers render no bar at all — the old invisible
    // placeholder reserved 32px of dead space under live text.
    expect(container?.querySelector(".agent-message-actions")).toBeNull();

    render({
      item: makeFinalAnswer("completed"),
      turnStatus: "completed",
      actionableAgentMessageID: "final-1",
      latestAgentMessageID: "final-1",
      streaming: false,
    });

    const visibleActions = actionBar();
    expect(visibleActions.getAttribute("aria-label")).toBe("助手消息操作");
    expect(visibleActions.dataset.wuuComponent).toBe("message-actions");
    expect(visibleActions.dataset.wuuPlacement).toBe("persistent");
    expect(visibleActions.querySelectorAll("button")).toHaveLength(2);
    const block = container?.querySelector(".agent-block");
    expect(block?.classList.contains("agent-actions-persistent")).toBe(true);
    expect(block?.classList.contains("agent-actions-overlay")).toBe(false);
  });

  it("renders historical answers with a hover overlay bar instead of an in-flow slot", () => {
    render({
      item: makeFinalAnswer("completed"),
      turnStatus: "completed",
      actionableAgentMessageID: "final-1",
      latestAgentMessageID: "final-99",
      streaming: false,
    });

    expect(actionBar().dataset.wuuPlacement).toBe("overlay");
    const block = container?.querySelector(".agent-block");
    expect(block?.classList.contains("agent-actions-overlay")).toBe(true);
    expect(block?.classList.contains("agent-actions-persistent")).toBe(false);
  });

  it("releases completed stream text after the settled view keeps the final answer", async () => {
    const key = streamTextKey("turn-1", "final-1", "text");
    streamTextStore.set(key, "Final answer text.");

    render({
      item: makeFinalAnswer("completed"),
      turnStatus: "completed",
      actionableAgentMessageID: "final-1",
      latestAgentMessageID: "final-1",
      streaming: false,
    });

    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 50));
    });

    expect(container?.textContent).toContain("Final answer text.");
    expect(streamTextStore.has(key)).toBe(false);
  });

  it("renders a plugin-generated query as a read-only user message", () => {
    const onEditMessage = vi.fn();
    render({
      item: {
        id: "plugin-query-1",
        type: "user_message",
        text: "子任务 太阳 已更新",
        read_only: true,
        origin: "plugin",
        origin_id: "subagent",
        cause: "subagent.completion",
        presentation_kind: "query_bubble",
      },
      turnStatus: "completed",
      streaming: false,
      onEditMessage,
    });

    expect(container?.querySelector(".user-message")?.textContent).toBe("子任务 太阳 已更新");
    const actions = container?.querySelectorAll<HTMLButtonElement>(".user-message-actions button");
    expect(actions).toHaveLength(1);
    expect(container?.querySelector<HTMLElement>(".user-message-actions")?.dataset.wuuPlacement).toBe("overlay");
    expect(onEditMessage).not.toHaveBeenCalled();
  });

  it("does not show a details action when input_text equals the bubble text", () => {
    render({
      item: {
        id: "user-own-1",
        type: "user_message",
        text: "这是我的普通消息",
        // Stale server projections used to leak the content back as
        // input_text for ordinary messages; the action must stay hidden.
        input_text: "这是我的普通消息",
        status: "completed",
      },
      turnStatus: "completed",
      streaming: false,
    });

    const actions = container?.querySelectorAll<HTMLButtonElement>(".user-message-actions button");
    expect(actions).toHaveLength(1);
    expect([...(actions ?? [])].some((button) => button.getAttribute("aria-label") === "投递详情")).toBe(false);
  });

  it("opens the delivery inspector from the details action on hidden wake prompts", async () => {
    const openPluginView = vi
      .spyOn(desktopWorkbenchController, "openPluginView")
      .mockResolvedValue("delivery:delivery.inspector:1");
    await desktopPluginHost.activateGeneration({
      pluginId: "delivery",
      generation: "one",
      register(api) {
        api.registerViewType({ id: "delivery.inspector", title: "Delivery details", render: () => null });
      },
    });
    render({
      item: {
        id: "plugin-query-1",
        type: "user_message",
        text: "子任务 太阳 已更新",
        input_text: "这是实际投递给子任务 太阳 的完整提示词",
        read_only: true,
        origin: "plugin",
        origin_id: "subagent",
        cause: "subagent.completion",
        presentation_kind: "query_bubble",
      },
      turnStatus: "completed",
      streaming: false,
    });

    const actions = container?.querySelectorAll<HTMLButtonElement>(".user-message-actions button");
    expect(actions).toHaveLength(2);
    const details = [...(actions ?? [])].find((button) => button.getAttribute("aria-label") === "投递详情");
    expect(details).toBeDefined();
    // The details action matches the copy/edit action buttons: same box and
    // same 15px icon so the row stays visually uniform.
    expect(details?.querySelector("svg")?.getAttribute("width")).toBe("15");
    expect(details?.className).toContain("message-action-button");

    act(() => details!.click());
    expect(openPluginView).toHaveBeenCalledWith("delivery", "delivery.inspector", expect.objectContaining({
      region: "auxiliary",
      context: expect.objectContaining({
        messageId: "plugin-query-1",
        displayText: "子任务 太阳 已更新",
        inputText: "这是实际投递给子任务 太阳 的完整提示词",
      }),
    }));

    act(() => desktopPluginHost.unload("delivery"));
  });

  it("splits the conversation and opens the child session from the details action", () => {
    const openInSplit = vi.fn();
    const openPluginView = vi.spyOn(desktopWorkbenchController, "openPluginView");
    setOpenThreadInSplitHandler(openInSplit);
    render({
      item: {
        id: "plugin-query-1",
        type: "user_message",
        text: "子任务 太阳 已更新",
        input_text: "子任务 太阳（session 20260817-171746-edd1069c0780f11a）已完成。请检查并整合以下交接结果：\n\n三个命令均已成功执行",
        read_only: true,
        origin: "plugin",
        origin_id: "subagent",
        cause: "subagent.completion",
        presentation_kind: "query_bubble",
      },
      turnStatus: "completed",
      streaming: false,
    });

    const actions = container?.querySelectorAll<HTMLButtonElement>(".user-message-actions button");
    expect(actions).toHaveLength(2);
    const details = [...(actions ?? [])].find((button) => button.getAttribute("aria-label") === "投递详情");
    expect(details).toBeDefined();

    act(() => details!.click());
    expect(openInSplit).toHaveBeenCalledWith("20260817-171746-edd1069c0780f11a");
    expect(openPluginView).not.toHaveBeenCalled();

    act(() => setOpenThreadInSplitHandler(undefined));
  });

});
