import { useCallback, useEffect, useState } from "react";
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
}: {
  client: Context;
  entry: HistoryEntry;
  active: boolean;
}) {
  const live = useProjection<HistoryEntryProjection>(client, entry.id, "history/entry");
  const value = live ?? entry;
  return (
    <button
      type="button"
      className={active ? "is-active" : undefined}
      aria-current={active ? "page" : undefined}
      title={entry.id}
      onClick={() => client.activeSession.select(entry.id)}
    >
      <span>{value.title}</span>
      {value.running ? <i aria-label="Running" /> : null}
    </button>
  );
}

function HistorySidebar({ client }: { client: Context }) {
  const activeSessionId = useActiveSession(client);
  const [entries, setEntries] = useState<HistoryEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string>();

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

  return (
    <div className="history-sidebar">
      <header className="history-header">
        <strong>Wuu</strong>
        <button type="button" aria-label="New task" disabled={creating} onClick={() => void create()}>
          {creating ? "…" : "+"}
        </button>
      </header>
      <nav className="history-list" aria-label="Tasks">
        {entries.map((entry) => (
          <HistoryRow
            key={entry.id}
            client={client}
            entry={entry}
            active={entry.id === activeSessionId}
          />
        ))}
        {loading ? <p>Loading tasks…</p> : null}
        {!loading && !entries.length ? <p>No tasks yet</p> : null}
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
