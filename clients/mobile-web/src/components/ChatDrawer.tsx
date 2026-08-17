// Left slide-out drawer, ChatGPT-style: "新对话" on top, the conversation
// list in the middle, host + account actions pinned at the bottom. Rendered
// once by the App shell; both the home and thread screens open it.

import type { Thread } from "@wuu/protocol";

import { formatListTimestamp } from "../lib/format";
import type { AppSnapshot } from "../lib/store";
import { isThreadUnread, isThreadRunning, isVisibleThread, sortThreads, threadDisplayTitle } from "../lib/threads";
import { IconPin, IconPlus, IconRefresh } from "./icons";

export function ChatDrawer({
  open,
  snapshot,
  onClose,
  onNewChat,
  onOpenThread,
  onTogglePin,
  onRefresh,
  onUnpair,
}: {
  open: boolean;
  snapshot: AppSnapshot;
  onClose: () => void;
  onNewChat: () => void;
  onOpenThread: (thread: Thread) => void;
  onTogglePin: (thread: Thread) => void;
  onRefresh: () => void;
  onUnpair: () => void;
}): React.JSX.Element {
  const threads = sortThreads(snapshot.threads.filter(isVisibleThread));

  return (
    <>
      <div className={`scrim${open ? " open" : ""}`} onClick={onClose} />
      <aside className={`drawer${open ? " open" : ""}`} aria-hidden={!open}>
        <div className="drawer-top">
          <button className="drawer-new" onClick={onNewChat}>
            <IconPlus size={18} />
            <span>新对话</span>
          </button>
        </div>

        <div className="drawer-list">
          {threads.length === 0 ? (
            <div className="drawer-empty">还没有会话</div>
          ) : (
            threads.map((thread) => (
              <DrawerRow
                key={thread.id}
                thread={thread}
                active={thread.id === snapshot.activeThreadId}
                unread={isThreadUnread(thread, snapshot.lastViewed)}
                onOpen={() => onOpenThread(thread)}
                onTogglePin={() => onTogglePin(thread)}
              />
            ))
          )}
        </div>

        <div className="drawer-foot">
          <span className="drawer-host" title={snapshot.hostName}>
            {snapshot.hostName || "Wuu"}
          </span>
          <button className="icon-btn" onClick={onRefresh} aria-label="刷新会话列表" title="刷新">
            <IconRefresh />
          </button>
          <button
            className="drawer-unpair"
            onClick={() => {
              if (window.confirm("取消配对？本机保存的凭据将被删除。")) onUnpair();
            }}
          >
            取消配对
          </button>
        </div>
      </aside>
    </>
  );
}

function DrawerRow({
  thread,
  active,
  unread,
  onOpen,
  onTogglePin,
}: {
  thread: Thread;
  active: boolean;
  unread: boolean;
  onOpen: () => void;
  onTogglePin: () => void;
}): React.JSX.Element {
  const title = threadDisplayTitle(thread);
  const running = isThreadRunning(thread);
  return (
    <div
      className={`chat-row${active ? " active" : ""}`}
      role="button"
      tabIndex={0}
      onClick={onOpen}
      onKeyDown={(e) => {
        if (e.key === "Enter") onOpen();
      }}
    >
      <div className="chat-row-main">
        <div className="chat-row-line1">
          {unread ? <span className="chat-row-unread" /> : null}
          <span className="chat-row-title">{title}</span>
          <span className="chat-row-time">{formatListTimestamp(thread.updated_at)}</span>
        </div>
        <div className="chat-row-line2">
          <span className="chat-row-preview">{running ? "正在运行…" : thread.preview}</span>
          <button
            className={`chat-row-pin${thread.pinned ? " pinned" : ""}`}
            title={thread.pinned ? "取消置顶" : "置顶"}
            aria-label={thread.pinned ? "取消置顶" : "置顶"}
            onClick={(e) => {
              e.stopPropagation();
              onTogglePin();
            }}
          >
            <IconPin size={14} />
          </button>
        </div>
      </div>
    </div>
  );
}
