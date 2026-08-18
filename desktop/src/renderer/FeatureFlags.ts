/// <reference types="vite/client" />

/**
 * Remote control is still experimental. Keep its desktop settings entry
 * opt-in while the mobile pairing experience is under development.
 *
 * Vite replaces this value at build time. Use
 * `VITE_ENABLE_REMOTE_CONTROL=true npm run dev` for internal testing.
 */
export const ENABLE_REMOTE_CONTROL =
  import.meta.env.VITE_ENABLE_REMOTE_CONTROL === "true";

/**
 * Collaboration remains experimental while its product model is aligned with
 * the plugin architecture. Development builds enable it by default for
 * dogfooding; release builds keep it out until that ownership boundary is
 * settled. A release-style build can still opt in explicitly with
 * `VITE_ENABLE_GROUP_CHAT=true npm run build`.
 */
export const ENABLE_GROUP_CHAT =
  import.meta.env.DEV || import.meta.env.VITE_ENABLE_GROUP_CHAT === "true";

/**
 * Voice input and its optional BYOK text polish are hidden while the native
 * recognition flow and polish experience are still being stabilized.
 *
 * Use `VITE_ENABLE_VOICE_INPUT=true npm run dev` for internal testing.
 */
export const ENABLE_VOICE_INPUT =
  import.meta.env.DEV && import.meta.env.VITE_ENABLE_VOICE_INPUT === "true";

/**
 * The embedded browser remains an internal development capability. Production
 * builds do not expose its workspace surface even if the build environment
 * happens to contain the opt-in variable.
 */
export const ENABLE_EMBEDDED_BROWSER =
  import.meta.env.DEV && import.meta.env.VITE_ENABLE_BROWSER === "true";

/**
 * The Skills management assistant is an early surface-assistant experiment.
 * Keep it out of release builds until its
 * ephemeral-session and correction UX have been validated through dogfooding.
 *
 * Use `VITE_ENABLE_MANAGEMENT_ASSISTANT=true npm run dev` for internal testing.
 */
export const ENABLE_MANAGEMENT_ASSISTANT =
  import.meta.env.VITE_ENABLE_MANAGEMENT_ASSISTANT === "true";
