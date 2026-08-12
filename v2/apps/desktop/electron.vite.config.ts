import { resolve } from "node:path";
import { fileURLToPath } from "node:url";
import react from "@vitejs/plugin-react";
import { defineConfig } from "electron-vite";

const directory = fileURLToPath(new URL(".", import.meta.url));

export default defineConfig({
  main: {
    build: {
      externalizeDeps: false,
      rollupOptions: {
        input: {
          index: resolve(directory, "src/main/index.ts"),
          worker: resolve(directory, "src/harness/worker.ts"),
        },
      },
    },
  },
  preload: {
    build: {
      externalizeDeps: false,
      isolatedEntries: true,
      rollupOptions: {
        input: { index: resolve(directory, "src/preload/index.ts") },
        output: { format: "cjs" },
      },
    },
  },
  renderer: {
    root: ".",
    plugins: [react()],
    resolve: { dedupe: ["react", "react-dom", "cordis"] },
    build: {
      rollupOptions: {
        input: { index: resolve(directory, "index.html") },
      },
    },
  },
});
