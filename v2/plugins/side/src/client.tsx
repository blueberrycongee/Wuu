import { useCallback, useEffect, useRef, useSyncExternalStore, type CSSProperties } from "react";
import {
  Service,
  useProjection,
  type Context,
  type Plugin,
  type ScopedStoreSeat,
} from "@wuu-v2/client-runtime";
import type {} from "@wuu-v2/plugin-conversation/client";
import type { ConversationValue } from "@wuu-v2/plugin-conversation/shared";
import { sideStyles } from "./styles.js";

interface SidePanelState {
  open: boolean;
  width: number;
  draft: string;
  scrollTop: number;
  pinned: boolean;
  resolving: boolean;
  sideSessionId?: string;
  error: string | undefined;
}

const initialState: SidePanelState = {
  open: false,
  width: 360,
  draft: "",
  scrollTop: 0,
  pinned: true,
  resolving: false,
  error: undefined,
};

class SidePanelsService extends Service {
  private readonly resolutions = new Map<string, Promise<string>>();
  private readonly owners = new Map<string, string>();
  private closed = false;

  constructor(ctx: Context, private readonly states: ScopedStoreSeat<SidePanelState>) {
    super(ctx, "sidePanels");
    ctx.effect(() => () => {
      this.closed = true;
      this.resolutions.clear();
      this.owners.clear();
    }, "dispose Side panel state");
  }

  get(sessionId: string): SidePanelState {
    return this.states.get(this.ownerSession(sessionId));
  }

  subscribe(sessionId: string, listener: () => void): () => void {
    return this.states.subscribe(this.ownerSession(sessionId), listener);
  }

  snapshot(sessionId: string): SidePanelState {
    return this.get(sessionId);
  }

  private update(sessionId: string, patch: Partial<SidePanelState>): void {
    if (this.closed) return;
    this.states.set(sessionId, (current) => ({ ...current, ...patch }));
  }

  private ownerSession(sessionId: string): string {
    return this.owners.get(sessionId) ?? sessionId;
  }

