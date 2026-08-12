import { useCallback, useEffect, useRef, useState, useSyncExternalStore } from "react";
import type { CSSProperties } from "react";
import {
  SlotOutlet,
  useActiveSession,
  type Context,
  type Plugin,
  type SlotHandle,
} from "@wuu-v2/client-runtime";
import { DialogLayerHost, SidebarToggleIcon } from "@wuu-v2/ui-kit";
import { layoutStyles } from "./styles.js";

const layoutClient: Plugin = function layout(client) {
  let conversationSlot: SlotHandle;
  let sideSlot: SlotHandle;
  let sidebarSlot: SlotHandle;
  function AppFrame({ client: componentClient, ownerProps }: { client: Context; ownerProps?: unknown }) {
    const [sidebarOpen, setSidebarOpen] = useState(false);
    const [isNarrow, setIsNarrow] = useState(() => typeof window !== "undefined" && window.innerWidth < 900);
    const [sidebarCollapsed, setSidebarCollapsed] = useState(() => {
      if (typeof window === "undefined") return false;
      return window.innerWidth < 900 || window.localStorage.getItem("wuu.v2.sidebarCollapsed") === "true";
    });
    const [sidebarWidth, setSidebarWidth] = useState(() => {
      if (typeof window === "undefined") return 326;
      const raw = window.localStorage.getItem("wuu.v2.sidebarWidth");
      if (raw === null) return 326;
      const value = Number(raw);
      return Number.isFinite(value) ? Math.min(520, Math.max(240, value)) : 326;
    });
    const desktopCollapsedRef = useRef(sidebarCollapsed);
    const shellRef = useRef<HTMLDivElement>(null);
    const resizeRef = useRef<{ pointerId: number; startX: number; startWidth: number; intent: boolean } | null>(null);
    const rafRef = useRef<number | null>(null);
    const liveWidthRef = useRef(sidebarWidth);
    const writeLiveWidth = useCallback((width: number) => {
      liveWidthRef.current = width;
      if (rafRef.current !== null) cancelAnimationFrame(rafRef.current);
      rafRef.current = requestAnimationFrame(() => {
        shellRef.current?.style.setProperty("--sidebar-width", `${width}px`);
        shellRef.current?.querySelector<HTMLElement>(".app-sidebar-resizer")?.setAttribute("aria-valuenow", String(Math.round(width)));
        rafRef.current = null;
      });
    }, []);
    const finishResize = useCallback(() => {
      const active = resizeRef.current;
      if (!active) return;
      resizeRef.current = null;
      const width = active.intent ? active.startWidth : liveWidthRef.current;
      setSidebarWidth(width);
      setSidebarCollapsed(active.intent);
      desktopCollapsedRef.current = active.intent;
      window.localStorage.setItem("wuu.v2.sidebarWidth", String(width));
      window.localStorage.setItem("wuu.v2.sidebarCollapsed", String(active.intent));
      document.body.classList.remove("is-sidebar-resizing");
    }, []);
    const onResizeMove = useCallback((event: PointerEvent) => {
      const active = resizeRef.current;
      if (!active || event.pointerId !== active.pointerId) return;
      const raw = active.startWidth + event.clientX - active.startX;
      active.intent = raw <= 208;
      writeLiveWidth(Math.min(520, Math.max(240, raw)));
    }, [writeLiveWidth]);
    const onResizeUp = useCallback((event: PointerEvent) => {
      if (resizeRef.current?.pointerId === event.pointerId) finishResize();
    }, [finishResize]);
    useEffect(() => {
      window.addEventListener("pointermove", onResizeMove);
      window.addEventListener("pointerup", onResizeUp);
      window.addEventListener("pointercancel", onResizeUp);
      return () => {
        window.removeEventListener("pointermove", onResizeMove);
        window.removeEventListener("pointerup", onResizeUp);
        window.removeEventListener("pointercancel", onResizeUp);
        if (rafRef.current !== null) cancelAnimationFrame(rafRef.current);
        document.body.classList.remove("is-sidebar-resizing");
      };
    }, [onResizeMove, onResizeUp]);
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
    useEffect(() => {
      const query = window.matchMedia("(max-width: 899px)");
      const update = () => {
        setIsNarrow(query.matches);
        if (query.matches) setSidebarCollapsed(false);
        else {
          setSidebarOpen(false);
          const stored = window.localStorage.getItem("wuu.v2.sidebarCollapsed") === "true";
          desktopCollapsedRef.current = stored;
          setSidebarCollapsed(stored);
        }
      };
      update();
      query.addEventListener("change", update);
      return () => query.removeEventListener("change", update);
    }, []);
    return (
      <DialogLayerHost>
      <div ref={shellRef} className={`app-shell${hasSidebar ? "" : " is-sidebar-empty"}${sidebarOpen ? " is-sidebar-open" : ""}${sidebarCollapsed ? " is-sidebar-collapsed" : ""}`} style={{ "--sidebar-width": `${sidebarWidth}px` } as CSSProperties}>
        {hasSidebar ? (
          <button
            type="button"
            className="app-sidebar-toggle"
            aria-label={isNarrow ? (sidebarOpen ? "Close task history" : "Open task history") : (sidebarCollapsed ? "Open task history" : "Collapse task history")}
            aria-expanded={isNarrow ? sidebarOpen : !sidebarCollapsed}
            onClick={() => {
              if (isNarrow) setSidebarOpen((value) => !value);
              else setSidebarCollapsed((value) => {
                const next = !value;
                desktopCollapsedRef.current = next;
                window.localStorage.setItem("wuu.v2.sidebarCollapsed", String(next));
                return next;
              });
            }}
          >
            <SidebarToggleIcon aria-hidden="true" />
          </button>
        ) : null}
        {hasSidebar ? (
          <aside className="app-sidebar" aria-label="Wuu sidebar">
            <SlotOutlet
              client={componentClient}
              slot={sidebarSlot}
              {...(sessionId ? { sessionId } : {})}
            />
            {!isNarrow && !sidebarCollapsed ? (
              <div
                className="app-sidebar-resizer"
                role="separator"
                aria-label="Resize sidebar"
                aria-orientation="vertical"
                aria-valuemin={240}
                aria-valuemax={520}
                aria-valuenow={Math.round(liveWidthRef.current)}
                tabIndex={0}
                onPointerDown={(event) => {
                  resizeRef.current = { pointerId: event.pointerId, startX: event.clientX, startWidth: sidebarWidth, intent: false };
                  document.body.classList.add("is-sidebar-resizing");
                  (event.currentTarget as HTMLElement).setPointerCapture?.(event.pointerId);
                  event.preventDefault();
                }}
                onKeyDown={(event) => {
                  let next = sidebarWidth;
                  if (event.key === "ArrowLeft") next -= 24;
                  else if (event.key === "ArrowRight") next += 24;
                  else if (event.key === "Home") next = 240;
                  else if (event.key === "End") next = 520;
                  else return;
                  event.preventDefault();
                  next = Math.min(520, Math.max(240, next));
                  setSidebarWidth(next);
                  writeLiveWidth(next);
                  window.localStorage.setItem("wuu.v2.sidebarWidth", String(next));
                }}
              />
            ) : null}
          </aside>
        ) : null}
        <main className="conversation-pane">
          <SlotOutlet
            client={componentClient}
            slot={conversationSlot}
            {...(sessionId ? { sessionId } : {})}
          />
        </main>
        <aside className="app-work-surface" aria-label="Auxiliary work surface">
          <SlotOutlet
            client={componentClient}
            slot={sideSlot}
            {...(sessionId ? { sessionId } : {})}
          />
        </aside>
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
