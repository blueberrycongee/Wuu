import { useState } from "react";
import type { Thread } from "@wuu/protocol";

import { Avatar } from "../components/Avatar";
import { formatListTimestamp } from "../lib/format";
import type { AppSnapshot } from "../lib/store";
import { isThreadUnread, isVisibleThread, sortThreads, threadDisplayTitle } from "../lib/threads";

export function ChatsScreen({
  snapshot,
  onRefresh,
  onOpenThread,
  onTogglePin,
  onNewThread,
  onUnpair,
}: {
  snapshot: AppSnapshot;
  onRefresh: () => void;
  onOpenThread: (thread: Thread) => void;
  onTogglePin: (thread: Thread) => void;
  /** Creates a conversation in the paired workspace and returns it. */
  onNewThread: () => Promise<Thread>;
  onUnpair: () => void;
}): React.JSX.Element {
  const [creating, setCreating] = useState(false);
  const threads = sortThreads(snapshot.threads.filter(isVisibleThread));
  const phaseLabel =
    snapshot.phase === "attached"
      ? "已连接"
      : snapshot.phase === "connecting"
        ? "连接中…"
        : snapshot.phase === "reconnecting"
          ? "重连中…"
          : "未连接";

  const newThread = async (): Promise<void> => {
    if (creating) return;
    setCreating(true);
    try {
      const thread = await onNewThread();
      onOpenThread(thread);
    } catch (err) {
      window.alert(err instanceof Error ? err.message : String(err));
    } finally {
      setCreating(false);
    }
  };

  return (
    <>
      <div className="header">
        <div className="header-title-wrap">
          <div className="header-title">{snapshot.hostName || "Wuu"}</div>
          <div className="header-subtitle">{phaseLabel}</div>
        </div>
        <button
          className="header-action"
          onClick={() => void newThread()}
          disabled={creating || snapshot.phase !== "attached"}
          title="新建对话"
        >
          {creating ? "…" : "＋ 新建"}
        </button>
        <button className="header-action" onClick={onRefresh}>
          刷新
        </button>
      </div>

      <div className="chats">
        {threads.length === 0 ? (
          <div className="chats-empty">
            还没有会话。
            <br />
            点右上角「＋ 新建」开始一个，或在电脑上新建后它会出现在这里。
          </div>
        ) : (
          threads.map((thread) => (
            <ThreadRow
              key={thread.id}
              thread={thread}
              unread={isThreadUnread(thread, snapshot.lastViewed)}
              onOpen={() => onOpenThread(thread)}
              onTogglePin={() => onTogglePin(thread)}
            />
          ))
        )}

        <div className="chats-footer">
          <button
            className="chats-unpair"
            onClick={() => {
              if (window.confirm("取消配对？本机保存的凭据将被删除。")) onUnpair();
            }}
          >
            取消配对
          </button>
        </div>
      </div>
    </>
  );
}

function ThreadRow({
  thread,
  unread,
  onOpen,
  onTogglePin,
}: {
  thread: Thread;
  unread: boolean;
  onOpen: () => void;
  onTogglePin: () => void;
}): React.JSX.Element {
  const title = threadDisplayTitle(thread);
  const running = thread.status === "in_progress";
  return (
    <div
      className="chat-row"
      role="button"
      tabIndex={0}
      onClick={onOpen}
      onKeyDown={(e) => {
        if (e.key === "Enter") onOpen();
      }}
    >
      <Avatar name={title} />
      <div className="chat-row-main">
        <div className="chat-row-line1">
          <span className="chat-row-title">{title}</span>
          <span className="chat-row-time">{formatListTimestamp(thread.updated_at)}</span>
        </div>
        <div className="chat-row-line2">
          <span className="chat-row-preview">{running ? "正在运行…" : thread.preview}</span>
          {running ? <span className="chat-row-badge running">运行中</span> : null}
          {thread.pinned ? <span className="chat-row-badge pinned">置顶</span> : null}
          {unread ? <span className="chat-row-unread" /> : null}
        </div>
      </div>
      <button
        className={`chat-row-pin${thread.pinned ? " pinned" : ""}`}
        title={thread.pinned ? "取消置顶" : "置顶"}
        onClick={(e) => {
          e.stopPropagation();
          onTogglePin();
        }}
      >
        📌
      </button>
    </div>
  );
}
