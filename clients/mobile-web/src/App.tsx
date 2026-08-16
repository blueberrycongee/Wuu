// App shell: three screens (pair → chats → thread) with hand-rolled
// navigation — the surface is small enough that a navigation library would
// outweigh the app. The controller is a module singleton; screens read the
// store via useSyncExternalStore.

import { useEffect, useState, useSyncExternalStore } from "react";

import { ConnectionBanner } from "./components/ConnectionBanner";
import { webCredStore } from "./lib/credStore";
import { WuuMobile } from "./lib/controller";
import { PairScreen } from "./screens/PairScreen";
import { ChatsScreen } from "./screens/ChatsScreen";
import { ThreadScreen } from "./screens/ThreadScreen";

const controller = new WuuMobile(webCredStore);

type Route =
  | { name: "boot" }
  | { name: "pair" }
  | { name: "chats" }
  | { name: "thread"; threadId: string };

export default function App(): React.JSX.Element {
  const [route, setRoute] = useState<Route>({ name: "boot" });
  const snapshot = useSyncExternalStore(controller.store.subscribe, controller.store.getSnapshot);

  useEffect(() => {
    void controller
      .startFromStoredCredentials()
      .then((hadCredentials) => {
        setRoute(hadCredentials ? { name: "chats" } : { name: "pair" });
      })
      .catch(() => {
        setRoute({ name: "pair" });
      });
  }, []);

  const openThread = (threadId: string): void => {
    setRoute({ name: "thread", threadId });
    void controller.openThread(threadId).catch(() => {});
  };

  const closeThread = (): void => {
    controller.closeThread();
    setRoute({ name: "chats" });
  };

  return (
    <div className="app">
      {route.name === "boot" ? (
        <div className="chats-empty">正在启动…</div>
      ) : route.name === "pair" ? (
        <PairScreen
          onPair={async (uri, deviceName) => {
            const creds = await controller.pairWithUri(uri, deviceName);
            return creds.host_name ?? "";
          }}
          onDone={() => setRoute({ name: "chats" })}
        />
      ) : route.name === "chats" ? (
        <>
          <ConnectionBanner phase={snapshot.phase} syncError={snapshot.syncError} />
          <ChatsScreen
            snapshot={snapshot}
            onRefresh={() => void controller.refreshThreads().catch(() => {})}
            onOpenThread={(thread) => openThread(thread.id)}
            onTogglePin={(thread) => void controller.togglePin(thread).catch(() => {})}
            onNewThread={() => controller.startThread()}
            onUnpair={() => {
              void controller.unpair().then(() => setRoute({ name: "pair" }));
            }}
          />
        </>
      ) : (
        <>
          <ConnectionBanner phase={snapshot.phase} syncError={snapshot.syncError} />
          <ThreadScreen
            snapshot={snapshot}
            threadId={route.threadId}
            onBack={closeThread}
            onSend={(thread, text) => void controller.sendMessage(thread, text).catch(() => {})}
            onInterrupt={(threadId) => void controller.interrupt(threadId).catch(() => {})}
          />
        </>
      )}
    </div>
  );
}
