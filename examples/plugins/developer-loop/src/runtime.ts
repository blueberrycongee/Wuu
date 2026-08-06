import { createInterface } from "node:readline";
import type { RuntimePlugin, RuntimeRequest, RuntimeResponse } from "@wuu/plugin-sdk";

const plugin: RuntimePlugin = {
  initialize() {
    return { hooks: [] };
  },
};

const lines = createInterface({ input: process.stdin, terminal: false });
lines.on("line", async (line) => {
  const request = JSON.parse(line) as RuntimeRequest;
  let response: RuntimeResponse;
  if (request.method === "initialize") {
    response = { id: request.id, result: await plugin.initialize(request.params) };
  } else if (request.method === "shutdown") {
    response = { id: request.id, result: null };
  } else {
    response = { id: request.id, error: { message: `unknown method ${request.method}` } };
  }
  process.stdout.write(`${JSON.stringify(response)}\n`);
});
