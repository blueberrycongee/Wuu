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
import {
  arriveDefaultClientProfile,
  buildDefaultClientBootManifest,
} from "@wuu-v2/profile-default/client";
import type {} from "@wuu-v2/plugin-slash/client-api";

const sessionId = "client-smoke";
const ctx = new Context();
const kernel = await ctx.plugin(clientKernelPlugin);
const modules = new ClientModuleSystem(ctx);

const manifest = await buildDefaultClientBootManifest();
arriveDefaultClientProfile(modules, manifest);
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
    ...manifest.map(({ id }) => id),
    "smoke-command",
  ]);
  modules.auditReady();
  const disconnect = ctx.clientActions.connect(async (action, input) => {
    if (action === "side/resolve") return { sessionId: "side-client-smoke" };
    return { action, input };
  });
  ctx.clientProjections.apply(sessionId, "conversation", 1, {
    items: [{
      kind: "message",
      id: "message-1",
      role: "assistant",
      text: "Client smoke **ready**.",
      status: "complete",
    }, {
      kind: "tool",
      id: "tool:smoke-read",
      callId: "smoke-read",
      name: "read",
      input: { path: "package.json" },
      result: "package contents",
      status: "complete",
    }, {
      kind: "status",
      id: "status-1",
      text: "Run failed visibly.",
      status: "failed",
    }],
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
  assert.match(markup, /Client smoke <strong>ready<\/strong>\./);
  assert.match(markup, /Run failed visibly\./);
  assert.match(markup, /tool-activity/);
  assert.match(markup, /package contents/);
  assert.match(markup, /Message Wuu/);
  assert.deepEqual(ctx.slashCommands.entries().map(({ name }) => name), ["side", "smoke"]);

  await ctx.sidePanels.show(sessionId);
  ctx.clientProjections.apply("side-client-smoke", "conversation", 1, {
    items: [{
      kind: "message",
      id: "side-message",
      role: "assistant",
      text: "Side smoke ready.",
      status: "complete",
    }],
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
