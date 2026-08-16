import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// The app imports TS sources from sibling packages (@wuu/remote-core,
// @wuu/protocol) via `file:` links. Allow the whole repo root on the dev
// server so linked sources outside this package resolve without fs errors.
export default defineConfig({
  plugins: [react()],
  server: {
    host: true,
    // The desktop app's electron-vite dev server owns 5173 on the same
    // machine; pin the phone companion to its own port so plain-browser
    // loads never collide with the Electron renderer.
    port: 5174,
    strictPort: true,
    fs: { allow: ["../.."] },
  },
  build: {
    target: "es2022",
    sourcemap: false,
  },
});
