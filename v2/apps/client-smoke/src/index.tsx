import assert from "node:assert/strict";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import {
  ClientModuleSystem,
  Context,
  SlotOutlet,
  clientKernelPlugin,
  type Plugin,
} from "@wuu-v2/client-runtime";
import type {} from "@wuu-v2/plugin-slash/client-api";

const sessionId = "client-smoke";
const ctx = new Context();
const kernel = await ctx.plugin(clientKernelPlugin);
const modules = new ClientModuleSystem(ctx);

modules.arrive("layout", "1", () => import("@wuu-v2/plugin-layout/client"));
modules.arrive("conversation", "1", () => import("@wuu-v2/plugin-conversation/client"));
modules.arrive("composer", "1", () => import("@wuu-v2/plugin-composer/client"));
modules.arrive("slash", "1", () => import("@wuu-v2/plugin-slash/client"));
modules.arrive("model-session", "1", () => import("@wuu-v2/plugin-model-session/client"));
modules.arrive("permission-session", "1", () => import("@wuu-v2/plugin-permission-session/client"));
modules.arrive("side", "1", () => import("@wuu-v2/plugin-side/client"));
modules.arrive("smoke-command", "1", async () => {
  const plugin: Plugin = function smokeCommand(client) {
    client.slashCommands.register({
      id: "smoke",
      name: "smoke",
      title: "Smoke command",
      execute: ({ args }) => ({ type: "replace", draft: `smoke ${args}`.trim() }),
    });
  };
  plugin.inject = ["slashCommands"];
  return { default: plugin };
});

try {
  await modules.activateAll([
    "layout",
    "conversation",
    "composer",
    "slash",
    "model-session",
    "permission-session",
    "side",
    "smoke-command",
  ]);
  const disconnect = ctx.clientActions.connect(async (action, input) => {
    if (action === "side/resolve") return { sessionId: "side-client-smoke" };
    return { action, input };
  });
  ctx.clientProjections.apply(sessionId, "conversation", 1, {
    messages: [{ id: "message-1", role: "assistant", text: "Client smoke ready.", status: "complete" }],
    running: false,
  });
  ctx.clientProjections.apply(sessionId, "model", 1, {
    selected: "smoke-model",
    options: [{ id: "smoke-model", label: "Smoke model" }],
  });
  ctx.clientProjections.apply(sessionId, "permission", 1, {
    selected: "workspace-write",
    options: [{ id: "workspace-write", label: "Workspace write" }],
  });

  const markup = renderToStaticMarkup(createElement(SlotOutlet, {
    client: ctx,
    slot: ctx.slots.root,
    ownerProps: { sessionId },
  }));
  assert.match(markup, /app-shell/);
  assert.match(markup, /Client smoke ready\./);
  assert.match(markup, /Message Wuu/);
  assert.deepEqual(ctx.slashCommands.entries().map(({ name }) => name), ["side", "smoke"]);

  await ctx.sidePanels.show(sessionId);
  ctx.clientProjections.apply("side-client-smoke", "conversation", 1, {
    messages: [{ id: "side-message", role: "assistant", text: "Side smoke ready.", status: "complete" }],
    running: false,
  });
  ctx.clientProjections.apply("side-client-smoke", "model", 1, {
    selected: "smoke-model",
    options: [{ id: "smoke-model", label: "Smoke model" }],
  });
  ctx.clientProjections.apply("side-client-smoke", "permission", 1, {
    selected: "read-only",
    options: [{ id: "read-only", label: "Read only" }],
  });
  const sideMarkup = renderToStaticMarkup(createElement(SlotOutlet, {
    client: ctx,
    slot: ctx.slots.root,
    ownerProps: { sessionId },
  }));
  assert.match(sideMarkup, /Side smoke ready\./);
  assert.match(sideMarkup, /wuu-composer-surface/);
  assert.match(sideMarkup, /Message Side/);
  assert.match(sideMarkup, /aria-label="Model"/);
  assert.match(sideMarkup, /aria-label="Permission"/);

  const response = await ctx.clientActions.execute("agent/prompt", { sessionId, text: "hello" });
  assert.deepEqual(response, {
    action: "agent/prompt",
    input: { sessionId, text: "hello" },
  });
  await modules.invalidate("smoke-command");
  assert.deepEqual(ctx.slashCommands.entries().map(({ name }) => name), ["side"]);
  disconnect();
  console.log(JSON.stringify({ runtime: "wuu-v2-client", status: "ready" }));
} finally {
  await modules.dispose();
  await kernel.dispose();
  await ctx.fiber.dispose();
}
