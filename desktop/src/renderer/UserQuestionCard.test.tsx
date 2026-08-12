import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { UserQuestionRequest, WuuDesktopApi } from "../shared/protocol";
import { I18nProvider, setActiveLocale } from "./i18n";
import { UserQuestionCard } from "./UserQuestionCard";

let root: Root | undefined;

afterEach(() => {
  act(() => root?.unmount());
  root = undefined;
  document.body.innerHTML = "";
  delete (window as unknown as { wuu?: unknown }).wuu;
  setActiveLocale("zh-CN");
});

describe("UserQuestionCard", () => {
  it("collects an option and custom answer before continuing", async () => {
    const onAnswer = vi.fn(async () => undefined);
    const request: UserQuestionRequest = {
      request_id: "request-1",
      plugin_id: "ask-user",
      execution_id: "execution-1",
      thread_id: "thread-1",
      turn_id: "turn-1",
      created_at: new Date().toISOString(),
      questions: [{
        id: "path",
        question: "Which path?",
        options: [{ label: "Safe", description: "Take fewer risks" }],
        allow_custom: true,
      }],
    };
    window.wuu = {
      initialLanguagePreference: "en-US",
      initialSystemLocale: "en-US",
    } as unknown as WuuDesktopApi;
    const container = document.createElement("div");
    document.body.append(container);
    root = createRoot(container);
    await act(async () => {
      root?.render(
        <I18nProvider>
          <UserQuestionCard request={request} onAnswer={onAnswer} onCancel={async () => undefined} />
        </I18nProvider>,
      );
    });

    const option = Array.from(container.querySelectorAll("button"))
      .find((button) => button.textContent?.includes("Safe"));
    const custom = container.querySelector("input");
    expect(option).toBeTruthy();
    expect(custom).toBeTruthy();
    await act(async () => {
      option?.click();
      if (custom) {
        const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")?.set;
        setter?.call(custom, "Keep the rollback simple");
        custom.dispatchEvent(new Event("input", { bubbles: true }));
      }
    });
    const submit = Array.from(container.querySelectorAll("button"))
      .find((button) => button.textContent === "Continue");
    await act(async () => { submit?.click(); });

    expect(onAnswer).toHaveBeenCalledWith({
      answers: [{ id: "path", selected: ["Safe"], custom: "Keep the rollback simple" }],
    });
  });
});
