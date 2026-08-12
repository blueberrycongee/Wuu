import type { DesktopBridge } from "../shared/bridge.js";

declare global {
  interface Window {
    wuuV2: DesktopBridge;
  }
}

export {};
