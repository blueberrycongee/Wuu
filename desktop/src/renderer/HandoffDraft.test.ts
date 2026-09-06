import { describe, expect, it } from "vitest";

import { canSubmitHandoffDraft, parseHandoffArgs, reduceHandoffDraft } from "./HandoffDraft";

const catalog = {
  providers: [
    {
      id: "openai",
      label: "OpenAI",
      models: [{ id: "gpt-5.4", label: "GPT-5.4" }, { id: "gpt-5.5", label: "GPT-5.5" }],
      allowCustomModel: true,
    },
    {
      id: "anthropic",
      label: "Anthropic",
      models: [{ id: "claude-opus-4.1", label: "Claude Opus 4.1" }],
    },
  ],
};

describe("handoff draft", () => {
  it("keeps incomplete quoted input as a draft instead of an error", () => {
    expect(parseHandoffArgs(`model "open`)).toEqual({
      providerPrefix: "open",
      modelPrefix: "",
      intent: "",
      complete: false,
    });
  });

  it("parses provider/model and intent without asking a model", () => {
    const draft = reduceHandoffDraft("model openai/gpt-5.5 -- keep the verified fix", catalog, 3);
    expect(draft).toMatchObject({
      providerId: "openai",
      modelId: "gpt-5.5",
      intent: "keep the verified fix",
      status: "resolved",
      revision: 3,
    });
    expect(canSubmitHandoffDraft(draft)).toBe(true);
  });

  it("marks custom model ids as unverified instead of inventing a provider", () => {
    const draft = reduceHandoffDraft("model openai/my-local-gate/vision", catalog, 1);
    expect(draft.status).toBe("unverified");
    expect(draft.modelId).toBe("my-local-gate/vision");
    expect(canSubmitHandoffDraft(draft)).toBe(true);
  });

  it("does not submit while the user is still choosing a candidate", () => {
    const draft = reduceHandoffDraft("model open", catalog, 1);
    expect(draft.status).toBe("pending");
    expect(draft.candidates.map((item) => item.modelId)).toEqual(["gpt-5.4", "gpt-5.5"]);
    expect(canSubmitHandoffDraft(draft)).toBe(false);
  });
});
