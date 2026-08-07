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
 * Collaboration is a standard desktop capability from v0.14.0 onward. Keep a
 * named constant while the renderer still shares gated component boundaries,
 * but do not let build-time environment drift hide the released product.
 */
export const ENABLE_GROUP_CHAT = true;

/**
 * Voice input and its optional BYOK text polish are hidden while the native
 * recognition flow and polish experience are still being stabilized.
 *
 * Use `VITE_ENABLE_VOICE_INPUT=true npm run dev` for internal testing.
 */
export const ENABLE_VOICE_INPUT =
  import.meta.env.VITE_ENABLE_VOICE_INPUT === "true";

/**
 * The Skills management assistant is an early surface-assistant experiment.
 * Keep it out of release builds until its
 * ephemeral-session and correction UX have been validated through dogfooding.
 *
 * Use `VITE_ENABLE_MANAGEMENT_ASSISTANT=true npm run dev` for internal testing.
 */
export const ENABLE_MANAGEMENT_ASSISTANT =
  import.meta.env.VITE_ENABLE_MANAGEMENT_ASSISTANT === "true";
