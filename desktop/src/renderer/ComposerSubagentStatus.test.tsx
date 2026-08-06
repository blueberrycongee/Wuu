import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Agent } from "../shared/protocol";
import {
  ComposerSubagentStatus,
  runningSubagents,
} from "./ComposerSubagentStatus";
import { hoverTooltipText, unhoverTooltip } from "./tooltipTestUtils";

let container: HTMLDivElement;
let root: Root;

function agent(id: string, status: string, name?: string): Agent {
  return {
    id,
    agent_path: `/root/${id}`,
    parent_id: "root",
    status,
    participant: name ? { id: `participant-${id}`, name, kind: "agent" } : undefined,
  } as Agent;
}

beforeEach(() => {
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  unhoverTooltip();
  act(() => root.unmount());
  container.remove();
});

describe("ComposerSubagentStatus", () => {
  it("keeps only agents that are truly still running", () => {
    expect(
      runningSubagents([
        agent("one", "running"),
        agent("two", "completed"),
        { ...agent("three", "waiting_children"), nested_running_count: 2 },
      ]).map((item) => item.id),
    ).toEqual(["one", "three"]);
  });

  it("stays hidden after all subagents settle", () => {
    act(() => {
      root.render(
        <ComposerSubagentStatus
          agents={[agent("one", "completed"), agent("two", "failed")]}
          onSelect={() => undefined}
          onOpenAll={() => undefined}
        />,
      );
    });

    expect(container.querySelector(".composer-subagent-capsule")).toBeNull();
  });

  it("shows each name on hover and opens that agent conversation", async () => {
    const ada = agent("one", "running", "Ada");
    const grace = agent("two", "running", "Grace");
    const onSelect = vi.fn();
    act(() => {
      root.render(
        <ComposerSubagentStatus
          agents={[ada, grace]}
          onSelect={onSelect}
          onOpenAll={() => undefined}
        />,
      );
    });

    const buttons = Array.from(
      container.querySelectorAll<HTMLButtonElement>(
        ".composer-subagent-avatar-button",
      ),
    );
    expect(buttons).toHaveLength(2);
    expect(await hoverTooltipText(buttons[0])).toBe("Ada");
    act(() => buttons[1].click());
    expect(onSelect).toHaveBeenCalledWith(grace);
  });
});
