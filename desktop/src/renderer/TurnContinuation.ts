import type { Thread, Turn } from "../shared/protocol";

export function isContinuationTurn(turn: Turn | undefined): boolean {
  return turn?.kind === "continuation";
}

export function canResumeInterruptedTurn(thread: Thread | undefined): boolean {
  if (!thread || thread.orchestration_interrupted) {
    return false;
  }
  const latest = thread.turns?.[thread.turns.length - 1];
  return latest?.status === "interrupted" && !isContinuationTurn(latest);
}
