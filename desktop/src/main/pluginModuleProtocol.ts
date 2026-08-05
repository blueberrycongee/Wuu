import { protocol } from "electron";
import { createHash } from "node:crypto";

import type {
  PluginDesktopModuleLoadResult,
  PluginDesktopModuleReadResult,
} from "../shared/protocol";

const PLUGIN_MODULE_SCHEME = "wuu-plugin";
const MAX_CACHED_MODULES = 32;
const modules = new Map<string, string>();

export function registerPluginModuleScheme(): void {
  protocol.registerSchemesAsPrivileged([{
    scheme: PLUGIN_MODULE_SCHEME,
    privileges: {
      standard: true,
      secure: true,
      supportFetchAPI: true,
      corsEnabled: true,
      codeCache: true,
    },
  }]);
}

export function registerPluginModuleProtocol(): void {
  protocol.handle(PLUGIN_MODULE_SCHEME, (request) => {
    const digest = digestFromPluginModuleURL(request.url);
    const source = digest ? modules.get(digest) : undefined;
    if (!source) {
      return new Response("Not found", { status: 404 });
    }
    return new Response(source, {
      headers: {
        "Content-Type": "text/javascript; charset=utf-8",
        "Cache-Control": "no-store",
        "Cross-Origin-Resource-Policy": "same-origin",
      },
    });
  });
}

export function cachePluginDesktopModule(
  module: PluginDesktopModuleReadResult,
): PluginDesktopModuleLoadResult {
  const digest = createHash("sha256").update(module.source, "utf8").digest("hex");
  if (digest !== module.digest || !/^[a-f0-9]{64}$/.test(digest)) {
    throw new Error("Desktop plugin module digest mismatch");
  }
  modules.delete(digest);
  modules.set(digest, module.source);
  while (modules.size > MAX_CACHED_MODULES) {
    const oldest = modules.keys().next().value;
    if (typeof oldest !== "string") {
      break;
    }
    modules.delete(oldest);
  }
  return {
    id: module.id,
    fingerprint: module.fingerprint,
    digest,
    url: `${PLUGIN_MODULE_SCHEME}://module/${digest}.js`,
  };
}

export function digestFromPluginModuleURL(value: string): string | undefined {
  try {
    const url = new URL(value);
    if (url.protocol !== `${PLUGIN_MODULE_SCHEME}:` || url.hostname !== "module") {
      return undefined;
    }
    const match = /^\/([a-f0-9]{64})\.js$/.exec(url.pathname);
    return match?.[1];
  } catch {
    return undefined;
  }
}
