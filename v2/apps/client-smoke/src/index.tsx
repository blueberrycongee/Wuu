import assert from "node:assert/strict";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import {
  ClientModuleSystem,
  Context,
  SlotOutlet,
  clientKernelPlugin,
} from "@wuu-v2/client-runtime";

const sessionId = "client-smoke";
const ctx = new Context();
const kernel = await ctx.plugin(clientKernelPlugin);
const modules = new ClientModuleSystem(ctx);

modules.arrive("layout", "1", () => import("@wuu-v2/plugin-layout/client"));
modules.arrive("conversation", "1", () => import("@wuu-v2/plugin-conversation/client"));
modules.arrive("composer", "1", () => import("@wuu-v2/plugin-composer/client"));

try {
  await modules.activateAll(["layout", "conversation", "composer"]);
  const disconnect = ctx.clientActions.connect(async (action, input) => ({ action, input }));
  ctx.clientProjections.apply(sessionId, "conversation", 1, {
    messages: [{ id: "message-1", role: "assistant", text: "Client smoke ready.", status: "complete" }],
    running: false,
  });

  const markup = renderToStaticMarkup(createElement(SlotOutlet, {
    client: ctx,
    slot: ctx.slots.root,
    ownerProps: { sessionId },
  }));
  assert.match(markup, /app-shell/);
  assert.match(markup, /Client smoke ready\./);
  assert.match(markup, /Message Wuu/);

  const response = await ctx.clientActions.execute("agent/prompt", { sessionId, text: "hello" });
  assert.deepEqual(response, {
    action: "agent/prompt",
    input: { sessionId, text: "hello" },
  });
  disconnect();
  console.log(JSON.stringify({ runtime: "wuu-v2-client", status: "ready" }));
} finally {
  await modules.dispose();
  await kernel.dispose();
  await ctx.fiber.dispose();
}
