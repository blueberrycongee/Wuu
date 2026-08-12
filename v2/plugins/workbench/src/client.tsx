import { createElement, useSyncExternalStore, type ReactNode } from "react";
import {
  Service, SlotOutlet, type Context, type Plugin, type SlotHandle,
} from "@wuu-v2/client-runtime";
import { AddWorkspaceIcon, SearchIcon, PluginsIcon, SettingsIcon } from "@wuu-v2/ui-kit";

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

function SidebarComposition({ client, sessionId, brand, primary, content, footer }: { client: Context; sessionId?: string; brand: SlotHandle; primary: SlotHandle; content: SlotHandle; footer: SlotHandle }) {
  return <div className="workbench-sidebar">
    <div className="workbench-brand-region"><SlotOutlet client={client} slot={brand} {...(sessionId ? { sessionId } : {})} /></div>
    <div className="workbench-primary-region"><SlotOutlet client={client} slot={primary} {...(sessionId ? { sessionId } : {})} /></div>
    <div className="workbench-content-region"><SlotOutlet client={client} slot={content} {...(sessionId ? { sessionId } : {})} /></div>
    <div className="workbench-footer-region"><SlotOutlet client={client} slot={footer} {...(sessionId ? { sessionId } : {})} /></div>
  </div>;
}

function MainComposition({ client, sessionId, slot }: { client: Context; sessionId?: string; slot: SlotHandle }) {
  const surface = useSurface(client);
  return <SlotOutlet client={client} slot={slot} businessKey={surface} {...(sessionId ? { sessionId } : {})} />;
}

const labels: Record<Exclude<WorkbenchSurface, "conversation">, string> = { search: "搜索", workspace: "添加工作区", plugins: "插件管理", settings: "设置" };

export function WorkbenchSidebarItem({
  children,
  active = false,
  disabled = false,
  onActivate,
}: {
  children: ReactNode;
  active?: boolean;
  disabled?: boolean;
  onActivate: () => void;
}) {
  return <button type="button" className={`workbench-entry${active ? " is-active" : ""}`} aria-current={active ? "page" : undefined} disabled={disabled} onClick={onActivate}>{children}</button>;
}

function ProductEntry({ client, surface, children }: { client: Context; surface: WorkbenchSurface; children: ReactNode }) {
  const active = useSurface(client) === surface;
  return <WorkbenchSidebarItem active={active} onActivate={() => client.workbenchNavigation.select(surface)}>{children}</WorkbenchSidebarItem>;
}

function ProductNavigation({ client }: { client: Context }) {
  return <nav className="workbench-navigation" aria-label="Workbench">
    <ProductEntry client={client} surface="search"><SearchIcon aria-hidden="true" /> <span>{labels.search}</span></ProductEntry>
    <ProductEntry client={client} surface="plugins"><PluginsIcon aria-hidden="true" /> <span>{labels.plugins}</span></ProductEntry>
  </nav>;
}

function ProductFooter({ client }: { client: Context }) {
  return <nav className="workbench-navigation workbench-footer" aria-label="Workbench settings">
    <ProductEntry client={client} surface="settings"><SettingsIcon aria-hidden="true" /> <span>{labels.settings}</span></ProductEntry>
  </nav>;
}

function WorkspaceSection({ client }: { client: Context }) {
  return <section className="workbench-section" aria-label="工作区"><header><span>工作区</span><button type="button" aria-label="添加工作区" title="添加工作区" onClick={() => client.workbenchNavigation.select("workspace")}><AddWorkspaceIcon aria-hidden="true" /></button></header></section>;
}

function EmptySurface({ ownerProps }: { ownerProps?: unknown }) {
  const surface = ownerProps as Exclude<WorkbenchSurface, "conversation">;
  return <section className="workbench-empty-surface" aria-label={labels[surface]} />;
}

