import * as React from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ConversationStatusCluster } from "./ConversationStatusCluster";
import { PluginHost } from "./plugins/PluginHost";

describe("ConversationStatusCluster", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(() => {
    act(() => root.unmount());
    container.remove();
  });

  it("closes the overflow menu when the user clicks outside", async () => {
    const host = new PluginHost({ react: React });
    await host.activateGeneration({
      pluginId: "status-test",
      generation: "one",
      register(api) {
        api.registerComposerStatusSource({
          id: "agents",
          getSnapshot: () => [
            { id: "one", label: "agent_one", state: "running" },
            { id: "two", label: "agent_two", state: "running" },
            { id: "three", label: "agent_three", state: "running" },
            { id: "four", label: "agent_four", state: "running" },
          ],
          subscribe: () => () => undefined,
        });
      },
    });

    act(() => root.render(
      <ConversationStatusCluster
        host={host}
        visible
        threadId="parent-session"
        todoUpdate={undefined}
        onOpenSession={vi.fn()}
      />,
    ));

    const overflow = container.querySelector<HTMLDetailsElement>(".conversation-status-overflow");
    const trigger = overflow?.querySelector<HTMLElement>("summary");
    expect(overflow).not.toBeNull();
    expect(trigger).not.toBeNull();

    act(() => trigger?.click());
    expect(overflow?.open).toBe(true);

    act(() => {
      document.body.dispatchEvent(new MouseEvent("pointerdown", { bubbles: true }));
    });
    expect(overflow?.open).toBe(false);
  });
});
