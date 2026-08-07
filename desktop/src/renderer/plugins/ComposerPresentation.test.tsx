// @vitest-environment jsdom
import * as React from "react";
import { act, createRef, useState, type ComponentProps } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, describe, expect, it, vi } from "vitest";

import { COMPOSER_ACTIONS, type ComposerSnapshotV1, type PresentationHost } from "../../shared/workbench";
import { ComposerPresentation, buildComposerSnapshot } from "./ComposerPresentation";
import { PluginHost } from "./PluginHost";
import { WorkbenchController } from "./Workbench";

let root: Root | undefined;
let container: HTMLDivElement | undefined;

afterEach(() => {
  act(() => root?.unmount());
  container?.remove();
  root = undefined;
  container = undefined;
});

describe("ComposerPresentation", () => {
  it("replaces the complete native root, publishes live snapshots, and falls back on unload", async () => {
    const { host, controller } = setup();
    let snapshot: ComposerSnapshotV1 | undefined;
    await host.activateGeneration({ pluginId: "composer", generation: "one", register(api) {
      api.registerPresenter({ id: "composer", target: "conversation.composer", render: (props) => {
        snapshot = props.snapshot as ComposerSnapshotV1;
        return <section data-presenter-root>{snapshot.draftText}</section>;
      } });
    } });
    let setDraft: ((value: string) => void) | undefined;
    function Harness(): JSX.Element {
      const [draftText, updateDraft] = useState("first");
      setDraft = updateDraft;
      return <ComposerPresentation {...baseProps(host, controller)} draftText={draftText}
        onSetDraft={updateDraft} fallback={<footer data-native-root>native</footer>} />;
    }
    render(<Harness />);
    expect(container?.querySelector("[data-presenter-root]")?.textContent).toBe("first");
    expect(container?.querySelector("[data-native-root]")).toBeNull();
    act(() => setDraft?.("second"));
    expect(container?.querySelector("[data-presenter-root]")?.textContent).toBe("second");
    expect(snapshot?.draftText).toBe("second");
    act(() => host.unload("composer"));
    expect(container?.querySelector("[data-native-root]")?.textContent).toBe("native");
  });

  it("dispatches setDraft, submit, and stop while rejecting invalid or disabled calls", async () => {
    const { host, controller } = setup();
    let presenterHost: PresentationHost | undefined;
    await host.activateGeneration({ pluginId: "actions", generation: "one", register(api) {
      api.registerPresenter({ id: "composer", target: "conversation.composer", render: ({ host: apiHost }) => {
        presenterHost = apiHost;
        return <div data-actions />;
      } });
    } });
    const onSetDraft = vi.fn();
    const onSubmit = vi.fn();
    const onStop = vi.fn();
    render(<ComposerPresentation {...baseProps(host, controller)} running draftText="send me"
      onSetDraft={onSetDraft} onSubmit={onSubmit} onStop={onStop} />);
    await act(async () => {
      await presenterHost?.invoke(COMPOSER_ACTIONS.setDraft, "updated");
      await presenterHost?.invoke(COMPOSER_ACTIONS.submit);
      await presenterHost?.invoke(COMPOSER_ACTIONS.stop);
    });
    expect(onSetDraft).toHaveBeenCalledWith("updated");
    expect(onSubmit).toHaveBeenCalledOnce();
    expect(onStop).toHaveBeenCalledOnce();
    await expect(presenterHost?.invoke(COMPOSER_ACTIONS.setDraft, { text: "bad" })).rejects.toThrow("must be a string");
    await expect(presenterHost?.invoke(COMPOSER_ACTIONS.submit, "bad")).rejects.toThrow("does not accept input");

    render(<ComposerPresentation {...baseProps(host, controller)} readOnly draftText="blocked" />);
    await expect(presenterHost?.invoke(COMPOSER_ACTIONS.setDraft, "nope")).rejects.toThrow("read-only");
  });

  it("builds an immutable representative snapshot without attachment bytes", () => {
    const image = { id: "image-1", media_type: "image/png", data: "raw-secret-bytes" };
    const file = { id: "file-1", filename: "notes.pdf", media_type: "application/pdf", data: "pdf-secret-bytes" };
    const snapshot = buildComposerSnapshot({
      draftText: "review", files: [file], images: [image],
      queuedMessages: [{ id: "q1", text: "later", files: [], images: [image] }],
      pendingMessages: [{ id: "p1", text: "next", files: [file], images: [] }],
      running: true, readOnly: false, variant: "dock", threadId: "thread-1",
      initialized: { protocol_version: "v", provider: "provider", model: "model", workspace_root: "/private",
        runtime_host: { kind: "local", instance_id: "runtime-1" }, permissions: { mode: "read_only" },
        advanced_settings: { context_window_tokens: 1000, max_steps: 100, max_context_tokens: 1000,
          temperature: 0, disable_auto_compact: false } },
      contextUsage: { turnID: "turn-1", used: 250, window: 1000, inputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0 },
      disabledReason: "wait",
      activeSubmissionMode: "steer", availableSubmissionModes: ["steer", "queue"],
    });
    expect(snapshot).toMatchObject({ contractVersion: 1, draftText: "review", threadId: "thread-1",
      attachments: [{ id: "file-1", name: "notes.pdf", mimeType: "application/pdf" },
        { id: "image-1", name: "image/png", mimeType: "image/png" }],
      queued: [{ id: "q1", attachmentCount: 1, status: "queued" }],
      pending: [{ id: "p1", attachmentCount: 1, status: "pending" }],
      model: { id: "model", providerId: "provider", contextWindowTokens: 1000 },
      runtime: { id: "runtime-1", status: "running" }, permission: { id: "read_only" },
      contextUsage: { usedTokens: 250, limitTokens: 1000, percent: 25 },
      disabledReason: "wait",
    });
    expect(JSON.stringify(snapshot)).not.toContain("secret-bytes");
    expect(Object.isFrozen(snapshot)).toBe(true);
    expect(Object.isFrozen(snapshot.attachments)).toBe(true);
    const minimal = buildComposerSnapshot({ draftText: "", files: [], images: [], queuedMessages: [], pendingMessages: [],
      running: false, readOnly: false, variant: "hero", activeSubmissionMode: "send", availableSubmissionModes: ["send"] });
    expect(minimal).not.toHaveProperty("threadId");
    expect(minimal).not.toHaveProperty("model");
    expect(minimal).not.toHaveProperty("runtime");
    expect(minimal).not.toHaveProperty("disabledReason");
  });
});

function setup(): { host: PluginHost; controller: WorkbenchController } {
  const host = new PluginHost({ react: React });
  const controller = new WorkbenchController(host);
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
  return { host, controller };
}

function baseProps(host: PluginHost, controller: WorkbenchController): ComponentProps<typeof ComposerPresentation> {
  return {
    enabled: true, fallback: <footer data-native-root>native</footer>, draftText: "draft", files: [], images: [],
    queuedMessages: [], pendingMessages: [], running: false, readOnly: false, sendDisabled: false, variant: "dock",
    activeSubmissionMode: "send", availableSubmissionModes: ["send"], attachmentInputRef: createRef(), attachmentsEnabled: true,
    onSetDraft: () => {}, onRemoveFile: () => {}, onRemoveImage: () => {}, onSubmit: () => {}, onStop: () => {}, host, controller,
  };
}

function render(node: React.ReactNode): void {
  act(() => root?.render(node));
}
