const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { app, BrowserWindow } = require("electron");
const root = path.resolve(__dirname, "..");
app.setPath("userData", fs.mkdtempSync(path.join(root, "out", "sidebar-touch-e2e-")));
process.env.WUU_RESIZE_E2E_CWD = path.resolve(root, "..");

app.whenReady().then(async () => {
  const timeout = setTimeout(() => { console.error("Touch E2E timed out"); app.exit(1); }, 30000);
  const win = new BrowserWindow({
    width: 390, height: 820, show: true,
    webPreferences: {
      preload: path.join(__dirname, "resize-e2e-preload.cjs"),
      contextIsolation: true, sandbox: false, backgroundThrottling: false,
    },
  });
  const evaluate = (fn) => win.webContents.executeJavaScript(`(${fn})()`);
  const waitFor = async (fn) => {
    for (let i = 0; i < 100; i++) {
      if (await evaluate(fn)) return;
      await new Promise((resolve) => setTimeout(resolve, 30));
    }
    throw new Error(`Timed out: ${fn}`);
  };
  await win.loadURL("about:blank");
  win.webContents.debugger.attach("1.3");
  const send = (method, params) => win.webContents.debugger.sendCommand(method, params);
  await send("Emulation.setTouchEmulationEnabled", { enabled: true, maxTouchPoints: 5 });
  await win.loadFile(path.join(root, "out/renderer/index.html"));
  win.focus();
  await waitFor(() => !!document.querySelector(".turn"));
  // Use the real renderer with fixture IPC, switching to the web host policy.
  await evaluate(() => { document.documentElement.dataset.hostKind = "web"; });
  win.setContentSize(800, 820);
  await waitFor(() => !document.querySelector(".app-shell").classList.contains("compact-navigation"));
  win.setContentSize(390, 820);
  await waitFor(() => document.querySelector(".app-shell").classList.contains("compact-navigation"));
  assert.equal(await evaluate(() => matchMedia("(pointer: coarse)").matches), true);
  await waitFor(() => document.querySelector(".app-shell")?.dataset.wuuSidebarMode === "collapsed");
  const swipe = async (points) => {
    for (let i = 0; i < points.length; i++) {
      await send("Input.dispatchTouchEvent", {
        type: i === 0 ? "touchStart" : "touchMove",
        touchPoints: [{ x: points[i][0], y: points[i][1] }],
      });
    }
    await send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
  };
  await evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
  await swipe([[16, 300], [32, 301], [60, 302], [110, 303]]);
  await waitFor(() => document.querySelector(".app-shell")?.dataset.wuuSidebarMode === "drawer");
  await evaluate(() => document.querySelector(".compact-session-switcher-backdrop").click());
  await waitFor(() => document.querySelector(".app-shell")?.dataset.wuuSidebarMode === "collapsed");
  await swipe([[160, 300], [200, 300], [260, 300]]);
  assert.equal(await evaluate(() => document.querySelector(".app-shell").dataset.wuuSidebarMode), "collapsed");
  await swipe([[16, 300], [18, 320], [20, 360]]);
  assert.equal(await evaluate(() => document.querySelector(".app-shell").dataset.wuuSidebarMode), "collapsed");
  console.log("PASS: Chromium touch edge swipe opens drawer; vertical/middle gestures do not; backdrop closes");
  clearTimeout(timeout);
  win.destroy();
  app.quit();
}).catch((error) => {
  console.error(error);
  app.exit(1);
});
