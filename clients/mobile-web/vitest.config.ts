import { defineConfig } from "vitest/config";

// Pure-logic tests only (src/lib), mirrored from clients/mobile/test for the
// shared data layer (store/chatModel/threads/handoff/format).
export default defineConfig({
  test: {
    include: ["test/**/*.test.ts"],
  },
});
