import { act, createRef } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import type { InitializeResult } from "../shared/protocol";
import { initialState, type AppState } from "./AppState";
import { environmentPanelScaleForWidth } from "./EnvironmentPanelScale";
import {
  EnvironmentSideStack,
} from "./EnvironmentSideStack";
import { unhoverTooltip } from "./tooltipTestUtils";

let container: HTMLDivElement;
let root: Root | null = null;

beforeEach(() => {
  container = document.createElement("div");
  document.body.appendChild(container);
});

afterEach(() => {
  unhoverTooltip();
  act(() => {
    root?.unmount();
  });
  root = null;
  container.remove();
});

function initialized(): InitializeResult {
  return {
    protocol_version: "wuu-app-server/v0.1",
    provider: "test",
    model: "test-model",
    workspace_root: "/repo",
  };
}

function renderStack(stateOverrides: Partial<AppState> = {}): void {
  const state: AppState = {
    ...initialState,
    initialized: initialized(),
    ...stateOverrides,
  };

  act(() => {
    root = createRoot(container);
    root.render(
      <EnvironmentSideStack
        visible={true}
        mounted={true}
        state={state}
        panelRef={createRef<HTMLDivElement>()}
        closing={false}
        motionState="open"
        activeMenu={null}
        running={false}
        pullRequestDisabledReason=""
        onSetActiveMenu={() => {}}
        onClose={() => {}}
        onSelectBranch={() => {}}
        onCreateBranch={() => Promise.resolve()}
        onOpenReview={() => {}}
        onOpenCommit={() => {}}
        onOpenPullRequest={() => {}}
      />,
    );
  });
}

describe("EnvironmentSideStack", () => {
  it("renders the environment panel without a docked query-history card", () => {
    renderStack();

    expect(container.querySelector(".environment-info-side-stack")).not.toBeNull();
    expect(container.querySelector(".environment-side-stack > .environment-panel")).not.toBeNull();
    expect(container.querySelector(".environment-side-stack > .query-history-environment-slot")).toBeNull();
    expect(container.querySelector(".query-history-popover")).toBeNull();
  });

  it("scales the complete environment panel at narrow widths", () => {
    Object.defineProperty(container, "clientWidth", {
      configurable: true,
      value: 420,
    });
    renderStack();

    expect(environmentPanelScaleForWidth(560)).toBe(0.8);
    expect(environmentPanelScaleForWidth(420)).toBe(0.6);
    expect(environmentPanelScaleForWidth(320)).toBe(0.576);
    expect(environmentPanelScaleForWidth(1_000)).toBe(0.8);
    expect(environmentPanelScaleForWidth(1_200)).toBe(0.9);
    expect(environmentPanelScaleForWidth(1_400)).toBe(1);

    const stack = container.querySelector<HTMLElement>(
      ".environment-info-side-stack",
    );
    expect(stack?.style.getPropertyValue("--environment-panel-scale")).toBe(
      "0.6",
    );
  });
});
