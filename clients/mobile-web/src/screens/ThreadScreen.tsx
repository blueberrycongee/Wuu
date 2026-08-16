import { useEffect, useRef, useState } from "react";
import type { ReactNode, TouchEvent } from "react";
import type { Thread } from "@wuu/protocol";

import mascotFace from "../assets/mascot-face.png";
import { MascotAvatar } from "../components/MascotAvatar";
import { chatRowsFromTurns } from "../lib/chatModel";
import { greetingFor } from "../lib/greetings";
import type { AppSnapshot } from "../lib/store";
import { isThreadRunning, threadDisplayTitle } from "../lib/threads";

export function ThreadScreen({
  snapshot,
  threadId,
  onSend,
  onInterrupt,
  drawerContent,
}: {
  snapshot: AppSnapshot;
  threadId: string;
  onSend: (thread: Thread, text: string) => void;
  onInterrupt: (threadId: string) => void;
  /** Conversation list rendered inside the left slide-out drawer. */
  drawerContent: ReactNode;
}): React.JSX.Element {
  const thread = snapshot.threads.find((t) => t.id === threadId);
  const [draft, setDraft] = useState("");
  const [drawerOpen, setDrawerOpen] = useState(false);
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const nearBottomRef = useRef(true);
  const touchStartRef = useRef<number | null>(null);

  const rows = thread ? chatRowsFromTurns(thread.turns) : [];
  const pending = snapshot.pending.filter((p) => p.threadId === threadId);
  const running = thread ? isThreadRunning(thread) : false;
  const lastTurn = thread?.turns[thread.turns.length - 1];
  const turnError =
    lastTurn && lastTurn.status === "failed"
      ? (lastTurn.error?.message ?? "本轮出错")
      : null;
  const isEmpty = !thread || (rows.length === 0 && pending.length === 0);

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

  // Edge swipe from the left opens the conversation drawer, mirroring the
  // desktop's always-visible session sidebar on a one-pane phone.
  const onTouchStart = (e: TouchEvent): void => {
    if (e.touches.length === 1 && e.touches[0].clientX < 28) {
      touchStartRef.current = e.touches[0].clientX;
    } else {
      touchStartRef.current = null;
    }
  };
  const onTouchEnd = (e: TouchEvent): void => {
    const startX = touchStartRef.current;
    touchStartRef.current = null;
    if (startX === null) return;
    const endX = e.changedTouches[0]?.clientX ?? startX;
    if (endX - startX > 70) setDrawerOpen(true);
  };

  const title = thread ? threadDisplayTitle(thread) : "会话";
  const hostName = snapshot.hostName || "Wuu";

  return (
    <div className="thread" onTouchStart={onTouchStart} onTouchEnd={onTouchEnd}>
      <div className="header">
        <button
          className="header-back"
          onClick={() => setDrawerOpen(true)}
          aria-label="打开会话列表"
        >
          ☰
        </button>
        <MascotAvatar id={threadId} name={title} size={32} />
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

      {isEmpty ? (
        <div className="hero">
          <span className="hero-mascot">
            <img src={mascotFace} alt="" draggable={false} />
          </span>
          <h2>{greetingFor(new Date().getHours())}</h2>
          <p>在下方输入消息，开始这段对话</p>
        </div>
      ) : (
        <div className="messages" ref={scrollRef} onScroll={trackScroll}>
          {rows.map((row) =>
            row.kind === "user" ? (
              <div key={row.id} className="msg user">
                <div className="msg-bubble">{row.item.text ?? ""}</div>
              </div>
            ) : (
              <div key={row.id} className="msg agent">
                <span className="hero-mascot" style={{ width: 30, height: 30, marginBottom: 0 }}>
                  <img src={mascotFace} alt="" draggable={false} />
                </span>
                <div className="msg-bubble">{row.item.text ?? ""}</div>
              </div>
            ),
          )}

          {pending.map((p) => (
            <div key={p.clientId} className="msg user">
              <div>
                <div className="msg-bubble pending">{p.text}</div>
                <div className="msg-meta">{p.queued ? "排队中…" : "发送中…"}</div>
              </div>
            </div>
          ))}

          {running && !turnError ? (
            <div className="msg agent">
              <span className="hero-mascot" style={{ width: 30, height: 30, marginBottom: 0 }}>
                <img src={mascotFace} alt="" draggable={false} />
              </span>
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
      )}

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

      <div
        className={`drawer-overlay${drawerOpen ? " open" : ""}`}
        onClick={() => setDrawerOpen(false)}
      />
      <aside className={`drawer-panel${drawerOpen ? " open" : ""}`}>{drawerContent}</aside>
    </div>
  );
}
