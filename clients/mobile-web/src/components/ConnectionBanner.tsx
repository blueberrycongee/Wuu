import type { ConnectionPhase } from "../lib/store";

/** Thin status strip: silent when attached, visible during reconnects or
 *  after a sync failure. */

export function ConnectionBanner({
  phase,
  syncError,
}: {
  phase: ConnectionPhase;
  syncError: string | null;
}): React.JSX.Element | null {
  if (syncError) {
    return <div className="conn-banner error">同步失败：{syncError}</div>;
  }
  if (phase === "reconnecting") {
    return <div className="conn-banner reconnecting">与电脑的连接中断，重连中…</div>;
  }
  if (phase === "connecting") {
    return <div className="conn-banner reconnecting">连接中…</div>;
  }
  return null;
}
