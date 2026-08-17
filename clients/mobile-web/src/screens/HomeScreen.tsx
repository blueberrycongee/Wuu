import { useState } from "react";

import { Composer } from "../components/Composer";
import { IconCheck, IconChevronDown, IconMenu } from "../components/icons";
import { greetingFor } from "../lib/greetings";
import type { AppSnapshot, WorkspaceInfo } from "../lib/store";

/** Post-pairing home, ChatGPT-style: a menu button and the active workspace
 *  up top, a one-line greeting in the middle, and the composer at the
 *  bottom. Sending creates a conversation in the chosen workspace; history
 *  lives in the left drawer. */
export function HomeScreen({
  snapshot,
  onCompose,
  onSelectWorkspace,
  onOpenDrawer,
}: {
  snapshot: AppSnapshot;
  onCompose: (text: string) => Promise<void>;
  onSelectWorkspace: (workspace: WorkspaceInfo) => void;
  onOpenDrawer: () => void;
}): React.JSX.Element {
  const [error, setError] = useState<string | null>(null);
  const [sheetOpen, setSheetOpen] = useState(false);

  const workspaces = snapshot.workspaces;
  const activeWorkspacePath = snapshot.activeWorkspacePath || snapshot.workdir;
  const activeName = workspaceName(
    workspaces.find((w) => w.path === activeWorkspacePath),
    activeWorkspacePath,
  );

  const compose = async (text: string): Promise<void> => {
    setError(null);
    try {
      await onCompose(text);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      throw err; // Composer keeps the draft on rejection.
    }
  };

  return (
    <div className="home">
      <div className="header">
        <button className="icon-btn" onClick={onOpenDrawer} aria-label="打开会话列表">
          <IconMenu />
        </button>
        <button
          className="ws-chip"
          onClick={() => setSheetOpen(true)}
          aria-label="选择工作区"
          title={activeWorkspacePath}
        >
          <span className="ws-chip-name">{activeName}</span>
          <IconChevronDown />
        </button>
      </div>

      <div className="home-body">
        <h2 className="home-greeting">{greetingFor(new Date().getHours())}</h2>
      </div>

      {error ? <div className="pair-error home-error">{error}</div> : null}

      <Composer
        placeholder="开始新对话…"
        disabled={snapshot.phase !== "attached"}
        onSend={compose}
      />

      <div className={`scrim${sheetOpen ? " open" : ""}`} onClick={() => setSheetOpen(false)} />
      <div className={`sheet${sheetOpen ? " open" : ""}`} role="dialog" aria-label="选择工作区">
        <div className="sheet-title">工作区</div>
        <div className="sheet-hint">新对话将在所选工作区中创建</div>
        <div className="sheet-list">
          {(workspaces.length > 0
            ? workspaces
            : [{ path: activeWorkspacePath, name: activeName }]
          ).map((workspace) => {
            const active = workspace.path === activeWorkspacePath;
            return (
              <button
                key={workspace.path || "current"}
                className={`sheet-row${active ? " active" : ""}`}
                onClick={() => {
                  setSheetOpen(false);
                  if (!active && workspaces.length > 0) onSelectWorkspace(workspace);
                }}
              >
                <span className="sheet-row-main">
                  <span className="sheet-row-name">{workspaceName(workspace, workspace.path)}</span>
                  <span className="sheet-row-path">{workspace.path}</span>
                </span>
                {active ? <IconCheck /> : null}
              </button>
            );
          })}
        </div>
      </div>
    </div>
  );
}

function workspaceName(workspace: WorkspaceInfo | undefined, path: string): string {
  if (workspace?.name) return workspace.name;
  const leaf = path.split("/").filter(Boolean).pop();
  return leaf || "当前工作区";
}
