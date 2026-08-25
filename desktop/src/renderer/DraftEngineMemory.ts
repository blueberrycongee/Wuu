import type { EngineInfo, EngineListResult } from "../shared/protocol";

// Last engine the user picked in the composer for a brand-new conversation.
// Engine binding is a thread-creation decision, so the draft selection is
// per-thread state that resets constantly. Without a persisted memory a user
// who works in Codex/Claude Code has to re-pick the agent on every new tab and
// every relaunch, while the built-in wuu provider/model choice survives
// because it lives in the server config.
const DRAFT_ENGINE_MEMORY_KEY = "wuu.desktop.lastDraftEngine";

// External engines that can be bound at thread creation. Mirrors the composer
// picker so a value written by a newer build (or hand-edited storage) cannot
// seed an engine the picker itself would not offer.
const EXTERNAL_ENGINE_IDS = new Set(["codex", "claude"]);

export type DraftEngineMemory = {
  engine: string;
  // Empty means "use the engine default". The composer already resolves an
  // empty model/effort through the engine catalog default, so a model that
  // disappeared degrades to the default instead of pinning a dead id.
  model: string;
  effort: string;
};

export function readDraftEngineMemory(): DraftEngineMemory | undefined {
  try {
    const raw = window.localStorage.getItem(DRAFT_ENGINE_MEMORY_KEY);
    if (!raw) return undefined;
    const parsed: unknown = JSON.parse(raw);
    if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
      return undefined;
    }
    const record = parsed as Partial<Record<keyof DraftEngineMemory, unknown>>;
    const engine = typeof record.engine === "string" ? record.engine.trim() : "";
    if (!engine) return undefined;
    return {
      engine,
      model: typeof record.model === "string" ? record.model : "",
      effort: typeof record.effort === "string" ? record.effort : "",
    };
  } catch {
    // Corrupted or blocked storage just means "no remembered engine".
    return undefined;
  }
}

export function writeDraftEngineMemory(memory: DraftEngineMemory): void {
  const engine = memory.engine.trim();
  if (!engine) {
    clearDraftEngineMemory();
    return;
  }
  try {
    window.localStorage.setItem(
      DRAFT_ENGINE_MEMORY_KEY,
      JSON.stringify({ engine, model: memory.model, effort: memory.effort }),
    );
  } catch {
    // A denied/quota-limited write should not break engine selection; the
    // in-memory draft still applies for the current window.
  }
}

export function clearDraftEngineMemory(): void {
  try {
    window.localStorage.removeItem(DRAFT_ENGINE_MEMORY_KEY);
  } catch {
    // Nothing to recover: the next read falls back to the settings default.
  }
}

/**
 * Remembered selection to seed a new conversation with, validated against the
 * live engine inventory. Returns undefined when nothing is remembered or the
 * remembered engine is not currently offered, in which case the caller keeps
 * the normal settings-default fallback.
 *
 * A stale entry is deliberately left in storage: an external CLI that is
 * missing right now (PATH not ready, reinstall in flight) should recover the
 * preference once it is detected again rather than lose it.
 */
export function resolveDraftEngineMemory(
  inventory: EngineListResult | undefined,
): DraftEngineMemory | undefined {
  const memory = readDraftEngineMemory();
  if (!memory) return undefined;
  // An explicit wuu pick overrides an external default engine and needs no
  // detection, so it applies before the inventory arrives.
  if (memory.engine === "wuu") {
    return { engine: "wuu", model: "", effort: "" };
  }
  if (!EXTERNAL_ENGINE_IDS.has(memory.engine)) return undefined;
  if (!inventory) return undefined;
  const engine = inventory.engines.find((item) => item.id === memory.engine);
  if (!engine?.enabled || !engine.binary_ok) return undefined;
  return {
    engine: memory.engine,
    ...runtimeWithinCatalog(engine, memory),
  };
}

// Drop a model/effort the engine no longer reports so the caller's default
// fallback takes over. Efforts are checked against the resolved model because
// the supported set is per-model.
function runtimeWithinCatalog(
  engine: EngineInfo,
  memory: DraftEngineMemory,
): { model: string; effort: string } {
  const models = engine.models ?? [];
  // An engine that reports no catalog (models_error) keeps the remembered
  // values: they were valid when picked and the engine still accepts them.
  if (models.length === 0) {
    return { model: memory.model, effort: memory.effort };
  }
  const model = models.find((item) => item.id === memory.model);
  if (!model) return { model: "", effort: "" };
  const effort = (model.supported_efforts ?? []).includes(memory.effort)
    ? memory.effort
    : "";
  return { model: memory.model, effort };
}
