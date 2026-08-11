import assert from "node:assert/strict";
import test from "node:test";
import { Context } from "cordis";
import {
  ClientModuleSystem,
  clientKernelPlugin,
  type Plugin,
  type SlotHandle,
} from "./index.js";

test("materializes lazily and invalidates owner slot authorization", async () => {
  const ctx = new Context();
  const kernel = await ctx.plugin(clientKernelPlugin);
  const modules = new ClientModuleSystem(ctx);
  let materialized = 0;
  let surface: SlotHandle | undefined;

  modules.arrive("contributor", "1", async () => {
    materialized += 1;
    const plugin: Plugin = function contributor(client) {
      client.slots.contribute("surface", {
        id: "conversation",
        component: () => null,
      });
    };
    plugin.inject = ["slots"];
    return {
      default: plugin,
    };
  });
  modules.arrive("owner", "1", async () => {
    materialized += 1;
    const plugin: Plugin = function owner(client) {
      const registration = client.slots.contribute("root", {
        id: "layout",
        component: () => null,
        children: [{
          name: "surface",
          kind: "single",
          scope: "root",
        }],
      });
      surface = registration.children.get("surface");
    };
    plugin.inject = ["slots"];
    return {
      default: plugin,
    };
  });

  assert.equal(materialized, 0);
  const [firstOwner, secondOwner] = await Promise.all([
    modules.materialize("owner"),
    modules.materialize("owner"),
  ]);
  assert.equal(firstOwner, secondOwner);
  assert.equal(materialized, 1);
  await modules.activateAll(["contributor", "owner"]);
  assert.equal(materialized, 2);
  assert.equal(ctx.slots.entries(surface!).length, 1);

  await modules.invalidate("owner");
  assert.throws(() => ctx.slots.entries(surface!), /stale slot authorization/);

  await modules.dispose();
  await kernel.dispose();
  await ctx.fiber.dispose();
});
