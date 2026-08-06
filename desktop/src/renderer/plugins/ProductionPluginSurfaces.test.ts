import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

import { PLUGIN_SURFACE_IDS } from "./PluginHost";

const PRODUCTION_SURFACE_OWNERS = Object.freeze({
  "app.shell": "../App.tsx",
  "app.sidebar": "../App.tsx",
  "app.main": "../App.tsx",
  "app.auxiliary": "../App.tsx",
  "app.status": "../App.tsx",
  "view.launch": "../App.tsx",
  "view.conversation": "../App.tsx",
  "view.workspace": "../App.tsx",
  "view.catalog": "../App.tsx",
  "view.settings": "../App.tsx",
  "conversation.timeline": "../TurnView.tsx",
  "conversation.message": "./ConversationMessageSurface.tsx",
  "conversation.composer": "../App.tsx",
} as const);

describe("production plugin surface mounts", () => {
  it("mounts every fixed legacy surface in a real product owner", () => {
    expect(Object.keys(PRODUCTION_SURFACE_OWNERS)).toEqual([...PLUGIN_SURFACE_IDS]);

    for (const [surfaceId, owner] of Object.entries(PRODUCTION_SURFACE_OWNERS)) {
      const source = readFileSync(new URL(owner, import.meta.url), "utf8");
      expect(source, `${surfaceId} production owner`).toContain(`id="${surfaceId}"`);
    }
  });
});
