import { createElement, useEffect, useRef } from "react";
import {
  Service,
  SlotOutlet,
  useProjection,
  type Context,
  type Plugin,
  type SlotHandle,
} from "@wuu-v2/client-runtime";
import { conversationStyles } from "./styles.js";
import type { ConversationItem, ConversationToolItem, ConversationValue } from "./shared.js";

interface ConversationItemsProps {
  client: Context;
  sessionId: string;
  items: readonly ConversationItem[];
  toolSlot: SlotHandle;
}

function ConversationItems({ client, sessionId, items, toolSlot }: ConversationItemsProps) {
  return items.map((item) => item.kind === "message" ? (
    <article className={`message message-${item.role}`} key={item.id} data-status={item.status}>
      {item.text}
    </article>
  ) : (
    <SlotOutlet
      key={item.id}
      client={client}
      slot={toolSlot}
      sessionId={sessionId}
      ownerProps={item}
    />
  ));
}

function GenericToolActivity({ ownerProps }: { ownerProps?: unknown }) {
  const tool = ownerProps as ConversationToolItem;
  return (
    <article className="tool-activity" data-status={tool.status}>
      <div className="tool-activity-heading">
        <code>{tool.name}</code>
        <span>{tool.status}</span>
      </div>
      <details>
        <summary>Input</summary>
        <pre>{JSON.stringify(tool.input, null, 2)}</pre>
      </details>
      {tool.result === null ? null : <pre className="tool-activity-result">{tool.result}</pre>}
    </article>
  );
}

export class ConversationSurfacesService extends Service {
  constructor(ctx: Context, private readonly toolSlot: SlotHandle) {
    super(ctx, "conversationSurfaces");
  }

  render(sessionId: string, items: readonly ConversationItem[]) {
    return createElement(ConversationItems, {
      client: this.ctx,
      sessionId,
      items,
      toolSlot: this.toolSlot,
    });
  }
}

declare module "cordis" {
  interface Context {
    conversationSurfaces: ConversationSurfacesService;
  }
}

const conversationClient: Plugin = function conversation(client) {
  let composerSlot: SlotHandle;
  let toolSlot: SlotHandle;
  function Conversation({ client: componentClient, sessionId }: { client: Context; sessionId?: string }) {
    const value = useProjection<ConversationValue>(componentClient, sessionId ?? "", "conversation");
    const scroll = useRef<HTMLDivElement>(null);
    const pinned = useRef(true);
    useEffect(() => {
      if (pinned.current && scroll.current) scroll.current.scrollTop = scroll.current.scrollHeight;
    }, [sessionId, value?.items]);
    if (!sessionId) return <div className="conversation-empty">Choose or create a task</div>;
    return (
      <section className="conversation-shell">
        <div
          ref={scroll}
          className="conversation-scroll"
          onScroll={(event) => {
            const element = event.currentTarget;
            pinned.current = element.scrollHeight - element.scrollTop - element.clientHeight < 24;
          }}
        >
          {componentClient.conversationSurfaces.render(sessionId, value?.items ?? [])}
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
    children: [
      { name: "conversation/composer", kind: "chain", scope: "session" },
      { name: "conversation/tool", kind: "chain", scope: "session" },
    ],
  });
  composerSlot = registration.children.get("conversation/composer")!;
  toolSlot = registration.children.get("conversation/tool")!;
  new ConversationSurfacesService(client, toolSlot);
  client.slots.contribute("conversation/tool", {
    id: "generic-tool-activity",
    priority: -100,
    component: GenericToolActivity,
  });
  client.effect(() => {
    if (typeof document === "undefined") return () => {};
    const style = document.createElement("style");
    style.dataset.wuuPluginStyle = "conversation";
    style.textContent = conversationStyles;
    document.head.append(style);
    return () => style.remove();
  }, "install conversation styles");
};
conversationClient.inject = ["clientProjections", "slots"];
conversationClient.provide = "conversationSurfaces";
export default conversationClient;
