import { createElement, useSyncExternalStore, type ReactNode } from "react";
import {
  Service, SlotOutlet, type Context, type Plugin, type SlotHandle,
} from "@wuu-v2/client-runtime";
import { PluginsIcon, SearchIcon, SettingsIcon } from "@wuu-v2/ui-kit";

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

export const workbenchSurfaceLabels: Record<Exclude<WorkbenchSurface, "conversation">, string> = { search: "搜索", workspace: "工作区", plugins: "插件管理", settings: "设置" };

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

function Brand() {
  return <div className="workbench-brand" aria-label="wuu harness"><span>wuu</span><small>harness</small></div>;
}

function SurfaceEntry({ client, surface, children }: { client: Context; surface: WorkbenchSurface; children: ReactNode }) {
  const active = useSurface(client) === surface;
  return <WorkbenchSidebarItem active={active} onActivate={() => client.workbenchNavigation.select(surface)}>{children}</WorkbenchSidebarItem>;
}

function Primary({ client }: { client: Context }) {
  return <nav className="workbench-navigation" aria-label="Workbench">
    <SurfaceEntry client={client} surface="search"><SearchIcon aria-hidden="true" /><span>搜索</span></SurfaceEntry>
    <SurfaceEntry client={client} surface="plugins"><PluginsIcon aria-hidden="true" /><span>插件管理</span></SurfaceEntry>
  </nav>;
}

function Footer({ client }: { client: Context }) {
  return <nav className="workbench-navigation workbench-footer" aria-label="设置">
    <SurfaceEntry client={client} surface="settings"><SettingsIcon aria-hidden="true" /><span>设置</span></SurfaceEntry>
  </nav>;
}

function EmptySurface({ ownerProps }: { ownerProps?: unknown }) {
  const surface = ownerProps as Exclude<WorkbenchSurface, "conversation">;
  return <section className="workbench-empty-surface" aria-label={workbenchSurfaceLabels[surface]} />;
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
  client.slots.contribute("workbench/sidebar-brand", { id: "workbench-brand", component: Brand });
  client.slots.contribute("workbench/sidebar-primary", { id: "workbench-primary", component: Primary });
  client.slots.contribute("workbench/sidebar-footer", { id: "workbench-footer", component: Footer });
  const mainRegistration = client.slots.contribute("layout/conversation", { id: "workbench-main", component: ({ client: c, sessionId }) => <MainComposition client={c} {...(sessionId ? { sessionId } : {})} slot={main} />, children: [{ name: "workbench/main", kind: "chain", scope: "session-maybe" }] });
  main = mainRegistration.children.get("workbench/main")!;
  for (const surface of ["search", "workspace", "plugins", "settings"] as const) {
    client.slots.contribute("workbench/main", { id: `empty-${surface}`, key: surface, component: () => <EmptySurface ownerProps={surface} />, select: (props) => props.businessKey === surface });
  }
  client.effect(() => {
    if (typeof document === "undefined") return () => {};
    const style = document.createElement("style");
    style.dataset.wuuPluginStyle = "workbench";
    style.textContent = `.workbench-sidebar{--sidebar-rhythm-row:4px;--sidebar-rhythm-heading:8px;--sidebar-rhythm-group:24px;--sidebar-rhythm-footer:16px;--sidebar-pad:10px;--sidebar-icon-col:18px;--sidebar-label-gap:10px;--sidebar-label-axis:calc(var(--sidebar-pad) * 2 + var(--sidebar-icon-col) + var(--sidebar-label-gap));display:grid;height:100%;min-height:0;grid-template-rows:auto auto minmax(0,1fr) auto}.workbench-brand-region,.workbench-footer-region{min-width:0}.workbench-primary-region{display:grid;min-width:0;gap:var(--sidebar-rhythm-row);padding:4px var(--sidebar-pad) 0}.workbench-primary-region>.workbench-navigation{padding:0}.workbench-content-region{display:flex;min-width:0;min-height:0;flex-direction:column;overflow:hidden}.workbench-brand{display:flex;align-items:baseline;gap:7px;padding:8px 16px 12px;transform:translateY(-2px);font-family:Georgia,serif;font-size:23px;font-weight:650;letter-spacing:.02em;color:var(--wuu-accent,#b64a32)}.workbench-brand small{font-family:inherit;font-size:12px;font-weight:500;letter-spacing:.03em;color:var(--ink-muted)}.workbench-navigation{display:grid;gap:var(--sidebar-rhythm-row);padding:0 var(--sidebar-pad)}.workbench-entry{display:flex;align-items:center;gap:var(--sidebar-label-gap);padding:8px var(--sidebar-pad);border:0;border-radius:7px;color:var(--ink);background:transparent;font:inherit;text-align:left;cursor:pointer}.workbench-entry svg{width:var(--sidebar-icon-col);height:var(--sidebar-icon-col);flex:0 0 var(--sidebar-icon-col)}.workbench-entry:hover,.workbench-entry:focus-visible,.workbench-entry.is-active{background:var(--surface-3);outline:none}.workbench-entry:disabled{cursor:default;opacity:.55}.workbench-section{flex:0 0 auto;padding:var(--sidebar-rhythm-group) 0 0}.workbench-section header{display:flex;height:28px;align-items:center;justify-content:space-between;padding:0 calc(var(--sidebar-pad) * 2) 8px var(--sidebar-label-axis);color:var(--ink-muted);font-size:11px;font-weight:650;letter-spacing:.04em;text-transform:uppercase}.workbench-section button{display:grid;width:24px;height:24px;place-items:center;border:0;border-radius:6px;color:inherit;background:transparent;cursor:pointer}.workbench-section button:hover,.workbench-section button:focus-visible{background:rgba(31,35,40,.08);outline:none}.workbench-footer{padding:var(--sidebar-rhythm-footer) 0 14px}.workbench-empty-surface{width:100%;height:100%;min-height:0}`;
    document.head.append(style);
    return () => style.remove();
  }, "install workbench styles");
};

workbenchClient.provide = "workbenchNavigation";
workbenchClient.inject = ["slots"];
export default workbenchClient;
