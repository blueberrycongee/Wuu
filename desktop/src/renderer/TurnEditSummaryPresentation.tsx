import { useEffect, useState } from "react";
import type { Turn } from "../shared/protocol";
import { turnIsAnswerReady } from "./AppState";
import { motionDurationMs } from "./motion";
import { TurnEditSummaryCard, turnHasFileEdits } from "./TurnEditSummaryCard";
import type { TurnFileDiffSelection } from "./TurnFileDiffTypes";

/** The timeline card is a temporary review surface, not the edit record. */
export function TurnEditSummaryPresentation({
  turn,
  isLatestTurn,
  cwd,
  onOpenFile,
  onOpenFileDiff,
  onCollapseComplete,
}: {
  turn: Turn;
  isLatestTurn: boolean;
  cwd?: string;
  onOpenFile?: (path: string) => void;
  onOpenFileDiff?: (selection: TurnFileDiffSelection) => void;
  onCollapseComplete?: () => void;
}): JSX.Element | null {
  const visible = isLatestTurn &&
    (turn.status !== "in_progress" || turnIsAnswerReady(turn)) &&
    turnHasFileEdits(turn);
  const [retained, setRetained] = useState(visible);

  useEffect(() => {
    if (visible) {
      setRetained(true);
      return;
    }
    if (!retained) return;
    const finish = (): void => {
      setRetained(false);
      onCollapseComplete?.();
    };
    if (window.matchMedia?.("(prefers-reduced-motion: reduce)").matches) {
      finish();
      return;
    }
    // Also finish in hidden panes, where transitionend may never fire.
    const timer = window.setTimeout(finish, motionDurationMs("--query-submit-duration", 220));
    return () => window.clearTimeout(timer);
  }, [visible, retained, onCollapseComplete]);

  if (!visible && !retained) return null;
  return (
    <div
      className={`turn-edit-presentation${visible ? "" : " is-exiting"}`}
      inert={!visible}
      aria-hidden={!visible || undefined}
    >
      <div className="turn-edit-presentation-body">
        <TurnEditSummaryCard
          turn={turn}
          cwd={cwd}
          onOpenFile={onOpenFile}
          onOpenFileDiff={onOpenFileDiff}
          compact={turn.status === "failed" || turn.status === "interrupted"}
        />
      </div>
    </div>
  );
}
