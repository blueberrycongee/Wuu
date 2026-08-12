import { contextBridge, ipcRenderer, type IpcRendererEvent } from "electron";
import type { JsonValue, ProjectionFrame } from "@wuu-v2/contracts";
import {
  bridgeChannels,
  type DesktopBridge,
  type HarnessState,
} from "../shared/bridge.js";

const projectionListeners = new Map<string, (frame: ProjectionFrame) => void>();
const stateListeners = new Set<(state: HarnessState) => void>();

ipcRenderer.on(
  bridgeChannels.projection,
  (_event: IpcRendererEvent, message: { subscriptionId: string; frame: ProjectionFrame }) => {
    projectionListeners.get(message.subscriptionId)?.(message.frame);
  },
);
ipcRenderer.on(bridgeChannels.state, (_event: IpcRendererEvent, state: HarnessState) => {
  for (const listener of stateListeners) listener(state);
});

const bridge: DesktopBridge = {
  boot: () => ipcRenderer.invoke(bridgeChannels.boot),
  restart: () => ipcRenderer.invoke(bridgeChannels.restart),
  action: (action: string, input: JsonValue) =>
    ipcRenderer.invoke(bridgeChannels.action, action, input),
  follow(sessionId, listener) {
    const subscriptionId = globalThis.crypto.randomUUID();
    projectionListeners.set(subscriptionId, listener);
    ipcRenderer.send(bridgeChannels.follow, subscriptionId, sessionId);
    return () => {
      projectionListeners.delete(subscriptionId);
      ipcRenderer.send(bridgeChannels.unfollow, subscriptionId);
    };
  },
  onHarnessState(listener) {
    stateListeners.add(listener);
    return () => stateListeners.delete(listener);
  },
};

contextBridge.exposeInMainWorld("wuuV2", bridge);
