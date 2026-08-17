// App shell: three screens (pair → chats → thread) with hand-rolled
// navigation — the surface is small enough that a navigation library would
// outweigh the app. The controller is a module singleton; screens read the
// store via useSyncExternalStore.

import { useEffect, useState, useSyncExternalStore } from "react";

import { ConnectionBanner } from "./components/ConnectionBanner";
import { webCredStore } from "./lib/credStore";
import { WuuMobile } from "./lib/controller";
import { PairScreen } from "./screens/PairScreen";
import { HomeScreen } from "./screens/HomeScreen";
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
    // Switching conversations from the drawer marks the previous one viewed
    // before the new active thread advances its own unread cursor.
    const active = controller.store.getSnapshot().activeThreadId;
    if (active && active !== threadId) controller.closeThread();
    setRoute({ name: "thread", threadId });
    void controller.openThread(threadId).catch(() => {});
  };

  /** Home composer: create a conversation in the host workspace, send the
   *  first message, then move into that thread's full chat view. */
  const startChat = async (text: string): Promise<void> => {
    const thread = await controller.startThread();
    await controller.sendMessage(thread, text);
    setRoute({ name: "thread", threadId: thread.id });
    await controller.openThread(thread.id).catch(() => {});
  };

  const chatList = (
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
  );

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
          <HomeScreen
            snapshot={snapshot}
            onCompose={startChat}
            onSelectWorkspace={(workspace) =>
              void controller.selectWorkspace(workspace).catch(() => {})
            }
            drawerContent={chatList}
          />
        </>
      ) : (
        <>
          <ConnectionBanner phase={snapshot.phase} syncError={snapshot.syncError} />
          <ThreadScreen
            snapshot={snapshot}
            threadId={route.threadId}
            onSend={(thread, text) => void controller.sendMessage(thread, text).catch(() => {})}
            onInterrupt={(threadId) => void controller.interrupt(threadId).catch(() => {})}
            drawerContent={chatList}
          />
        </>
      )}
    </div>
  );
}
