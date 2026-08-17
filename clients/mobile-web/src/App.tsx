// App shell: pair → home ⇄ thread, ChatGPT-style. Home is the new-chat
// screen (greeting + composer + workspace picker); history lives in a left
// drawer shared by both screens. The controller is a module singleton;
// screens read the store via useSyncExternalStore.

import { useEffect, useState, useSyncExternalStore } from "react";

import { ChatDrawer } from "./components/ChatDrawer";
import { ConnectionBanner } from "./components/ConnectionBanner";
import { webCredStore } from "./lib/credStore";
import { WuuMobile } from "./lib/controller";
import { PairScreen } from "./screens/PairScreen";
import { HomeScreen } from "./screens/HomeScreen";
import { ThreadScreen } from "./screens/ThreadScreen";

const controller = new WuuMobile(webCredStore);

type Route =
  | { name: "boot" }
  | { name: "pair" }
  | { name: "home" }
  | { name: "thread"; threadId: string };

export default function App(): React.JSX.Element {
  const [route, setRoute] = useState<Route>({ name: "boot" });
  const [drawerOpen, setDrawerOpen] = useState(false);
  const snapshot = useSyncExternalStore(controller.store.subscribe, controller.store.getSnapshot);

  useEffect(() => {
    void controller
      .startFromStoredCredentials()
      .then((hadCredentials) => {
        setRoute(hadCredentials ? { name: "home" } : { name: "pair" });
      })
      .catch(() => {
        setRoute({ name: "pair" });
      });
  }, []);

  const openThread = (threadId: string): void => {
    setDrawerOpen(false);
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

  /** Drawer "新对话": back to the home composer, like ChatGPT's new chat. */
  const newChat = (): void => {
    setDrawerOpen(false);
    if (route.name === "thread") controller.closeThread();
    if (route.name !== "home") setRoute({ name: "home" });
  };

  return (
    <div className="app">
      {route.name === "boot" ? (
        <div className="boot-note">正在启动…</div>
      ) : route.name === "pair" ? (
        <PairScreen
          onPair={async (uri, deviceName) => {
            const creds = await controller.pairWithUri(uri, deviceName);
            return creds.host_name ?? "";
          }}
          onDone={() => setRoute({ name: "home" })}
        />
      ) : (
        <>
          <ConnectionBanner phase={snapshot.phase} syncError={snapshot.syncError} />
          {route.name === "home" ? (
            <HomeScreen
              snapshot={snapshot}
              onCompose={startChat}
              onSelectWorkspace={(workspace) =>
                void controller.selectWorkspace(workspace).catch(() => {})
              }
              onOpenDrawer={() => setDrawerOpen(true)}
            />
          ) : (
            <ThreadScreen
              snapshot={snapshot}
              threadId={route.threadId}
              onSend={(thread, text) => void controller.sendMessage(thread, text).catch(() => {})}
              onInterrupt={(threadId) => void controller.interrupt(threadId).catch(() => {})}
              onOpenDrawer={() => setDrawerOpen(true)}
            />
          )}
          <ChatDrawer
            open={drawerOpen}
            snapshot={snapshot}
            onClose={() => setDrawerOpen(false)}
            onNewChat={newChat}
            onOpenThread={(thread) => openThread(thread.id)}
            onTogglePin={(thread) => void controller.togglePin(thread).catch(() => {})}
            onRefresh={() => void controller.refreshThreads().catch(() => {})}
            onUnpair={() => {
              setDrawerOpen(false);
              void controller.unpair().then(() => setRoute({ name: "pair" }));
            }}
          />
        </>
      )}
    </div>
  );
}
