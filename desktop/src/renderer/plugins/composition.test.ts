import { describe, expect, it } from "vitest";
import type { Context } from "cordis";

import { DesktopService, createDesktopCompositionRoot } from "./composition";

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
});