  private async resolve(sessionId: string): Promise<string> {
    if (this.closed) throw new Error("Side plugin is stopping");
    const existing = this.get(sessionId).sideSessionId;
    if (existing) return existing;
    const pending = this.resolutions.get(sessionId);
    if (pending) return pending;
    this.update(sessionId, { resolving: true, error: undefined });
    const task = this.ctx.clientActions.execute("side/resolve", { sessionId }).then((result) => {
      if (this.closed) throw new Error("Side plugin is stopping");
      if (!result || Array.isArray(result) || typeof result !== "object") {
        throw new Error("Side session resolution returned an invalid response");
      }
      const sideSessionId = result.sessionId;
      if (typeof sideSessionId !== "string") throw new Error("Side session resolution omitted sessionId");
      this.owners.set(sideSessionId, sessionId);
      this.update(sessionId, { sideSessionId, resolving: false, error: undefined });
      return sideSessionId;
    }).catch((cause) => {
      if (this.closed) throw cause;
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
    if (this.closed) throw new Error("Side plugin is stopping");
    const owner = this.ownerSession(sessionId);
    this.update(owner, { open: true });
    await this.resolve(owner);
  }

  hide(sessionId: string): void {
    this.update(this.ownerSession(sessionId), { open: false });
  }

  async toggle(sessionId: string): Promise<void> {
    if (this.closed) throw new Error("Side plugin is stopping");
    const owner = this.ownerSession(sessionId);
    if (this.get(owner).open) this.hide(owner);
    else await this.show(owner);
  }

  isOpen(sessionId: string): boolean {
    return this.get(sessionId).open;
  }

  setDraft(sessionId: string, draft: string): void {
    this.update(sessionId, { draft });
  }

  setWidth(sessionId: string, width: number): void {
    this.update(sessionId, { width: Math.max(280, Math.min(720, Math.round(width))) });
  }

  setScroll(sessionId: string, scrollTop: number, pinned: boolean): void {
    const current = this.get(sessionId);
    if (current.scrollTop === scrollTop && current.pinned === pinned) return;
    this.update(sessionId, { scrollTop, pinned });
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

function SideToggle({ client, sessionId }: { client: Context; sessionId?: string; ownerProps?: unknown }) {
  const scope = sessionId ?? "";
  useSyncExternalStore(
    useCallback((listener) => client.sidePanels.subscribe(scope, listener), [client, scope]),
    useCallback(() => client.sidePanels.snapshot(scope), [client, scope]),
    useCallback(() => client.sidePanels.snapshot(scope), [client, scope]),
  );
  if (!sessionId) return null;
  const open = client.sidePanels.isOpen(sessionId);
  return (
    <button
      className="side-toolbar-toggle"
      type="button"
      aria-label={open ? "Close Side" : "Open Side"}
      aria-pressed={open}
      onClick={() => void client.sidePanels.toggle(sessionId).catch(() => {})}
    >
      {open ? "Side ✓" : "Side"}
    </button>
  );
}

function SidePanel({ client, sessionId }: { client: Context; sessionId?: string }) {
  const scope = sessionId ?? "";
  useSyncExternalStore(
    useCallback((listener) => client.sidePanels.subscribe(scope, listener), [client, scope]),
    useCallback(() => client.sidePanels.snapshot(scope), [client, scope]),
    useCallback(() => client.sidePanels.snapshot(scope), [client, scope]),
  );
  const state = sessionId ? client.sidePanels.get(sessionId) : initialState;
  const conversation = useProjection<ConversationValue>(
    client,
    state.sideSessionId ?? "",
    "conversation",
  );
  const log = useRef<HTMLDivElement>(null);
  const lastScrollTop = useRef(0);
  const resize = useRef<{
    pointerId: number;
    startX: number;
    startWidth: number;
  } | undefined>(undefined);
  const onComposerHeightChange = useCallback(() => {
    const element = log.current;
    if (!state.pinned || !element) return;
    const nextTop = Math.max(0, element.scrollHeight - element.clientHeight);
    lastScrollTop.current = nextTop;
    element.scrollTop = nextTop;
  }, [state.pinned]);

  useEffect(() => {
    const element = log.current;
    if (!element) return;
    const nextTop = state.pinned
      ? Math.max(0, element.scrollHeight - element.clientHeight)
      : Math.min(state.scrollTop, Math.max(0, element.scrollHeight - element.clientHeight));
    lastScrollTop.current = nextTop;
    element.scrollTop = nextTop;
  }, [conversation?.items, sessionId, state.open, state.pinned]);

  if (!sessionId) return null;
  if (!state.open) return null;
  return (
    <aside
      className="side-panel"
      aria-label="Side conversation"
      style={{ "--side-width": `${state.width}px` } as CSSProperties}
    >
      <div
        className="side-resizer"
        role="separator"
        aria-label="Resize Side"
        aria-orientation="vertical"
        aria-valuemin={280}
        aria-valuemax={720}
        aria-valuenow={state.width}
        tabIndex={0}
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
        onLostPointerCapture={() => {
          resize.current = undefined;
        }}
        onKeyDown={(event) => {
          if (event.key === "Home") client.sidePanels.setWidth(sessionId, 280);
          else if (event.key === "End") client.sidePanels.setWidth(sessionId, 720);
          else if (event.key === "ArrowLeft") client.sidePanels.setWidth(sessionId, state.width + 16);
          else if (event.key === "ArrowRight") client.sidePanels.setWidth(sessionId, state.width - 16);
          else return;
          event.preventDefault();
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
          const movedUp = element.scrollTop < lastScrollTop.current;
          lastScrollTop.current = element.scrollTop;
          client.sidePanels.setScroll(
            sessionId,
            element.scrollTop,
            !movedUp && element.scrollHeight - element.scrollTop - element.clientHeight < 24,
          );
        }}
      >
        {state.sideSessionId
          ? client.conversationSurfaces.render(state.sideSessionId, conversation?.items ?? [])
          : null}
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
          onVisualHeightChange: onComposerHeightChange,
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
  const states = client.scopedStores.define("side/panel", () => ({ ...initialState }));
  new SidePanelsService(client, states);
  client.slots.contribute("layout/side", {
    id: "side-panel",
    component: SidePanel,
  });
  client.slots.contribute("composer/toolbar-left", {
    id: "side-toggle",
    order: 30,
    component: SideToggle,
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
        return { type: "replace", draft: "" };
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

sideClient.inject = [
  "clientActions",
  "clientProjections",
  "composerSurfaces",
  "conversationSurfaces",
  "scopedStores",
  "slots",
];
sideClient.provide = "sidePanels";
export default sideClient;