const workbenchClient: Plugin = function workbench(client) {
  new WorkbenchNavigationService(client);
  let sidebarBrand: SlotHandle; let sidebarPrimary: SlotHandle; let sidebarContent: SlotHandle; let sidebarFooter: SlotHandle; let main: SlotHandle;
  const sidebar = client.slots.contribute("layout/sidebar", { id: "workbench-sidebar", component: ({ client: c, sessionId }) => <SidebarComposition client={c} {...(sessionId ? { sessionId } : {})} brand={sidebarBrand} primary={sidebarPrimary} content={sidebarContent} footer={sidebarFooter} />, children: [
    { name: "workbench/sidebar-brand", kind: "list", scope: "session-maybe" },
    { name: "workbench/sidebar-primary", kind: "list", scope: "session-maybe" },
    { name: "workbench/sidebar-content", kind: "list", scope: "session-maybe" },
    { name: "workbench/sidebar-footer", kind: "list", scope: "session-maybe" },
  ] });
  sidebarBrand = sidebar.children.get("workbench/sidebar-brand")!;
  sidebarPrimary = sidebar.children.get("workbench/sidebar-primary")!;
  sidebarContent = sidebar.children.get("workbench/sidebar-content")!;
  sidebarFooter = sidebar.children.get("workbench/sidebar-footer")!;
  const mainRegistration = client.slots.contribute("layout/conversation", { id: "workbench-main", component: ({ client: c, sessionId }) => <MainComposition client={c} {...(sessionId ? { sessionId } : {})} slot={main} />, children: [{ name: "workbench/main", kind: "chain", scope: "session-maybe" }] });
  main = mainRegistration.children.get("workbench/main")!;
  client.slots.contribute("workbench/sidebar-primary", { id: "product-navigation", component: ProductNavigation });
  client.slots.contribute("workbench/sidebar-brand", { id: "product-brand", component: () => <div className="workbench-brand" aria-label="wuu">wuu</div> });
  client.slots.contribute("workbench/sidebar-content", { id: "workspace-section", order: -100, component: WorkspaceSection });
  client.slots.contribute("workbench/sidebar-footer", { id: "product-footer", component: ProductFooter });
  for (const surface of ["search", "workspace", "plugins", "settings"] as const) {
    client.slots.contribute("workbench/main", { id: `empty-${surface}`, key: surface, component: () => <EmptySurface ownerProps={surface} />, select: (props) => props.businessKey === surface });
  }
  client.effect(() => {
    if (typeof document === "undefined") return () => {};
    const style = document.createElement("style");
    style.dataset.wuuPluginStyle = "workbench";
    style.textContent = `.workbench-sidebar{display:grid;height:100%;min-height:0;grid-template-rows:auto auto minmax(0,1fr) auto}.workbench-brand-region,.workbench-footer-region{min-width:0}.workbench-primary-region{display:grid;min-width:0;gap:2px;padding:6px 10px}.workbench-primary-region>.workbench-navigation{padding:0}.workbench-content-region{display:flex;min-width:0;min-height:0;flex-direction:column;overflow:hidden}.workbench-brand{padding:18px 16px 10px;font-family:Georgia,serif;font-size:23px;font-weight:650;letter-spacing:.02em;color:var(--wuu-accent,#b64a32)}.workbench-navigation{display:grid;gap:2px;padding:6px 10px}.workbench-entry{display:flex;align-items:center;gap:10px;padding:8px 10px;border:0;border-radius:7px;color:var(--ink);background:transparent;font:inherit;text-align:left;cursor:pointer}.workbench-entry svg{width:18px;height:18px;flex:0 0 auto}.workbench-entry:hover,.workbench-entry:focus-visible,.workbench-entry.is-active{background:var(--surface-3);outline:none}.workbench-entry:disabled{cursor:default;opacity:.55}.workbench-section{flex:0 0 auto;padding:16px 10px 0}.workbench-section header{display:flex;height:28px;align-items:center;justify-content:space-between;padding:0 10px;color:var(--ink-muted);font-size:11px;font-weight:650;letter-spacing:.04em;text-transform:uppercase}.workbench-section button{display:grid;width:24px;height:24px;place-items:center;border:0;border-radius:6px;color:inherit;background:transparent;cursor:pointer}.workbench-section button:hover,.workbench-section button:focus-visible{background:rgba(31,35,40,.08);outline:none}.workbench-footer{padding-bottom:14px}.workbench-empty-surface{width:100%;height:100%;min-height:0}`;
    document.head.append(style);
    return () => style.remove();
  }, "install workbench styles");
};

workbenchClient.provide = "workbenchNavigation";
workbenchClient.inject = ["slots"];
export default workbenchClient;
