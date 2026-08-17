// ChatGPT-style composer: one floating pill holding an auto-growing textarea
// and a single circular action button — send (↑) normally, stop (■) while a
// turn is running. Enter sends, Shift+Enter inserts a newline, IME
// composition never triggers a send.

import { useEffect, useRef, useState } from "react";

import { IconArrowUp, IconStop } from "./icons";

const MAX_HEIGHT = 132;

export function Composer({
  placeholder,
  disabled = false,
  running = false,
  onSend,
  onStop,
}: {
  placeholder: string;
  /** External gate, e.g. connection not attached yet. */
  disabled?: boolean;
  /** A turn is running: the action button becomes stop. */
  running?: boolean;
  /** May return a promise; a rejection keeps the draft so nothing is lost. */
  onSend: (text: string) => void | Promise<void>;
  onStop?: () => void;
}): React.JSX.Element {
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);
  const areaRef = useRef<HTMLTextAreaElement | null>(null);

  // Auto-grow the textarea up to MAX_HEIGHT; shrink back when text is cut.
  useEffect(() => {
    const el = areaRef.current;
    if (!el) return;
    el.style.height = "0px";
    el.style.height = `${Math.min(el.scrollHeight, MAX_HEIGHT)}px`;
  }, [draft]);

  const send = async (): Promise<void> => {
    const text = draft.trim();
    if (!text || sending || disabled) return;
    setSending(true);
    try {
      await onSend(text);
      setDraft("");
    } catch {
      // Parent surfaces the error; the draft stays put.
    } finally {
      setSending(false);
    }
  };

  const canSend = draft.trim().length > 0 && !disabled && !sending;

  return (
    <div className="composer">
      <div className="composer-pill">
        <textarea
          ref={areaRef}
          value={draft}
          placeholder={placeholder}
          rows={1}
          autoCapitalize="sentences"
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey && !e.nativeEvent.isComposing) {
              e.preventDefault();
              void send();
            }
          }}
        />
        {running ? (
          <button
            className="composer-action stop"
            onClick={onStop}
            aria-label="停止当前运行"
            title="停止"
          >
            <IconStop />
          </button>
        ) : (
          <button
            className="composer-action send"
            disabled={!canSend}
            onClick={() => void send()}
            aria-label="发送"
            title="发送"
          >
            <IconArrowUp />
          </button>
        )}
      </div>
    </div>
  );
}
