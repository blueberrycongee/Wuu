import type { SessionRecord } from "@wuu-v2/contracts";

export type SubagentStatus =
  | "starting"
  | "running"
  | "completed"
  | "cancelled"
  | "failed"
  | "interrupted";

export interface SubagentValue {
  [key: string]: import("@wuu-v2/contracts").JsonValue;
  id: string;
  childSessionId: string;
  task: string;
  status: SubagentStatus;
  runId: string | null;
}

export type SubagentRecord =
  | SessionRecord<"subagent/created", {
      id: string;
      childSessionId: string;
      parentSeq: number;
      task: string;
    }>
  | SessionRecord<"subagent/run-started", {
      id: string;
      runId: string;
    }>
  | SessionRecord<"subagent/settled", {
      id: string;
      status: Exclude<SubagentStatus, "starting" | "running">;
    }>
  | SessionRecord<"subagent/lineage", {
      id: string;
      parentSessionId: string;
      parentSeq: number;
      task: string;
    }>;
