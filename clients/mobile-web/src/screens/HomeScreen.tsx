import { useState } from "react";
import type { ReactNode } from "react";

import mascotFace from "../assets/mascot-face.png";
import { MascotAvatar } from "../components/MascotAvatar";
import { greetingFor } from "../lib/greetings";
import type { AppSnapshot, WorkspaceInfo } from "../lib/store";

/** Post-pairing home: pick a workspace up top and type directly below.
 *  Sending creates a new conversation in that workspace. History stays one
 *  tap away in the left drawer. */
export function HomeScreen({
  snapshot,
  onCompose,
  onSelectWorkspace,
  drawerContent,
}: {
  snapshot: AppSnapshot;
  onCompose: (text: string) => Promise<void>;
  onSelectWorkspace: (workspace: WorkspaceInfo) => void;
  drawerContent: ReactNode;
}): React.JSX.Element {
  const [draft, setDraft] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);

  const phaseLabel =
    snapshot.phase === "attached"
      ? "已连接"
      : snapshot.phase === "connecting"
        ? "连接中…"
        : snapshot.phase === "reconnecting"
          ? "重连中…"
          : "未连接";

  const workspaces = snapshot.workspaces;
  const activeWorkspacePath = snapshot.activeWorkspacePath || snapshot.workdir;
  const activeWorkspace = workspaces.find((w) => w.path === activeWorkspacePath);

  const send = async (): Promise<void> => {
    const text = draft.trim();
    if (!text || busy) return;
    setBusy(true);
    setError(null);
    try {
      await onCompose(text);
      setDraft("");
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="home">
      <div className="header">
        <button
          className="header-back"
          onClick={() => setDrawerOpen(true)}
          aria-label="打开会话列表"
        >
          ☰
        </button>
        <MascotAvatar name="wuu" size={32} />
        <div className="header-title-wrap">
          <div className="header-title">{snapshot.hostName || "Wuu"}</div>
          <div className="header-subtitle">{phaseLabel}</div>
        </div>
      </div>

      <div className="workspace-bar">
        <label className="workspace-label" htmlFor="workspace-select">
          工作区
        </label>
        <select
          id="workspace-select"
          className="workspace-select"
          value={activeWorkspacePath}
          onChange={(e) => {
            const workspace = workspaces.find((w) => w.path === e.target.value);
            if (workspace) onSelectWorkspace(workspace);
          }}
        >
          {workspaces.length === 0 ? (
            <option value={activeWorkspacePath}>
              {activeWorkspace?.name ??
                (activeWorkspacePath
                  ? activeWorkspacePath.split("/").filter(Boolean).pop() ?? activeWorkspacePath
                  : "当前工作区")}
            </option>
          ) : (
            workspaces.map((workspace) => (
              <option key={workspace.path} value={workspace.path}>
                {workspace.name || workspace.path.split("/").filter(Boolean).pop()}
              </option>
            ))
          )}
        </select>
      </div>

      <div className="hero home-hero">
        <span className="hero-mascot">
          <img src={mascotFace} alt="" draggable={false} />
        </span>
        <h2>{greetingFor(new Date().getHours())}</h2>
        <p>在下方输入消息，开始一段新对话</p>
      </div>

      {error ? <div className="pair-error home-error">{error}</div> : null}

      <div className="composer">
        <textarea
          value={draft}
          placeholder="发消息…"
          rows={1}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey && !e.nativeEvent.isComposing) {
              e.preventDefault();
              void send();
            }
          }}
        />
        <button
          className="composer-send"
          disabled={!draft.trim() || busy || snapshot.phase !== "attached"}
          onClick={() => void send()}
        >
          {busy ? "发送中…" : "发送"}
        </button>
      </div>

      <div
        className={`drawer-overlay${drawerOpen ? " open" : ""}`}
        onClick={() => setDrawerOpen(false)}
      />
      <aside className={`drawer-panel${drawerOpen ? " open" : ""}`}>
        {drawerContent}
      </aside>
    </div>
  );
}
