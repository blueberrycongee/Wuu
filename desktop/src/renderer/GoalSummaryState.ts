import type { ComposerGoalSummary } from "../shared/protocol";

export function goalSummaryForActiveThread(
  summary: ComposerGoalSummary | null,
  activeThreadID: string | undefined,
): ComposerGoalSummary | null {
  if (!summary || !activeThreadID || summary.thread_id !== activeThreadID) {
    return null;
  }
  return summary;
}

export function requireGoalMutationSuccess(
  result: { ok: boolean },
  action: string,
): void {
  if (!result.ok) {
    throw new Error(`Goal ${action} was not applied`);
  }
}
