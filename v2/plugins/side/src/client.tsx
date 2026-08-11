import { useEffect, useRef, useSyncExternalStore } from "react";
import {
  Service,
  useProjection,
  type Context,
  type Plugin,
} from "@wuu-v2/client-runtime";
import { sideStyles } from "./styles.js";

type ConversationValue = {
  messages: Array<{ id: string; role: string; text: string; status: string }>;
  running: boolean;
};

interface SidePanelState {
  open: boolean;
  width: number;
  draft: string;
  resolving: boolean;
  sideSessionId?: string;
  error: string | undefined;
}

const initialState: SidePanelState = {
  open: false,
  width: 360,
  draft: "",
  resolving: false,
  error: undefined,
};

class SidePanelsService extends Service {
  private readonly states = new Map<string, SidePanelState>();
  private readonly listeners = new Set<() => void>();
  private readonly resolutions = new Map<string, Promise<string>>();
  private revision = 0;

  constructor(ctx: Context) {
    super(ctx, "sidePanels");
  }

  get(sessionId: string): SidePanelState {
    return this.states.get(sessionId) ?? initialState;
  }

  subscribe(listener: () => void): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  snapshot = (): number => this.revision;

  private update(sessionId: string, patch: Partial<SidePanelState>): void {
    this.states.set(sessionId, { ...this.get(sessionId), ...patch });
    this.revision += 1;
    for (const listener of this.listeners) listener();
  }

  private async resolve(sessionId: string): Promise<string> {
    const existing = this.get(sessionId).sideSessionId;
    if (existing) return existing;
    const pending = this.resolutions.get(sessionId);
    if (pending) return pending;
    this.update(sessionId, { resolving: true, error: undefined });
    const task = this.ctx.clientActions.execute("side/resolve", { sessionId }).then((result) => {
      if (!result || Array.isArray(result) || typeof result !== "object") {
        throw new Error("Side session resolution returned an invalid response");
      }
      const sideSessionId = result.sessionId;
      if (typeof sideSessionId !== "string") throw new Error("Side session resolution omitted sessionId");
      this.update(sessionId, { sideSessionId, resolving: false, error: undefined });
      return sideSessionId;
    }).catch((cause) => {
      this.update(sessionId, {
        resolving: false,
        error: cause instanceof Error ? cause.message : String(cause),
      });
      throw cause;
    }).finally(() => {
      this.resolutions.delete(sessionId);
    });
    this.resolutions.set(sessionId, task);
    return task;
  }

  async show(sessionId: string): Promise<void> {
    this.update(sessionId, { open: true });
    await this.resolve(sessionId);
  }

  hide(sessionId: string): void {
    this.update(sessionId, { open: false });
  }

  async toggle(sessionId: string): Promise<void> {
    if (this.get(sessionId).open) this.hide(sessionId);
    else await this.show(sessionId);
  }

  setDraft(sessionId: string, draft: string): void {
    this.update(sessionId, { draft });
  }

  setWidth(sessionId: string, width: number): void {
    this.update(sessionId, { width: Math.max(280, Math.min(720, Math.round(width))) });
  }

  async send(sessionId: string, draft: string): Promise<void> {
    const text = draft.trim();
    if (!text) return;
    await this.resolve(sessionId);
    await this.ctx.clientActions.execute("side/prompt", { sessionId, text });
    this.update(sessionId, { draft: "", error: undefined });
  }
}

declare module "cordis" {
  interface Context {
    sidePanels: SidePanelsService;
  }
}

