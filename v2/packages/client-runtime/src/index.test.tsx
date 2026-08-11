import assert from "node:assert/strict";
import test from "node:test";
import { Context } from "cordis";
import {
  ClientModuleSystem,
  type ScopedStoreSeat,
  clientKernelPlugin,
  type Plugin,
  type SlotHandle,
} from "./index.js";

test("materializes lazily and invalidates owner slot authorization", async () => {
  const ctx = new Context();
  const kernel = await ctx.plugin(clientKernelPlugin);
  const modules = new ClientModuleSystem(ctx);
  let materialized = 0;
  let contributorActive = 0;
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
      client.effect(() => {
        contributorActive += 1;
        return () => { contributorActive -= 1; };
      }, "contributor lifetime");
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
  assert.equal(contributorActive, 1);

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
  assert.equal(contributorActive, 0);
  modules.arrive("owner", "2", ownerModule);
  await modules.activate("owner");
  await modules.activate("contributor");
  assert.equal(ctx.slots.entries(surface!).length, 1);
  assert.equal(contributorActive, 1);

  let store: ScopedStoreSeat<string> | undefined;
  modules.arrive("store-owner", "1", async () => {
    const plugin: Plugin = function storeOwner(client) {
      store = client.scopedStores.define("test/draft", () => "");
    };
    plugin.inject = ["scopedStores"];
    return { default: plugin };
  });
  await modules.activate("store-owner");
  store!.set("session-a", "draft a");
  store!.set("session-b", "draft b");
  assert.equal(store!.get("session-a"), "draft a");
  assert.equal(store!.get("session-b"), "draft b");
  await modules.invalidate("store-owner");
  assert.throws(() => store!.get("session-a"), /stale scoped store seat/);

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

  let pendingContributorActive = 0;
  modules.arrive("pending-contributor", "1", async () => {
    const plugin: Plugin = function pendingContributor(client) {
      client.slots.contribute("future-seat", {
        id: "pending-view",
        component: () => null,
      });
      client.effect(() => {
        pendingContributorActive += 1;
        return () => { pendingContributorActive -= 1; };
      }, "pending contributor lifetime");
    };
    plugin.inject = ["slots"];
    return { default: plugin };
  });
  modules.arrive("failed-owner", "1", async () => {
    const plugin: Plugin = function failedOwner(client) {
      client.slots.contribute("future-parent", {
        id: "failed-owner",
        component: () => null,
        children: [
          { name: "future-seat", kind: "single", scope: "root" },
          { name: "surface", kind: "single", scope: "root" },
        ],
      });
    };
    plugin.inject = ["slots"];
    return { default: plugin };
  });
  await modules.activate("pending-contributor");
  await assert.rejects(modules.activate("failed-owner"), /duplicate slot declaration: surface/);
  assert.equal(pendingContributorActive, 1);
  let futureSeat: SlotHandle | undefined;
  modules.arrive("future-owner", "1", async () => {
    const plugin: Plugin = function futureOwner(client) {
      const registration = client.slots.contribute("future-parent", {
        id: "future-owner",
        component: () => null,
        children: [{ name: "future-seat", kind: "single", scope: "root" }],
      });
      futureSeat = registration.children.get("future-seat");
    };
    plugin.inject = ["slots"];
    return { default: plugin };
  });
  await modules.activate("future-owner");
  assert.equal(ctx.slots.entries(futureSeat!).length, 1);
  assert.equal(pendingContributorActive, 1);

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
