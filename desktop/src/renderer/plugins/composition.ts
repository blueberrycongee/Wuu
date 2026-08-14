import { Context, Service } from "cordis";

import type { PluginHost } from "./PluginHost";

// DesktopService is the root type for the composable desktop kernel. PluginHost
// and Workbench will migrate onto this base in later steps; today it only
// establishes the Cordis composition substrate without changing behavior.
export abstract class DesktopService extends Service {
  constructor(ctx: Context, name: string) {
    super(ctx, name);
  }
}

// PluginHostService exposes the existing renderer PluginHost on the composition
// kernel. Consumers keep using the exported desktopPluginHost singleton; this
// service is the seam that later steps migrate onto Cordis scope/effect.
export class PluginHostService extends DesktopService {
  constructor(ctx: Context, readonly host: PluginHost) {
    super(ctx, "plugin-host");
  }
}

// createDesktopCompositionRoot creates the composition kernel for the renderer.
// The returned context owns every mounted service and its reversible effects.
export function createDesktopCompositionRoot(): Context {
  return new Context();
}
