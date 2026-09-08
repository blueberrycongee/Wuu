const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { app, BrowserWindow } = require("electron");
const root = path.resolve(__dirname, "..");
app.setPath("userData", fs.mkdtempSync(path.join(root, "out", "pull-session-e2e-")));
process.env.WUU_RESIZE_E2E_CWD = path.resolve(root, "..");
process.env.WUU_PULL_SESSION_E2E = "1";

app.whenReady().then(async () => {
  const timeout = setTimeout(() => app.exit(1), 30000);
  const win = new BrowserWindow({
    width: 390, height: 820, show: true,
    webPreferences: {
      preload: path.join(__dirname, "resize-e2e-preload.cjs"),
      contextIsolation: true, sandbox: false, backgroundThrottling: false,
    },
  });
  const evaluate = (fn) => win.webContents.executeJavaScript(`(${fn})()`);
  const waitFor = async (fn) => {
    for (let i = 0; i < 150; i++) {
      if (await evaluate(fn)) return;
      await new Promise((resolve) => setTimeout(resolve, 20));
    }
    throw new Error(`Timed out: ${fn}`);
  };
  try {
    await win.loadURL("about:blank");
    win.webContents.debugger.attach("1.3");
    const send = (method, params) => win.webContents.debugger.sendCommand(method, params);
    await send("Emulation.setTouchEmulationEnabled", { enabled: true, maxTouchPoints: 5 });
    await win.loadFile(path.join(root, "out/renderer/index.html"));
    await waitFor(() => !!document.querySelector(".turn"));
    await evaluate(() => { document.documentElement.dataset.hostKind = "web"; });
    win.setContentSize(800, 820);
    await waitFor(() => !document.querySelector(".app-shell").classList.contains("compact-navigation"));
    win.setContentSize(390, 820);
    await waitFor(() => document.querySelector(".app-shell").classList.contains("compact-navigation"));
    assert.equal(await evaluate(() => matchMedia("(pointer: coarse)").matches), true);
    await waitFor(() => !!document.querySelector(".conversation-status-cluster"));
    await evaluate(() => {
      const viewport = document.querySelector(".scroll-region");
      viewport.scrollTop = viewport.scrollHeight;
    });
    await waitFor(() => {
      const node = document.querySelector(".scroll-region");
      return Math.abs(node.scrollHeight - node.clientHeight - node.scrollTop) <= 2;
    });
    const before = await evaluate(() => {
      const viewport = document.querySelector(".scroll-region");
      const rect = viewport.getBoundingClientRect();
      const composer = document.querySelector('[data-main-conversation-composer="dock"]');
      const dock = composer.getBoundingClientRect();
      return { x: rect.left + 10, y: dock.top - 50,
        contentTop: document.querySelector(".scroll-region-content").getBoundingClientRect().top,
        dockTop: dock.top, scrollTop: viewport.scrollTop };
    });
    const touch = async (type, y) => {
      await send("Input.dispatchTouchEvent", {
        type, touchPoints: type === "touchEnd" ? [] : [{ x: before.x, y, id: 1 }],
      });
      await evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    };
    await touch("touchStart", before.y);
    await touch("touchMove", before.y - 35);
    await waitFor(() => !!document.querySelector('[data-phase="pulling"]'));
    const first = await evaluate(() => ({
      top: document.querySelector(".scroll-region-content").getBoundingClientRect().top,
      drop: document.querySelector(".pull-to-new-session-drop").getBoundingClientRect().toJSON(),
      todoVisible: document.querySelector(".conversation-status-cluster").checkVisibility({ checkVisibilityCSS: true }),
    }));
    assert.ok(first.top < before.contentTop, "message flow should move upward");
    assert.equal(first.todoVisible, false, "TODO must not overlap the pull animation");
    await touch("touchMove", before.y - 110);
    const stretched = await evaluate(() => ({
      drop: document.querySelector(".pull-to-new-session-drop").getBoundingClientRect().toJSON(),
      dockTop: document.querySelector('[data-main-conversation-composer="dock"]').getBoundingClientRect().top,
      scrollTop: document.querySelector(".scroll-region").scrollTop,
    }));
    assert.ok(stretched.drop.height > first.drop.height, "upper tip should stretch with the finger");
    assert.ok(Math.abs(stretched.drop.bottom - first.drop.bottom) < 1, "lower tip must stay anchored");
    assert.equal(stretched.dockTop, before.dockTop, "composer must stay fixed");
    assert.equal(stretched.scrollTop, before.scrollTop, "pull must not move the native scroll position");
    assert.ok(stretched.drop.bottom < stretched.dockTop, "drop must not overlap composer");
    await touch("touchMove", before.y - 25);
    await touch("touchEnd", before.y - 25);
    await waitFor(() => !document.querySelector(".pull-to-new-session"));
    assert.ok(await evaluate(() => !!document.querySelector(".turn")), "retreat should keep the session");
    const restored = await evaluate(() => document.querySelector(".scroll-region-content").getBoundingClientRect().top);
    assert.ok(Math.abs(restored - before.contentTop) < 1, "cancel should restore message position");
    assert.equal(await evaluate(() => document.querySelector(".conversation-status-cluster").checkVisibility({ checkVisibilityCSS: true })), true, "cancel should restore TODO");
    await touch("touchStart", before.y);
    await touch("touchMove", before.y - 110);
    await touch("touchEnd", before.y - 110);
    await waitFor(() => !!document.querySelector(".empty-scroll-region"));
    assert.equal(await evaluate(() => document.querySelector(".conversation-pane").hasAttribute("data-session-pull")), false);
    console.log("PASS: native touch moves messages, stretches an anchored oval, keeps composer fixed, cancels, and opens a draft after release.");
    clearTimeout(timeout);
    win.destroy();
    app.quit();
  } catch (error) {
    console.error(error);
    clearTimeout(timeout);
    app.exit(1);
  }
});
