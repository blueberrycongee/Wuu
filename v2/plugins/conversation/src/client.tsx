import { SlotOutlet, useProjection, type Context, type Plugin, type SlotHandle } from "@wuu-v2/client-runtime";
type ConversationValue = {
  messages: Array<{ id: string; role: string; text: string; status: string }>;
  running: boolean;
};

const conversationClient: Plugin = function conversation(client) {
  let composerSlot: SlotHandle;
  function Conversation({ client: componentClient, sessionId }: { client: Context; sessionId?: string }) {
    const value = useProjection<ConversationValue>(componentClient, sessionId ?? "", "conversation");
    if (!sessionId) return <div className="conversation-empty">Choose or create a task</div>;
    return (
      <section className="conversation-shell">
        <div className="conversation-scroll">
          {(value?.messages ?? []).map((message) => (
            <article className={`message message-${message.role}`} key={message.id} data-status={message.status}>
              {message.text}
            </article>
          ))}
        </div>
        <SlotOutlet
          client={componentClient}
          slot={composerSlot}
          sessionId={sessionId}
          ownerProps={{ running: value?.running ?? false }}
        />
      </section>
    );
  }

  const registration = client.slots.contribute("layout/conversation", {
    id: "conversation",
    component: Conversation,
    children: [{ name: "conversation/composer", kind: "chain", scope: "session" }],
  });
  composerSlot = registration.children.get("conversation/composer")!;
};
conversationClient.inject = ["clientProjections", "slots"];
export default conversationClient;