function SidePanel({ client, sessionId }: { client: Context; sessionId?: string }) {
  useSyncExternalStore(
    client.sidePanels.subscribe.bind(client.sidePanels),
    client.sidePanels.snapshot,
    client.sidePanels.snapshot,
  );
  const state = sessionId ? client.sidePanels.get(sessionId) : initialState;
  const conversation = useProjection<ConversationValue>(
    client,
    state.sideSessionId ?? "",
    "conversation",
  );
  const log = useRef<HTMLDivElement>(null);
  const pinned = useRef(true);
  const resize = useRef<{
    pointerId: number;
    startX: number;
    startWidth: number;
  } | undefined>(undefined);

  useEffect(() => {
    if (pinned.current && log.current) log.current.scrollTop = log.current.scrollHeight;
  }, [conversation?.messages]);

  if (!sessionId) return null;
  if (!state.open) {
    return (
      <button
        className="side-open-button"
        type="button"
        aria-label="Open Side"
        onClick={() => void client.sidePanels.show(sessionId)}
      >
        Side
      </button>
    );
  }
  return (
    <aside className="side-panel" aria-label="Side conversation" style={{ width: state.width }}>
      <div
        className="side-resizer"
        role="separator"
        aria-label="Resize Side"
        aria-orientation="vertical"
        onPointerDown={(event) => {
          event.currentTarget.setPointerCapture(event.pointerId);
          resize.current = {
            pointerId: event.pointerId,
            startX: event.clientX,
            startWidth: state.width,
          };
        }}
        onPointerMove={(event) => {
          const active = resize.current;
          if (!active || active.pointerId !== event.pointerId) return;
          client.sidePanels.setWidth(
            sessionId,
            active.startWidth + active.startX - event.clientX,
          );
        }}
        onPointerUp={(event) => {
          if (resize.current?.pointerId !== event.pointerId) return;
          resize.current = undefined;
          event.currentTarget.releasePointerCapture(event.pointerId);
        }}
      />
      <header className="side-header">
        <strong>Side</strong>
        <button type="button" aria-label="Close Side" onClick={() => client.sidePanels.hide(sessionId)}>Close</button>
      </header>
      <div
        ref={log}
        className="side-conversation"
        role="log"
        aria-live="polite"
        onScroll={(event) => {
          const element = event.currentTarget;
          pinned.current = element.scrollHeight - element.scrollTop - element.clientHeight < 24;
        }}
      >
        {(conversation?.messages ?? []).map((message) => (
          <article key={message.id} className={`side-message side-message-${message.role}`} data-status={message.status}>
            {message.text}
          </article>
        ))}
        {state.resolving ? <p>Opening Side…</p> : null}
        {state.error ? <p role="alert">{state.error}</p> : null}
      </div>
      <footer className="side-composer-host">
        {state.sideSessionId ? client.composerSurfaces.render({
          sessionId: state.sideSessionId,
          draft: state.draft,
          running: conversation?.running ?? false,
          ariaLabel: "Message Side",
          autoFocus: true,
          placeholder: "Ask a side question",
          commandContext: { kind: "side", ownerSessionId: sessionId },
          onDraftChange: (draft) => client.sidePanels.setDraft(sessionId, draft),
          onSubmit: (draft) => client.sidePanels.send(sessionId, draft),
          onCancel: async () => {
            await client.clientActions.execute("side/cancel", { sessionId });
          },
        }) : null}
      </footer>
    </aside>
  );
}

const sideClient: Plugin = function side(client) {
  new SidePanelsService(client);
  client.slots.contribute("layout/side", {
    id: "side-panel",
    component: SidePanel,
  });
  void client.inject(["sidePanels", "slashCommands"], (slashClient) => {
    slashClient.slashCommands.register({
      id: "side.toggle",
      name: "side",
      title: "Toggle Side conversation",
      keywords: ["panel", "secondary", "chat"],
      execute: async ({ sessionId, surface }) => {
        const ownerSessionId = surface && typeof surface === "object" && !Array.isArray(surface)
          && surface.kind === "side" && typeof surface.ownerSessionId === "string"
          ? surface.ownerSessionId
          : sessionId;
        await slashClient.sidePanels.toggle(ownerSessionId);
        return { type: "handled" };
      },
    });
  });
  client.effect(() => {
    if (typeof document === "undefined") return () => {};
    const style = document.createElement("style");
    style.dataset.wuuPluginStyle = "side";
    style.textContent = sideStyles;
    document.head.append(style);
    return () => style.remove();
  }, "install Side styles");
};

sideClient.inject = ["clientActions", "clientProjections", "composerSurfaces", "slots"];
sideClient.provide = "sidePanels";
export default sideClient;
