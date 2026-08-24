import { act, type ComponentProps } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, describe, expect, it } from "vitest";
import type { Thread } from "../shared/protocol";
import { CachedConversationPanes } from "./CachedConversationPanes";
import { retainCachedConversationPaneThreads } from "./ConversationPaneCache";
import { ImagePreviewProvider } from "./ImagePreview";

let roots: Root[] = [];

afterEach(() => {
  for (const root of roots) {
    act(() => root.unmount());
  }
  roots = [];
  document.body.innerHTML = "";
});

function longThread(id: string, turnCount: number): Thread {
  return {
    id,
    title: id,
    preview: id,
    model_provider: "test",
    model: "test",
    cwd: `/tmp/${id}`,
    status: "idle",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    turns: Array.from({ length: turnCount }, (_, index) => ({
      id: `${id}-turn-${index}`,
      status: "completed",
      items_view: "full",
      items: [
        {
          id: `${id}-user-${index}`,
          type: "user_message",
          text: `${id} question ${index}`,
        },
        {
          id: `${id}-agent-${index}`,
          type: "agent_message",
          text: `${id} answer ${index}`,
        },
      ],
    })),
  } as Thread;
}

describe("CachedConversationPanes real message tree", () => {
  it("keeps a long cross-workspace pane mounted and reveals it synchronously", () => {
    const source = longThread("source-workspace", 200);
    const target = longThread("target-workspace", 200);
    const container = document.createElement("div");
    document.body.appendChild(container);
    const root = createRoot(container);
    roots.push(root);

    const stableProps = {
      contextCompositionEntries: [],
      instructionFilesEntries: [],
      onStreamFrame: () => {},
      onCollapseComplete: () => {},
      onDismissContextComposition: () => {},
      onDismissInstructions: () => {},
      canEditThreadMessage: () => false,
      onForkMessage: () => {},
      onOpenAgent: () => {},
      onEditMessage: () => {},
      onCancelEditMessage: () => {},
      onSubmitEditMessage: () => {},
      onOpenFileDiff: () => {},
      turnStreamStatus: {},
    } satisfies Omit<
      ComponentProps<typeof CachedConversationPanes>,
      "activeThreadID" | "threadIDs" | "threadsByID"
    >;
    let paneThreads = retainCachedConversationPaneThreads({
      threadIDs: [source.id, target.id],
      currentThreadsByID: new Map([
        [source.id, source],
        [target.id, target],
      ]),
      previousThreadsByID: new Map(),
    });
    const renderActive = (activeThreadID: string): void => {
      act(() => {
        root.render(
          <ImagePreviewProvider>
            <CachedConversationPanes
              {...stableProps}
              activeThreadID={activeThreadID}
              threadIDs={[target.id, source.id]}
              threadsByID={paneThreads}
            />
          </ImagePreviewProvider>,
        );
      });
    };

    renderActive(source.id);
    const sourcePane = container.querySelector<HTMLElement>(
      `[data-thread-id="${source.id}"]`,
    );
    const sourceLastTurn = sourcePane?.querySelector<HTMLElement>(
      `[data-turn-id="${source.id}-turn-199"]`,
    );
    expect(sourcePane?.querySelectorAll(".turn")).toHaveLength(40);

    paneThreads = retainCachedConversationPaneThreads({
      threadIDs: [target.id, source.id],
      currentThreadsByID: new Map([[target.id, target]]),
      previousThreadsByID: paneThreads,
    });
    renderActive(target.id);
    expect(sourcePane?.getAttribute("data-active")).toBe("false");
    expect(sourcePane?.isConnected).toBe(true);

    renderActive(source.id);
    expect(sourcePane?.getAttribute("data-active")).toBe("true");
    expect(sourcePane?.hasAttribute("inert")).toBe(false);
    expect(
      sourcePane?.querySelector(`[data-turn-id="${source.id}-turn-199"]`),
    ).toBe(sourceLastTurn);
  });
});
