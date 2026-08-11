import { createKernelContext, kernelPlugin } from "@wuu-v2/kernel";

const ctx = createKernelContext();
const kernel = await ctx.plugin(kernelPlugin);

console.log(JSON.stringify({
  runtime: "wuu-v2",
  state: "kernel-ready",
  services: ["agents", "projections", "prompts", "providers", "tools"],
}));

const shutdown = async () => {
  await kernel.dispose();
  await ctx.fiber.dispose();
};

process.once("SIGINT", () => void shutdown().finally(() => process.exit(0)));
process.once("SIGTERM", () => void shutdown().finally(() => process.exit(0)));
