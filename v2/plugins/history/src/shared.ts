import type { JsonValue, SessionRecord } from "@wuu-v2/contracts";

export type HistoryRecord = SessionRecord<"history/session-created",
  | { version: 1 }
  | { version: 2; workspaceId: string }
>;

export interface HistoryEntry {
  [key: string]: JsonValue;
  id: string;
  title: string;
  updatedAt: string;
  running: boolean;
}

export interface HistoryEntryProjection {
  [key: string]: JsonValue;
  title: string;
  updatedAt: string;
  running: boolean;
  /** Durable fold state used to keep concurrent runs readable in the sidebar. */
  runningRunIds: string[];
  hasPrompt: boolean;
}
