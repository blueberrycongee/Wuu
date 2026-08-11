import { SlotOutlet, type Context, type Plugin, type SlotHandle } from "@wuu-v2/client-runtime";
import { layoutStyles } from "./styles.js";

const layoutClient: Plugin = function layout(client) {
  let conversationSlot: SlotHandle;
  let sideSlot: SlotHandle;
  function AppFrame({ client: componentClient, ownerProps }: { client: Context; ownerProps?: unknown }) {
    const sessionId = (ownerProps as { sessionId?: string } | undefined)?.sessionId;
    return (
      <div className="app-shell">
        <aside className="app-sidebar" aria-label="Wuu sidebar" />
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
    );
  }

  const registration = client.slots.contribute("root", {
    id: "default-layout",
    component: AppFrame,
    children: [
      { name: "layout/conversation", kind: "single", scope: "session" },
      { name: "layout/side", kind: "single", scope: "session" },
    ],
  });
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

layoutClient.inject = ["slots"];
export default layoutClient;
