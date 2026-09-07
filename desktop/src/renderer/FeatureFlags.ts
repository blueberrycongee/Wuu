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
 * Collaboration is part of the default desktop product in development and
 * release builds. Keep a build-time opt-out for emergency rollback without
 * maintaining a separate release-only product surface.
 */
export const ENABLE_GROUP_CHAT =
  import.meta.env.VITE_ENABLE_GROUP_CHAT !== "false";

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

/**
 * Temporary local shortcut for opening first-run setup in a dedicated
 * 1120x840 window while iterating on the onboarding UI. Production builds
 * never expose it.
 */
export const ENABLE_ONBOARDING_PREVIEW = import.meta.env.DEV;
