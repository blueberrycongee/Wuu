import type {
  ClientModuleFactory,
  ClientModuleSystem,
} from "@wuu-v2/client-runtime";

export interface ClientBootEntry {
  id: string;
  url: string;
  revision: string;
  inject: string[];
  immediate?: boolean;
}

interface ProfileClientEntry {
  id: string;
  url: string;
  revision: string;
  load: ClientModuleFactory;
}

const entries: ProfileClientEntry[] = [
  { id: "theme-default", url: "@wuu-v2/plugin-theme-default/client", revision: "1", load: () => import("@wuu-v2/plugin-theme-default/client") },
  { id: "layout", url: "@wuu-v2/plugin-layout/client", revision: "1", load: () => import("@wuu-v2/plugin-layout/client") },
  { id: "conversation", url: "@wuu-v2/plugin-conversation/client", revision: "1", load: () => import("@wuu-v2/plugin-conversation/client") },
  { id: "composer", url: "@wuu-v2/plugin-composer/client", revision: "1", load: () => import("@wuu-v2/plugin-composer/client") },
  { id: "slash", url: "@wuu-v2/plugin-slash/client", revision: "1", load: () => import("@wuu-v2/plugin-slash/client") },
  { id: "model-session", url: "@wuu-v2/plugin-model-session/client", revision: "1", load: () => import("@wuu-v2/plugin-model-session/client") },
  { id: "permission-session", url: "@wuu-v2/plugin-permission-session/client", revision: "1", load: () => import("@wuu-v2/plugin-permission-session/client") },
  { id: "side", url: "@wuu-v2/plugin-side/client", revision: "1", load: () => import("@wuu-v2/plugin-side/client") },
];

function injectKeys(inject: Awaited<ReturnType<ClientModuleFactory>>["default"]["inject"]): string[] {
  if (!inject) return [];
  return Array.isArray(inject) ? inject.map(String) : Object.keys(inject);
}

export async function buildDefaultClientBootManifest(): Promise<ClientBootEntry[]> {
  return Promise.all(entries.map(async (entry) => {
    const module = await entry.load();
    return {
      id: entry.id,
      url: entry.url,
      revision: entry.revision,
      inject: injectKeys(module.default.inject),
    };
  }));
}

export function arriveDefaultClientProfile(
  modules: ClientModuleSystem,
  manifest: readonly ClientBootEntry[],
): void {
  const selected = new Map(entries.map((entry) => [entry.id, entry]));
  for (const boot of manifest) {
    const entry = selected.get(boot.id);
    if (!entry || entry.url !== boot.url || entry.revision !== boot.revision) {
      throw new Error(`client package is not available: ${boot.id}@${boot.revision}`);
    }
    modules.arrive(entry.id, entry.revision, async () => {
      const module = await entry.load();
      const actual = injectKeys(module.default.inject);
      if (actual.join("\0") !== boot.inject.join("\0")) {
        throw new Error(`client dependency manifest changed: ${entry.id}`);
      }
      return module;
    });
  }
}
