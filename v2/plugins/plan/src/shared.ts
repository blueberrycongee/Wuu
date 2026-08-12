import type { SessionRecord } from "@wuu-v2/contracts";

export type PlanStepStatus = "pending" | "in_progress" | "completed";

export type PlanStep = {
  id: string;
  text: string;
  status: PlanStepStatus;
};

export type PlanValue = {
  explanation?: string;
  steps: PlanStep[];
};

export type PlanUpdatedRecord = SessionRecord<"plan/updated", PlanValue>;
export type PlanActivatedRecord = SessionRecord<"plan/activated", Record<string, never>>;
