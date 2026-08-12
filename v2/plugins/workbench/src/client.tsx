import { createElement, useSyncExternalStore, type ReactNode } from "react";
import {
  Service, SlotOutlet, type Context, type Plugin, type SlotHandle,
} from "@wuu-v2/client-runtime";

export type WorkbenchSurface = "conversation" | "search" | "workspace" | "plugins" | "settings";

export class WorkbenchNavigationService extends Service {
  private value: WorkbenchSurface = "conversation";
  private readonly listeners = new Set<() => void>();
  constructor(ctx: Context) { super(ctx, "workbenchNavigation"); ctx.effect(() => () => this.listeners.clear(), "clear workbench navigation"); }
  current(): WorkbenchSurface { return this.value; }
  select(surface: WorkbenchSurface): void { if (this.value === surface) return; this.value = surface; for (const listener of this.listeners) listener(); }
  subscribe(listener: () => void): () => void { this.listeners.add(listener); return () => this.listeners.delete(listener); }
  snapshot = (): WorkbenchSurface => this.value;
}

declare module "cordis" { interface Context { workbenchNavigation: WorkbenchNavigationService } }

function useSurface(client: Context): WorkbenchSurface {
  return useSyncExternalStore(client.workbenchNavigation.subscribe.bind(client.workbenchNavigation), client.workbenchNavigation.snapshot, client.workbenchNavigation.snapshot);
}

function SidebarComposition({ client, sessionId, primary, content, footer }: { client: Context; sessionId?: string; primary: SlotHandle; content: SlotHandle; footer: SlotHandle }) {
  return <div className="workbench-sidebar">
    <SlotOutlet client={client} slot={primary} {...(sessionId ? { sessionId } : {})} />
    <SlotOutlet client={client} slot={content} {...(sessionId ? { sessionId } : {})} />
    <SlotOutlet client={client} slot={footer} {...(sessionId ? { sessionId } : {})} />
  </div>;
}

function MainComposition({ client, sessionId, slot }: { client: Context; sessionId?: string; slot: SlotHandle }) {
  const surface = useSurface(client);
  return <SlotOutlet client={client} slot={slot} businessKey={surface} {...(sessionId ? { sessionId } : {})} />;
}

const labels: Record<Exclude<WorkbenchSurface, "conversation">, string> = { search: "搜索", workspace: "添加工作区", plugins: "插件管理", settings: "设置" };

function ProductEntry({ client, surface, children }: { client: Context; surface: WorkbenchSurface; children: ReactNode }) {
  const active = useSurface(client) === surface;
  return <button type="button" className={`workbench-entry${active ? " is-active" : ""}`} aria-current={active ? "page" : undefined} onClick={() => client.workbenchNavigation.select(surface)}>{children}</button>;
}

function ProductNavigation({ client }: { client: Context }) {
  return <nav className="workbench-navigation" aria-label="Workbench">
    <ProductEntry client={client} surface="search">⌕ <span>{labels.search}</span></ProductEntry>
    <ProductEntry client={client} surface="workspace">▣ <span>{labels.workspace}</span></ProductEntry>
    <ProductEntry client={client} surface="plugins">⋯ <span>{labels.plugins}</span></ProductEntry>
  </nav>;
}

function ProductFooter({ client }: { client: Context }) {
  return <nav className="workbench-navigation workbench-footer" aria-label="Workbench settings">
    <ProductEntry client={client} surface="settings">⚙ <span>{labels.settings}</span></ProductEntry>
  </nav>;
}

function EmptySurface({ ownerProps }: { ownerProps?: unknown }) {
  const surface = ownerProps as Exclude<WorkbenchSurface, "conversation">;
  return <section className="workbench-empty-surface" aria-label={labels[surface]} />;
}

const workbenchClient: Plugin = function workbench(client) {
  new WorkbenchNavigationService(client);
  let sidebarPrimary: SlotHandle; let sidebarContent: SlotHandle; let sidebarFooter: SlotHandle; let main: SlotHandle;
  const sidebar = client.slots.contribute("layout/sidebar", { id: "workbench-sidebar", component: ({ client: c, sessionId }) => <SidebarComposition client={c} {...(sessionId ? { sessionId } : {})} primary={sidebarPrimary} content={sidebarContent} footer={sidebarFooter} />, children: [
    { name: "workbench/sidebar-primary", kind: "list", scope: "session-maybe" },
    { name: "workbench/sidebar-content", kind: "list", scope: "session-maybe" },
    { name: "workbench/sidebar-footer", kind: "list", scope: "session-maybe" },
  ] });
  sidebarPrimary = sidebar.children.get("workbench/sidebar-primary")!;
  sidebarContent = sidebar.children.get("workbench/sidebar-content")!;
  sidebarFooter = sidebar.children.get("workbench/sidebar-footer")!;
  const mainRegistration = client.slots.contribute("layout/conversation", { id: "workbench-main", component: ({ client: c, sessionId }) => <MainComposition client={c} {...(sessionId ? { sessionId } : {})} slot={main} />, children: [{ name: "workbench/main", kind: "chain", scope: "session-maybe" }] });
  main = mainRegistration.children.get("workbench/main")!;
  client.slots.contribute("workbench/sidebar-primary", { id: "product-navigation", component: ProductNavigation });
  client.slots.contribute("workbench/sidebar-footer", { id: "product-footer", component: ProductFooter });
  for (const surface of ["search", "workspace", "plugins", "settings"] as const) {
    client.slots.contribute("workbench/main", { id: `empty-${surface}`, key: surface, component: () => <EmptySurface ownerProps={surface} />, select: (props) => props.businessKey === surface });
  }
  client.effect(() => {
    if (typeof document === "undefined") return () => {};
    const style = document.createElement("style");
    style.dataset.wuuPluginStyle = "workbench";
    style.textContent = `.workbench-sidebar{display:grid;height:100%;min-height:0;grid-template-rows:auto minmax(0,1fr) auto}.workbench-navigation{display:grid;gap:2px;padding:10px}.workbench-entry{display:flex;align-items:center;gap:9px;padding:8px 10px;border:0;border-radius:7px;color:var(--ink);background:transparent;font:inherit;text-align:left;cursor:pointer}.workbench-entry:hover,.workbench-entry:focus-visible,.workbench-entry.is-active{background:var(--surface-3);outline:none}.workbench-empty-surface{width:100%;height:100%;min-height:0}`;
    document.head.append(style);
    return () => style.remove();
  }, "install workbench styles");
};

workbenchClient.provide = "workbenchNavigation";
workbenchClient.inject = ["slots"];
export default workbenchClient;
