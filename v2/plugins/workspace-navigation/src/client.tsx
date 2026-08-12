import { useState } from "react";
import { SlotOutlet, type Context, type Plugin, type SlotHandle } from "@wuu-v2/client-runtime";
import { AddWorkspaceIcon, Dialog } from "@wuu-v2/ui-kit";

const conversationWorkspace = { workspaceId: "conversation", kind: "conversation" } as const;

function AddWorkspaceDialog({ client, onClose }: { client: Context; onClose: () => void }) {
  const [path, setPath] = useState<string>();
  const [choosing, setChoosing] = useState(false);
  const [error, setError] = useState<string>();
  const choose = async () => {
    setChoosing(true);
    setError(undefined);
    try {
      const selected = await client.shellCapabilities.chooseWorkspaceDirectory();
      if (selected) setPath(selected);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setChoosing(false);
    }
  };
  return (
    <Dialog
      title="添加工作区"
      subtitle="选择一个 Wuu 可以读取和编辑的文件夹。"
      icon={<AddWorkspaceIcon aria-hidden="true" />}
      onClose={onClose}
      className="workspace-add-dialog"
      footer={<>
        <button type="button" className="workspace-dialog-secondary" onClick={onClose}>取消</button>
        <button type="button" className="workspace-dialog-primary" disabled>添加工作区</button>
      </>}
    >
      <button type="button" className="workspace-directory-picker" disabled={choosing} onClick={() => void choose()}>
        <AddWorkspaceIcon aria-hidden="true" />
        <span>{path ?? (choosing ? "正在打开文件夹选择器…" : "选择文件夹")}</span>
      </button>
      {path ? <p className="workspace-dialog-notice">目录已经选好；多工作区持久化接通后即可添加。</p> : null}
      {error ? <p className="workspace-dialog-error" role="alert">{error}</p> : null}
    </Dialog>
  );
}

function Workspace({
  client,
  actions,
  sessions,
}: {
  client: Context;
  actions: SlotHandle;
  sessions: SlotHandle;
}) {
  const [adding, setAdding] = useState(false);
  return (
    <section className="workbench-section workspace-navigation-section" aria-label="工作区">
      <header>
        <span>工作区</span>
        <button type="button" aria-label="添加工作区" title="添加工作区" onClick={() => setAdding(true)}>
          <AddWorkspaceIcon aria-hidden="true" />
        </button>
      </header>
      <div className="workspace-group">
        <div className="workspace-row">
          <button type="button" className="workspace-select" onClick={() => client.workbenchNavigation.select("conversation")}>
            <span className="workspace-folder" aria-hidden="true" />
            <span>对话</span>
          </button>
          <SlotOutlet client={client} slot={actions} ownerProps={conversationWorkspace} />
        </div>
        <div className="workspace-session-slot"><SlotOutlet client={client} slot={sessions} ownerProps={conversationWorkspace} /></div>
      </div>
      {adding ? <AddWorkspaceDialog client={client} onClose={() => setAdding(false)} /> : null}
    </section>
  );
}

const plugin: Plugin = function workspaceNavigation(client) {
  let actions: SlotHandle;
  let sessions: SlotHandle;
  const registration = client.slots.contribute("workbench/sidebar-content", {
    id: "workspace-navigation",
    component: ({ client: componentClient }) => <Workspace client={componentClient} actions={actions} sessions={sessions} />,
    children: [
      { name: "workspace-navigation/workspace-actions", kind: "list", scope: "session-maybe" },
      { name: "workspace-navigation/workspace-sessions", kind: "list", scope: "session-maybe" },
    ],
  });
  actions = registration.children.get("workspace-navigation/workspace-actions")!;
  sessions = registration.children.get("workspace-navigation/workspace-sessions")!;
  client.effect(() => {
    if (typeof document === "undefined") return () => {};
    const style = document.createElement("style");
    style.dataset.wuuPluginStyle = "workspace-navigation";
    style.textContent = `.workspace-group{display:grid;gap:2px}.workspace-row{display:flex;align-items:center;width:calc(100% - var(--sidebar-pad,10px) * 2);min-width:0;margin:0 var(--sidebar-pad,10px);border-radius:7px}.workspace-row:hover,.workspace-row:focus-within{background:var(--surface-3)}.workspace-select{display:flex;min-width:0;min-height:32px;flex:1 1 auto;align-items:center;gap:var(--sidebar-label-gap,10px);padding:7px var(--sidebar-pad,10px);border:0;color:var(--ink);background:transparent;font:inherit;text-align:left;cursor:pointer}.workspace-select:focus-visible{outline:none}.workspace-folder{width:var(--sidebar-icon-col,18px);height:var(--sidebar-icon-col,18px);flex:0 0 var(--sidebar-icon-col,18px)}.workspace-row>.history-workspace-create{flex:0 0 auto}.workspace-session-slot{min-height:0;padding-left:0}.workspace-session-slot .history-list{padding-top:2px}.workspace-session-slot .history-list button{width:calc(100% - var(--sidebar-pad,10px) * 2);margin-inline:var(--sidebar-pad,10px);padding-left:calc(var(--sidebar-label-axis,48px) - var(--sidebar-pad,10px))}.workspace-add-dialog{--wuu-dialog-max-width:600px}.workspace-directory-picker{display:grid;min-height:132px;place-items:center;align-content:center;gap:10px;padding:20px;border:1px solid var(--hairline-strong);border-radius:12px;color:var(--ink);background:var(--surface-1,transparent);font:inherit;cursor:pointer}.workspace-directory-picker:hover:not(:disabled),.workspace-directory-picker:focus-visible{background:var(--surface-2);outline:none}.workspace-directory-picker svg{color:var(--ink-muted)}.workspace-directory-picker span{max-width:100%;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.workspace-dialog-primary,.workspace-dialog-secondary{min-height:38px;padding:0 16px;border:0;border-radius:9px;font:inherit}.workspace-dialog-primary{color:white;background:var(--ink)}.workspace-dialog-primary:disabled{opacity:.38}.workspace-dialog-secondary{color:var(--ink-muted);background:transparent}.workspace-dialog-notice,.workspace-dialog-error{margin:0;color:var(--ink-muted);font-size:12px}.workspace-dialog-error{color:var(--danger,#b42318)}`;
    document.head.append(style);
    return () => style.remove();
  }, "install workspace navigation styles");
};

plugin.inject = ["shellCapabilities", "slots", "workbenchNavigation"];
export default plugin;
