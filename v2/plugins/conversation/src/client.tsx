import { createElement, useCallback, useEffect, useRef } from "react";
import {
  Service,
  SlotOutlet,
  useProjection,
  useScopedStore,
  type Context,
  type Plugin,
  type SlotHandle,
} from "@wuu-v2/client-runtime";
import { conversationStyles } from "./styles.js";
import { MarkdownText } from "./markdown.js";
import type {
  ConversationItem,
  ConversationMessageItem,
  ConversationStatusItem,
  ConversationToolItem,
  ConversationValue,
} from "./shared.js";

interface ConversationItemsProps {
  client: Context;
  sessionId: string;
  items: readonly ConversationItem[];
  messageSlot: SlotHandle;
  statusSlot: SlotHandle;
  toolSlot: SlotHandle;
}

function ConversationItems({
  client,
  sessionId,
  items,
  messageSlot,
  statusSlot,
  toolSlot,
}: ConversationItemsProps) {
  return items.map((item) => (
    <SlotOutlet
      key={item.id}
      client={client}
      slot={item.kind === "message" ? messageSlot : item.kind === "tool" ? toolSlot : statusSlot}
      sessionId={sessionId}
      ownerProps={item}
    />
  ));
}

function MarkdownMessage({ ownerProps }: { ownerProps?: unknown }) {
  const message = ownerProps as ConversationMessageItem;
  const terminal = message.status === "error"
    ? "Response failed"
    : message.status === "failed"
      ? "Response failed"
      : message.status === "cancelled"
        ? "Response cancelled"
        : message.status === "interrupted" ? "Response interrupted" : undefined;
  return (
    <article className={`message message-${message.role}`} data-status={message.status}>
      {message.text ? <MarkdownText text={message.text} /> : null}
      {terminal ? <small className="message-terminal" role="status">{terminal}</small> : null}
    </article>
  );
}

function ConversationStatus({ ownerProps }: { ownerProps?: unknown }) {
  const status = ownerProps as ConversationStatusItem;
  return <p className="conversation-status" data-status={status.status} role="status">{status.text}</p>;
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
  constructor(
    ctx: Context,
    private readonly messageSlot: SlotHandle,
    private readonly statusSlot: SlotHandle,
    private readonly toolSlot: SlotHandle,
  ) {
    super(ctx, "conversationSurfaces");
  }

  render(sessionId: string, items: readonly ConversationItem[]) {
    return createElement(ConversationItems, {
      client: this.ctx,
      sessionId,
      items,
      messageSlot: this.messageSlot,
      statusSlot: this.statusSlot,
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
  let messageSlot: SlotHandle;
  let statusSlot: SlotHandle;
  let toolSlot: SlotHandle;
  const scrollStates = client.scopedStores.define("conversation/scroll", () => ({
    top: 0,
    pinned: true,
  }));
  function SessionConversation({
    componentClient,
    sessionId,
  }: {
    componentClient: Context;
    sessionId: string;
  }) {
    const value = useProjection<ConversationValue>(componentClient, sessionId, "conversation");
    const [scrollState, setScrollState] = useScopedStore(scrollStates, sessionId);
    const scroll = useRef<HTMLDivElement>(null);
    const lastScrollTop = useRef(0);
    const onComposerHeightChange = useCallback(() => {
      const element = scroll.current;
      if (!scrollState.pinned || !element) return;
      const nextTop = Math.max(0, element.scrollHeight - element.clientHeight);
      lastScrollTop.current = nextTop;
      element.scrollTop = nextTop;
    }, [scrollState.pinned]);
    useEffect(() => {
      const element = scroll.current;
      if (!element) return;
      const nextTop = scrollState.pinned
        ? Math.max(0, element.scrollHeight - element.clientHeight)
        : Math.min(scrollState.top, Math.max(0, element.scrollHeight - element.clientHeight));
      lastScrollTop.current = nextTop;
      element.scrollTop = nextTop;
    }, [sessionId, value?.items, scrollState.pinned]);
    return (
      <section className="conversation-shell">
        <div
          ref={scroll}
          className="conversation-scroll"
          onScroll={(event) => {
            const element = event.currentTarget;
            const movedUp = element.scrollTop < lastScrollTop.current;
            lastScrollTop.current = element.scrollTop;
            const next = {
              top: element.scrollTop,
              pinned: !movedUp &&
                element.scrollHeight - element.scrollTop - element.clientHeight < 24,
            };
            setScrollState((current) =>
              current.top === next.top && current.pinned === next.pinned ? current : next);
          }}
        >
          {componentClient.conversationSurfaces.render(sessionId, value?.items ?? [])}
        </div>
        <SlotOutlet
          client={componentClient}
          slot={composerSlot}
          sessionId={sessionId}
          ownerProps={{
            running: value?.running ?? false,
            onVisualHeightChange: onComposerHeightChange,
          }}
        />
      </section>
    );
  }
  function Conversation({ client: componentClient, sessionId }: { client: Context; sessionId?: string }) {
    return sessionId
      ? <SessionConversation componentClient={componentClient} sessionId={sessionId} />
      : <div className="conversation-empty">Choose or create a task</div>;
  }

  const registration = client.slots.contribute("layout/conversation", {
    id: "conversation",
    component: Conversation,
    children: [
      { name: "conversation/composer", kind: "chain", scope: "session" },
      { name: "conversation/message", kind: "chain", scope: "session" },
      { name: "conversation/status", kind: "chain", scope: "session" },
      { name: "conversation/tool", kind: "chain", scope: "session" },
    ],
  });
  composerSlot = registration.children.get("conversation/composer")!;
  messageSlot = registration.children.get("conversation/message")!;
  statusSlot = registration.children.get("conversation/status")!;
  toolSlot = registration.children.get("conversation/tool")!;
  new ConversationSurfacesService(client, messageSlot, statusSlot, toolSlot);
  client.slots.contribute("conversation/message", {
    id: "markdown-message",
    priority: -100,
    component: MarkdownMessage,
  });
  client.slots.contribute("conversation/status", {
    id: "conversation-status",
    priority: -100,
    component: ConversationStatus,
  });
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
conversationClient.inject = ["clientProjections", "scopedStores", "slots"];
conversationClient.provide = "conversationSurfaces";
export default conversationClient;
