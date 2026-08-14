import { describe, expect, it } from "vitest";
import type { Context } from "cordis";

import type { PluginHost } from "./PluginHost";
import { DesktopService, PluginHostService, createDesktopCompositionRoot } from "./composition";

class DemoService extends DesktopService {
  constructor(ctx: Context) {
    super(ctx, "demo");
  }
}

describe("desktop composition root", () => {
  it("mounts and disposes a service on the kernel", async () => {
    const root = createDesktopCompositionRoot();
    const fiber = root.plugin(DemoService);
    await fiber;

    expect(root.get("demo")).toBeInstanceOf(DemoService);

    await fiber.dispose();
    expect(root.get("demo")).toBeUndefined();
  });

  it("exposes the PluginHost seam without changing the host instance", async () => {
    const host = {} as PluginHost;
    const root = createDesktopCompositionRoot();
    const fiber = root.plugin(PluginHostService, host);
    await fiber;

    const service = root.get("plugin-host");
    expect(service).toBeInstanceOf(PluginHostService);
    expect((service as PluginHostService).host).toBe(host);
  });
});
