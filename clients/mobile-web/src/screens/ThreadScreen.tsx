import { useEffect, useRef, useState } from "react";
import type { Thread } from "@wuu/protocol";

import { Avatar } from "../components/Avatar";
import { chatRowsFromTurns } from "../lib/chatModel";
import type { AppSnapshot } from "../lib/store";
import { isThreadRunning, threadDisplayTitle } from "../lib/threads";

export function ThreadScreen({
  snapshot,
  threadId,
  onBack,
  onSend,
  onInterrupt,
}: {
  snapshot: AppSnapshot;
  threadId: string;
  onBack: () => void;
  onSend: (thread: Thread, text: string) => void;
  onInterrupt: (threadId: string) => void;
}): React.JSX.Element {
  const thread = snapshot.threads.find((t) => t.id === threadId);
  const [draft, setDraft] = useState("");
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const nearBottomRef = useRef(true);

  const rows = thread ? chatRowsFromTurns(thread.turns) : [];
  const pending = snapshot.pending.filter((p) => p.threadId === threadId);
  const running = thread ? isThreadRunning(thread) : false;
  const lastTurn = thread?.turns[thread.turns.length - 1];
  const turnError =
    lastTurn && lastTurn.status === "failed"
      ? (lastTurn.error?.message ?? "本轮出错")
      : null;

  // Keep the view pinned to the newest message while the user is near the
  // bottom; respect a deliberate scroll-up to read history.
  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    if (nearBottomRef.current) {
      el.scrollTop = el.scrollHeight;
    }
  }, [rows.length, pending.length, running]);

  const trackScroll = (): void => {
    const el = scrollRef.current;
    if (!el) return;
    nearBottomRef.current = el.scrollHeight - el.scrollTop - el.clientHeight < 120;
  };

  const send = (): void => {
    if (!thread) return;
    const text = draft.trim();
    if (!text) return;
    setDraft("");
    nearBottomRef.current = true;
    onSend(thread, text);
  };

  const title = thread ? threadDisplayTitle(thread) : "会话";
  const hostName = snapshot.hostName || "Wuu";

  return (
    <div className="thread">
      <div className="header">
        <button className="header-back" onClick={onBack} aria-label="返回">
          ‹
        </button>
        <Avatar name={title} small round />
        <div className="header-title-wrap">
          <div className="header-title">{title}</div>
          <div className="header-subtitle">{hostName}</div>
        </div>
        {running ? (
          <button className="header-action danger" onClick={() => onInterrupt(threadId)}>
            打断
          </button>
        ) : null}
      </div>

      <div className="messages" ref={scrollRef} onScroll={trackScroll}>
        {!thread ? (
          <div className="sys-note">会话加载中…</div>
        ) : rows.length === 0 && pending.length === 0 ? (
          <div className="sys-note">还没有消息，说点什么开始吧</div>
        ) : null}

        {rows.map((row) =>
          row.kind === "user" ? (
            <div key={row.id} className="msg user">
              <div className="msg-bubble">{row.item.text ?? ""}</div>
            </div>
          ) : (
            <div key={row.id} className="msg agent">
              <Avatar name={hostName} small round />
              <div className="msg-bubble">{row.item.text ?? ""}</div>
            </div>
          ),
        )}

        {pending.map((p) => (
          <div key={p.clientId} className="msg user">
            <div>
              <div className="msg-bubble pending">{p.text}</div>
              <div className="msg-meta pending">{p.queued ? "排队中…" : "发送中…"}</div>
            </div>
          </div>
        ))}

        {running && !turnError ? (
          <div className="msg agent">
            <Avatar name={hostName} small round />
            <div className="msg-bubble thinking">
              <span>正在思考</span>
              <span className="thinking-dots">
                <i />
                <i />
                <i />
              </span>
            </div>
          </div>
        ) : null}

        {turnError ? <div className="sys-note error">{turnError}</div> : null}
      </div>

      <div className="composer">
        <textarea
          value={draft}
          placeholder="发消息…"
          rows={1}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey && !e.nativeEvent.isComposing) {
              e.preventDefault();
              send();
            }
          }}
        />
        <button className="composer-send" disabled={!draft.trim() || !thread} onClick={send}>
          发送
        </button>
      </div>
    </div>
  );
}
