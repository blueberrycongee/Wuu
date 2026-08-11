import type { JsonValue, SessionRecord } from "@wuu-v2/contracts";

export type HistoryRecord = SessionRecord<"history/session-created", {
  version: 1;
}>;

export interface HistoryEntry {
  [key: string]: JsonValue;
  id: string;
  title: string;
  updatedAt: string;
  running: boolean;
}
