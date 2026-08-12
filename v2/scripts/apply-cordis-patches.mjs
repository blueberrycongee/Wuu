import { readFile, writeFile } from "node:fs/promises";
import { join } from "node:path";

const root = new URL("..", import.meta.url).pathname;
const packagePath = join(root, "node_modules", "cordis", "package.json");
const entryPath = join(root, "node_modules", "cordis", "lib", "index.js");
const manifest = JSON.parse(await readFile(packagePath, "utf8"));
if (manifest.version !== "4.0.0-rc.8") {
  throw new Error(`unsupported Cordis patch target: ${manifest.version}`);
}

const patches = [{
  name: "join repeated effect disposal",
  before: `    const wrapper = defineProperty4(() => {
      if (!runner.epoch) return;
      runner.epoch = false;
      return task ? task.then(dispose) : dispose();
    }, symbols.effect, meta);
    const disposeAsync = /* @__PURE__ */ __name(() => {
      if (!runner.epoch) return;
      runner.epoch = false;
      return dispose();
    }, "disposeAsync");
    wrapper.then = async (onFulfilled, onRejected) => {
      return Promise.resolve(task).then(() => disposeAsync).then(onFulfilled, onRejected);
    };`,
  previous: `    let disposal;
    const disposeOnce = /* @__PURE__ */ __name(() => {
      if (disposal) return disposal;
      if (!runner.epoch) return disposal;
      runner.epoch = false;
      disposal = task ? task.then(dispose) : Promise.resolve(dispose());
      return disposal;
    }, "disposeOnce");
    const wrapper = defineProperty4(() => disposeOnce(), symbols.effect, meta);
    wrapper.then = async (onFulfilled, onRejected) => {
      return Promise.resolve(disposeOnce()).then(onFulfilled, onRejected);
    };`,
  after: `    let disposal;
    const disposeOnce = /* @__PURE__ */ __name(() => {
      if (disposal) return disposal;
      if (!runner.epoch) return disposal;
      runner.epoch = false;
      disposal = task ? task.then(dispose) : Promise.resolve().then(dispose);
      return disposal;
    }, "disposeOnce");
    const wrapper = defineProperty4(() => disposeOnce(), symbols.effect, meta);
    wrapper.then = async (onFulfilled, onRejected) => {
      return Promise.resolve(disposeOnce()).then(onFulfilled, onRejected);
    };`,
}, {
  name: "serialize fiber disposers in reverse registration order",
  before: `    await Promise.all(this._disposables.clear().map(async (dispose) => {
      try {
        await composeError(async (info) => {
          await Promise.resolve();
          info.error = new Error();
          await dispose();
        }, this._runner.getOuterStack);
      } catch (reason) {
        this.ctx.logger.error(reason);
      }
    }));`,
  previous: `    for (const dispose of this._disposables.clear()) {
      try {
        await composeError(async (info) => {
          await Promise.resolve();
          info.error = new Error();
          await dispose();
        }, this._runner.getOuterStack);
      } catch (reason) {
        this.ctx.logger.error(reason);
      }
    }`,
  after: `    await Promise.resolve();
    for (const dispose of this._disposables.clear()) {
      try {
        await composeError(async (info) => {
          await Promise.resolve();
          info.error = new Error();
          await dispose();
        }, this._runner.getOuterStack);
      } catch (reason) {
        this.ctx.logger.error(reason);
      }
    }`,
}];

let source = await readFile(entryPath, "utf8");
for (const patch of patches) {
  if (source.includes(patch.after)) continue;
  const before = source.includes(patch.before)
    ? patch.before
    : patch.previous && source.includes(patch.previous)
    ? patch.previous
    : undefined;
  if (!before) {
    throw new Error(`Cordis patch context changed: ${patch.name}`);
  }
  source = source.replace(before, patch.after);
}
await writeFile(entryPath, source, "utf8");
