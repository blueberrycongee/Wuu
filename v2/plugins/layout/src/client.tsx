import { SlotOutlet, type Context, type Plugin, type SlotHandle } from "@wuu-v2/client-runtime";

const layoutClient: Plugin = function layout(client) {
  let conversationSlot: SlotHandle;
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
      </div>
    );
  }

  const registration = client.slots.contribute("root", {
    id: "default-layout",
    component: AppFrame,
    children: [{ name: "layout/conversation", kind: "single", scope: "session" }],
  });
  conversationSlot = registration.children.get("layout/conversation")!;
};

layoutClient.inject = ["slots"];
export default layoutClient;
