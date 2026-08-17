import { useEffect, useRef } from "react";
import type { TouchEvent } from "react";
import type { Thread } from "@wuu/protocol";

import { Composer } from "../components/Composer";
import { IconMenu } from "../components/icons";
import { Markdown } from "../components/Markdown";
import { chatRowsFromTurns } from "../lib/chatModel";
import { greetingFor } from "../lib/greetings";
import type { AppSnapshot } from "../lib/store";
import { isThreadRunning, threadDisplayTitle } from "../lib/threads";

/** Conversation view, ChatGPT-style: the user's messages are right-aligned
 *  bubbles, agent replies render as full-width Markdown documents, and the
 *  composer's action button turns into stop while a turn runs. */
export function ThreadScreen({
  snapshot,
  threadId,
  onSend,
  onInterrupt,
  onOpenDrawer,
}: {
  snapshot: AppSnapshot;
  threadId: string;
  onSend: (thread: Thread, text: string) => void;
  onInterrupt: (threadId: string) => void;
  onOpenDrawer: () => void;
}): React.JSX.Element {
  const thread = snapshot.threads.find((t) => t.id === threadId);
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
    if (endX - startX > 70) onOpenDrawer();
  };

  const title = thread ? threadDisplayTitle(thread) : "会话";

  return (
    <div className="thread" onTouchStart={onTouchStart} onTouchEnd={onTouchEnd}>
      <div className="header">
        <button className="icon-btn" onClick={onOpenDrawer} aria-label="打开会话列表">
          <IconMenu />
        </button>
        <div className="header-title" title={title}>
          {title}
        </div>
        <span className="header-spacer" />
      </div>

      {isEmpty ? (
        <div className="home-body">
          <h2 className="home-greeting">{greetingFor(new Date().getHours())}</h2>
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
                <Markdown text={row.item.text ?? ""} />
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
            <div className="thinking">
              <span>正在思考</span>
              <span className="thinking-dots">
                <i />
                <i />
                <i />
              </span>
            </div>
          ) : null}

          {turnError ? <div className="sys-note error">{turnError}</div> : null}
        </div>
      )}

      <Composer
        placeholder="发消息…"
        running={running}
        onSend={(text) => {
          if (!thread) return;
          nearBottomRef.current = true;
          onSend(thread, text);
        }}
        onStop={() => onInterrupt(threadId)}
      />
    </div>
  );
}
