import { useEffect, useState, useSyncExternalStore } from "react";
import {
  SlotOutlet,
  useActiveSession,
  type Context,
  type Plugin,
  type SlotHandle,
} from "@wuu-v2/client-runtime";
import { DialogLayerHost } from "@wuu-v2/ui-kit";
import { layoutStyles } from "./styles.js";

const layoutClient: Plugin = function layout(client) {
  let conversationSlot: SlotHandle;
  let sideSlot: SlotHandle;
  let sidebarSlot: SlotHandle;
  function AppFrame({ client: componentClient, ownerProps }: { client: Context; ownerProps?: unknown }) {
    const [sidebarOpen, setSidebarOpen] = useState(false);
    const initialSessionId = (ownerProps as { sessionId?: string } | undefined)?.sessionId;
    const selectedSessionId = useActiveSession(componentClient);
    const sessionId = selectedSessionId ?? initialSessionId;
    useSyncExternalStore(
      componentClient.slots.subscribe.bind(componentClient.slots),
      componentClient.slots.snapshot,
      componentClient.slots.snapshot,
    );
    const hasSidebar = componentClient.slots.renderEntries(sidebarSlot, {
      client: componentClient,
      ...(sessionId ? { sessionId } : {}),
    }).length > 0;
    useEffect(() => {
      if (!selectedSessionId && initialSessionId) componentClient.activeSession.select(initialSessionId);
    }, [componentClient, initialSessionId, selectedSessionId]);
    useEffect(() => setSidebarOpen(false), [selectedSessionId]);
    return (
      <DialogLayerHost>
      <div className={`app-shell${hasSidebar ? "" : " is-sidebar-empty"}${sidebarOpen ? " is-sidebar-open" : ""}`}>
        {hasSidebar ? (
          <button
            type="button"
            className="app-sidebar-toggle"
            aria-label={sidebarOpen ? "Close task history" : "Open task history"}
            aria-expanded={sidebarOpen}
            onClick={() => setSidebarOpen((value) => !value)}
          >
            {sidebarOpen ? "×" : "☰"}
          </button>
        ) : null}
        {hasSidebar ? (
          <aside className="app-sidebar" aria-label="Wuu sidebar">
            <SlotOutlet
              client={componentClient}
              slot={sidebarSlot}
              {...(sessionId ? { sessionId } : {})}
            />
          </aside>
        ) : null}
        <main className="conversation-pane">
          <SlotOutlet
            client={componentClient}
            slot={conversationSlot}
            {...(sessionId ? { sessionId } : {})}
          />
        </main>
        <SlotOutlet
          client={componentClient}
          slot={sideSlot}
          {...(sessionId ? { sessionId } : {})}
        />
      </div>
      </DialogLayerHost>
    );
  }

  const registration = client.slots.contribute("root", {
    id: "default-layout",
    component: AppFrame,
    children: [
      { name: "layout/sidebar", kind: "single", scope: "session-maybe" },
      { name: "layout/conversation", kind: "single", scope: "session" },
      { name: "layout/side", kind: "single", scope: "session" },
    ],
  });
  sidebarSlot = registration.children.get("layout/sidebar")!;
  conversationSlot = registration.children.get("layout/conversation")!;
  sideSlot = registration.children.get("layout/side")!;
  client.effect(() => {
    if (typeof document === "undefined") return () => {};
    const style = document.createElement("style");
    style.dataset.wuuPluginStyle = "layout";
    style.textContent = layoutStyles;
    document.head.append(style);
    return () => style.remove();
  }, "install layout styles");
};

layoutClient.inject = ["activeSession", "slots"];
export default layoutClient;
