import type { SessionRecord } from "@wuu-v2/contracts";

export type GoalValue = {
  objective: string;
  status: "active" | "completed";
};

export type GoalRecord =
  | SessionRecord<"goal/activated", { objective: string }>
  | SessionRecord<"goal/completed", Record<string, never>>;
