import { defineConfig } from "vitest/config";

// Bridge tests run in Node; shell lifecycle tests opt into jsdom.
export default defineConfig({
  test: {
    include: ["test/**/*.test.ts"],
  },
});
