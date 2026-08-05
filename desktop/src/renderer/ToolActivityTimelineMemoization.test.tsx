import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { ThreadItem } from "../shared/protocol";

const textProbe = vi.hoisted(() => ({ renders: 0 }));

vi.mock("./LightweightStreamingText", () => ({
  LightweightStreamingText: ({ text }: { text: string }): JSX.Element => {
    textProbe.renders += 1;
    return <span>{text}</span>;
  },
}));

import { ToolActivityTimeline } from "./ToolActivity";

let container: HTMLDivElement | undefined;
let root: Root | undefined;

afterEach(() => {
  if (root) {
    act(() => root?.unmount());
  }
  root = undefined;
  container?.remove();
  container = undefined;
  textProbe.renders = 0;
});

function readTool(id: string, path: string, status: "completed" | "in_progress"): ThreadItem {
  return {
    id,
    type: "tool_call",
    name: "read_file",
    status,
    arguments: JSON.stringify({ path }),
  };
}

describe("ToolActivityTimeline memoization", () => {
  it("renders only the appended row when a long process timeline grows", () => {
    const first = readTool("tool-1", "first.ts", "completed");
    const second = readTool("tool-2", "second.ts", "completed");
    const third = readTool("tool-3", "third.ts", "in_progress");
    const render = (items: ThreadItem[]): void => {
      root?.render(<ToolActivityTimeline items={items} streaming />);
    };

    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);

    act(() => render([first, second]));
    expect(textProbe.renders).toBe(2);

    act(() => render([first, second, third]));
    expect(textProbe.renders).toBe(3);
  });
});
