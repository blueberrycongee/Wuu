import { Context, Service } from "cordis";

// DesktopService is the root type for the composable desktop kernel. PluginHost
// and Workbench will migrate onto this base in later steps; today it only
// establishes the Cordis composition substrate without changing behavior.
export abstract class DesktopService extends Service {
  constructor(ctx: Context, name: string) {
    super(ctx, name);
  }
}

// createDesktopCompositionRoot creates the composition kernel for the renderer.
// The returned context owns every mounted service and its reversible effects.
export function createDesktopCompositionRoot(): Context {
  return new Context();
}
