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
  const ownerModule = async () => {
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
    return { default: plugin };
  };

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
  modules.arrive("owner", "1", ownerModule);

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

  ctx.clientProjections.applyFrame({
    sessionId: "session-1",
    lastDurableSeq: 2,
    projections: [{ key: "conversation", seq: 2, value: { running: true } }],
  });
  ctx.clientProjections.applyFrame({
    sessionId: "session-1",
    lastDurableSeq: 1,
    projections: [],
  });
  assert.deepEqual(ctx.clientProjections.get("session-1", "conversation")?.value, { running: true });
  ctx.clientProjections.applyFrame({
    sessionId: "session-1",
    lastDurableSeq: 2,
    projections: [],
  });
  assert.equal(ctx.clientProjections.get("session-1", "conversation"), undefined);

  await modules.invalidate("owner");
  assert.throws(() => ctx.slots.entries(surface!), /stale slot authorization/);
  modules.arrive("owner", "2", ownerModule);
  await modules.activate("owner");
  assert.equal(ctx.slots.entries(surface!).length, 1);

  let candidateActive = 0;
  modules.arrive("candidate", "1", async () => {
    const plugin: Plugin = function candidate(client) {
      client.effect(() => {
        candidateActive += 1;
        return () => { candidateActive -= 1; };
      }, "candidate lifetime");
    };
    return { default: plugin };
  });
  modules.arrive("broken", "1", async () => {
    throw new Error("candidate module failed");
  });
  await assert.rejects(
    modules.activateAll(["candidate", "broken"]),
    /candidate module failed/,
  );
  assert.equal(candidateActive, 0);
  await modules.activate("candidate");
  assert.equal(candidateActive, 1);

  modules.arrive("missing-dependency", "1", async () => {
    const plugin: Plugin = () => {};
    plugin.inject = ["absentService"];
    return { default: plugin };
  });
  await assert.rejects(
    modules.activate("missing-dependency"),
    /missing services: missing-dependency -> absentService/,
  );

  modules.arrive("cycle-a", "1", async () => {
    const plugin: Plugin = () => {};
    plugin.inject = ["cycleB"];
    plugin.provide = "cycleA";
    return { default: plugin };
  });
  modules.arrive("cycle-b", "1", async () => {
    const plugin: Plugin = () => {};
    plugin.inject = ["cycleA"];
    plugin.provide = "cycleB";
    return { default: plugin };
  });
  await assert.rejects(
    modules.activateAll(["cycle-a", "cycle-b"]),
    /dependency cycle: cycle-a -> cycle-b -> cycle-a/,
  );

  await modules.dispose();
  assert.equal(candidateActive, 0);
  await kernel.dispose();
  await ctx.fiber.dispose();
});
