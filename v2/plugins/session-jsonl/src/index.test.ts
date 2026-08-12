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

test("does not start when the durable directory cannot be owned", async () => {
  const root = await mkdtemp(join(tmpdir(), "wuu-v2-session-fail-"));
  const blocked = join(root, "blocked");
  await writeFile(blocked, "not a directory", "utf8");
  const ctx = createKernelContext();
  try {
    await assert.rejects(async () => {
      await ctx.plugin(jsonlSessionPlugin, { directory: blocked });
    });
    assert.equal(ctx.get("sessions"), undefined);
  } finally {
    await ctx.fiber.dispose();
    await rm(root, { recursive: true, force: true });
  }
});
