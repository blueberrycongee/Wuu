import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { EngineListResult } from "../shared/protocol";
import { EngineSettingsSection } from "./EngineSettingsSection";

const inventory: EngineListResult = {
  engines: [
    { id: "wuu", enabled: true, binary_ok: true },
    { id: "codex", enabled: true, binary_ok: true },
    { id: "claude", enabled: true, binary_ok: true },
  ],
  settings: { default_engine: "wuu" },
};

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

describe("EngineSettingsSection", () => {
  it("renders a supplied session snapshot without starting detection on mount", () => {
    const onRefresh = vi.fn();
    act(() => {
      root.render(
        <EngineSettingsSection
          result={inventory}
          onRefresh={onRefresh}
          onUpdate={vi.fn()}
        />,
      );
    });

    expect(container.querySelector('[data-testid="settings-default-engine"]')).not.toBeNull();
    expect(container.querySelector('[data-testid="settings-engine-codex-status"]')?.textContent).toContain("已就绪");
    expect(onRefresh).not.toHaveBeenCalled();
  });

  it("keeps the cached snapshot visible while the user explicitly refreshes", async () => {
    let resolveRefresh: ((value: EngineListResult) => void) | undefined;
    const onRefresh = vi.fn(() => new Promise<EngineListResult>((resolve) => {
      resolveRefresh = resolve;
    }));
    act(() => {
      root.render(
        <EngineSettingsSection
          result={inventory}
          onRefresh={onRefresh}
          onUpdate={vi.fn()}
        />,
      );
    });

    const refresh = container.querySelector<HTMLButtonElement>('[data-testid="settings-engine-refresh"]')!;
    await act(async () => {
      refresh.click();
    });

    expect(onRefresh).toHaveBeenCalledOnce();
    expect(refresh.disabled).toBe(true);
    expect(container.querySelector('[data-testid="settings-default-engine"]')).not.toBeNull();

    await act(async () => {
      resolveRefresh?.(inventory);
    });
    expect(refresh.disabled).toBe(false);
  });

  it("uses a stable skeleton only when no session snapshot exists", () => {
    act(() => {
      root.render(
        <EngineSettingsSection
          onRefresh={vi.fn()}
          onUpdate={vi.fn()}
        />,
      );
    });

    expect(container.querySelector('[aria-busy="true"]')).not.toBeNull();
    expect(container.querySelector('[data-testid="settings-default-engine"]')).toBeNull();
    expect(container.querySelector('[data-testid="settings-engine-refresh"]')).not.toBeNull();
  });
});
