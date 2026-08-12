import { useCallback, useEffect, useRef, useState } from "react";
import {
  useProjection,
  useActiveSession,
  type Context,
  type Plugin,
} from "@wuu-v2/client-runtime";
import type { JsonValue } from "@wuu-v2/contracts";
import type { HistoryEntry, HistoryEntryProjection } from "./shared.js";
import { historyStyles } from "./styles.js";

function objectValue(value: JsonValue | undefined): Record<string, JsonValue> {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    throw new Error("History returned an invalid response");
  }
  return value;
}

function parseEntries(value: JsonValue | undefined): HistoryEntry[] {
  const sessions = objectValue(value).sessions;
  if (!Array.isArray(sessions)) throw new Error("History response omitted sessions");
  return sessions.map((candidate) => {
    const entry = objectValue(candidate);
    if (
      typeof entry.id !== "string" ||
      typeof entry.title !== "string" ||
      typeof entry.updatedAt !== "string" ||
      typeof entry.running !== "boolean"
    ) throw new Error("History returned an invalid Session entry");
    return {
      id: entry.id,
      title: entry.title,
      updatedAt: entry.updatedAt,
      running: entry.running,
    };
  });
}

function HistoryRow({
  client,
  entry,
  active,
  onKeyDown,
  rowRef,
}: {
  client: Context;
  entry: HistoryEntry;
  active: boolean;
  onKeyDown: (event: React.KeyboardEvent<HTMLButtonElement>) => void;
  rowRef: (element: HTMLButtonElement | null) => void;
}) {
  const live = useProjection<HistoryEntryProjection>(client, entry.id, "history/entry");
  const value = live ?? entry;
  return (
    <button
      type="button"
      className={active ? "is-active" : undefined}
      aria-current={active ? "page" : undefined}
      aria-label={`${value.title}${value.running ? " (running)" : ""}`}
      title={entry.id}
      ref={rowRef}
      onKeyDown={onKeyDown}
      onClick={() => client.activeSession.select(entry.id)}
    >
      <span>{value.title}</span>
      {value.running ? <i aria-hidden="true" /> : null}
    </button>
  );
}

function HistorySidebar({ client }: { client: Context }) {
  const activeSessionId = useActiveSession(client);
  const [entries, setEntries] = useState<HistoryEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string>();
  const rowRefs = useRef(new Map<string, HTMLButtonElement>());

  const refresh = useCallback(async () => {
    setError(undefined);
    try {
      setEntries(parseEntries(await client.clientActions.execute("history/list", {})));
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setLoading(false);
    }
  }, [client]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const create = async () => {
    if (creating) return;
    setCreating(true);
    setError(undefined);
    try {
      const result = objectValue(await client.clientActions.execute("history/create", {}));
      if (typeof result.sessionId !== "string") throw new Error("History did not create a Session");
      client.activeSession.select(result.sessionId);
      await refresh();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setCreating(false);
    }
  };

  const moveSelection = (index: number) => {
    const entry = entries[index];
    if (!entry) return;
    client.activeSession.select(entry.id);
    rowRefs.current.get(entry.id)?.focus();
  };

  const handleRowKeyDown = (index: number) => (event: React.KeyboardEvent<HTMLButtonElement>) => {
    if (!entries.length) return;
    let next: number | undefined;
    if (event.key === "ArrowDown") next = Math.min(index + 1, entries.length - 1);
    if (event.key === "ArrowUp") next = Math.max(index - 1, 0);
    if (event.key === "Home") next = 0;
    if (event.key === "End") next = entries.length - 1;
    if (next === undefined) return;
    event.preventDefault();
    moveSelection(next);
  };

  return (
    <div className="history-sidebar">
      <header className="history-header">
        <strong>Wuu</strong>
        <button type="button" aria-label="New task" disabled={creating} onClick={() => void create()}>
          {creating ? "…" : "+"}
        </button>
      </header>
      <nav className="history-list" aria-label="Tasks" aria-busy={loading}>
        {entries.map((entry, index) => (
          <HistoryRow
            key={entry.id}
            client={client}
            entry={entry}
            active={entry.id === activeSessionId}
            onKeyDown={handleRowKeyDown(index)}
            rowRef={(element) => {
              if (element) rowRefs.current.set(entry.id, element);
              else rowRefs.current.delete(entry.id);
            }}
          />
        ))}
        {loading ? <p role="status">Loading tasks…</p> : null}
        {!loading && !entries.length ? <p role="status">No tasks yet</p> : null}
      </nav>
      {error ? (
        <div className="history-error" role="alert">
          <span>{error}</span>
          <button type="button" onClick={() => void refresh()}>Retry</button>
        </div>
      ) : null}
    </div>
  );
}

const historyClient: Plugin = function history(client) {
  client.slots.contribute("layout/sidebar", {
    id: "history",
    component: HistorySidebar,
  });
  client.effect(() => {
    if (typeof document === "undefined") return () => {};
    const style = document.createElement("style");
    style.dataset.wuuPluginStyle = "history";
    style.textContent = historyStyles;
    document.head.append(style);
    return () => style.remove();
  }, "install History styles");
};

historyClient.inject = ["activeSession", "clientActions", "clientProjections", "slots"];
export default historyClient;
