import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

import { PRESENTATION_TARGETS } from "../../shared/workbench";

const PRODUCTION_PRESENTER_OWNERS = Object.freeze({
  "conversation.item": ["./ConversationItemPresentation.tsx", 'target="conversation.item"'],
  "conversation.process": ["./ConversationProcessPresentation.tsx", 'target="conversation.process"'],
  "conversation.tool-activity": ["./ToolActivityPresenter.tsx", 'target="conversation.tool-activity"'],
  "conversation.composer": ["./ComposerPresentation.tsx", 'target="conversation.composer"'],
  "header.conversation": ["./HeaderPresentation.tsx", "`header.${snapshot.scope}`"],
  "header.workspace": ["./HeaderPresentation.tsx", "`header.${snapshot.scope}`"],
  "navigation.primary": ["./NavigationPresentation.tsx", 'target="navigation.primary"'],
  "app.status": ["./Workbench.tsx", 'target="app.status"'],
  "content.preview": ["./FilePreviewPresentation.tsx", 'target="content.preview"'],
  settings: ["./SettingsPresentation.tsx", 'target="settings"'],
} as const);

describe("production semantic presenter mounts", () => {
  it("gives every built-in target a production adapter", () => {
    expect(Object.keys(PRODUCTION_PRESENTER_OWNERS)).toEqual([...PRESENTATION_TARGETS]);

    for (const [target, [owner, needle]] of Object.entries(PRODUCTION_PRESENTER_OWNERS)) {
      const source = readFileSync(new URL(owner, import.meta.url), "utf8");
      expect(source, `${target} production adapter`).toContain(needle);
    }
  });
});
