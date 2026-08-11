import assert from "node:assert/strict";
import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import type { EventSource, SessionRecord } from "@wuu-v2/contracts";
import { createKernelContext } from "@wuu-v2/kernel";
import { jsonlSessionPlugin } from "./index.js";

const source: EventSource = { pluginId: "test", generation: "v1" };
const record = (value: number): SessionRecord<"test/value", { value: number }> => ({
  type: "test/value",
  data: { value },
});

test("serializes concurrent appends into a recoverable sequence", async () => {
  const directory = await mkdtemp(join(tmpdir(), "wuu-v2-session-"));
  const ctx = createKernelContext();
  const fiber = await ctx.plugin(jsonlSessionPlugin, { directory });
  try {
    const appended = await Promise.all(
      Array.from({ length: 16 }, (_, value) =>
        ctx.sessions.append("session-1", source, record(value)),
      ),
    );
    assert.deepEqual(appended.map((event) => event.seq), Array.from({ length: 16 }, (_, index) => index + 1));
    assert.deepEqual((await ctx.sessions.load("session-1")).map((event) => event.seq), appended.map((event) => event.seq));
  } finally {
    await fiber.dispose();
    await ctx.fiber.dispose();
    await rm(directory, { recursive: true, force: true });
  }
});

test("does not publish an append that cannot commit", async () => {
  const root = await mkdtemp(join(tmpdir(), "wuu-v2-session-fail-"));
  const blocked = join(root, "blocked");
  await writeFile(blocked, "not a directory", "utf8");
  const ctx = createKernelContext();
  const fiber = await ctx.plugin(jsonlSessionPlugin, { directory: blocked });
  const published: number[] = [];
  ctx.sessions.subscribe("session-1", (event) => published.push(event.seq));
  try {
    await assert.rejects(ctx.sessions.append("session-1", source, record(1)));
    assert.deepEqual(published, []);
  } finally {
    await fiber.dispose();
    await ctx.fiber.dispose();
    await rm(root, { recursive: true, force: true });
  }
});
