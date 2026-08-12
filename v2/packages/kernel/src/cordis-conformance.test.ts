import assert from "node:assert/strict";
import test from "node:test";
import { Context, type Plugin } from "cordis";

test("drains async disposers serially in reverse registration order", async () => {
  const events: string[] = [];
  const plugin: Plugin = (ctx) => {
    ctx.effect(() => async () => {
      events.push("first:start");
      await Promise.resolve();
      events.push("first:end");
    });
    ctx.effect(() => async () => {
      events.push("second:start");
      await Promise.resolve();
      events.push("second:end");
    });
  };
  const ctx = new Context();
  const fiber = await ctx.plugin(plugin);
  await fiber.dispose();
  assert.deepEqual(events, [
    "second:start",
    "second:end",
    "first:start",
    "first:end",
  ]);
  await ctx.fiber.dispose();
});

test("repeated disposal joins the same cleanup", async () => {
  let release!: () => void;
  let started!: () => void;
  const cleanupStarted = new Promise<void>((resolve) => {
    started = resolve;
  });
  const cleanupRelease = new Promise<void>((resolve) => {
    release = resolve;
  });
  const plugin: Plugin = (ctx) => {
    ctx.effect(() => async () => {
      started();
      await cleanupRelease;
    });
  };
  const ctx = new Context();
  const fiber = await ctx.plugin(plugin);
  const first = fiber.dispose();
  await cleanupStarted;
  let secondSettled = false;
  const second = Promise.resolve(fiber.dispose()).then(() => {
    secondSettled = true;
  });
  await Promise.resolve();
  assert.equal(secondSettled, false);
  release();
  await Promise.all([first, second]);
  assert.equal(secondSettled, true);
  await ctx.fiber.dispose();
});
