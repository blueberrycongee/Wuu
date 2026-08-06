import type { Turn } from "../shared/protocol";
import {
  agentHandoffAgentIDs,
  agentHandoffChipDisplayItems,
  isAgentHandoffItem,
  isTerminalSubagentOutcome,
} from "./AgentHandoff";

export type TurnGroup = {
  id: string;
  turns: Turn[];
};

type TurnGroupingFacts = {
  isAgentWake: boolean;
  hasRealUserMessage: boolean;
  hasSpawnAgentCall: boolean;
  spawnedAgentIDs: string[];
  wakeAgentIDs: string[];
};

export function isAgentWakeTurn(turn: Turn): boolean {
  return turnGroupingFacts(turn).isAgentWake;
}

export function turnHasRealUserMessage(turn: Turn): boolean {
  return turnGroupingFacts(turn).hasRealUserMessage;
}

export function turnHasSpawnAgentCall(turn: Turn): boolean {
  return turnGroupingFacts(turn).hasSpawnAgentCall;
}

export function turnsHavePendingSubagents(turns: Turn[]): boolean {
  return subagentProgressForTurns(turns).remaining > 0;
}

export type SubagentTimelineProgress = {
  total: number;
  finished: number;
  remaining: number;
};

export function subagentProgressForTurns(turns: Turn[]): SubagentTimelineProgress {
  const pendingAgentIDs = new Set<string>();
  const settledAgentIDs = new Set<string>();
  let anonymousPending = 0;
  let finished = 0;
  for (const turn of turns) {
    for (const item of turn.items) {
      if (isSpawnAgentItem(item) && spawnResultIsPending(item.result)) {
        const agentID = spawnResultAgentID(item.result);
        if (agentID) {
          pendingAgentIDs.add(agentID);
        } else {
          anonymousPending += 1;
        }
        continue;
      }
      if (item.type === "user_message" && isAgentHandoffItem(item)) {
        for (const chip of agentHandoffChipDisplayItems(item)) {
          if (!isTerminalSubagentOutcome(chip.outcome)) {
            continue;
          }
          if (chip.agentID) {
            if (settledAgentIDs.has(chip.agentID)) {
              continue;
            }
            if (pendingAgentIDs.delete(chip.agentID)) {
              settledAgentIDs.add(chip.agentID);
              finished += 1;
              continue;
            }
            if (anonymousPending > 0) {
              settledAgentIDs.add(chip.agentID);
              anonymousPending -= 1;
              finished += 1;
            }
            continue;
          }
          if (anonymousPending > 0) {
            anonymousPending -= 1;
            finished += 1;
          }
        }
      }
    }
  }
  const remaining = pendingAgentIDs.size + anonymousPending;
  return { total: remaining + finished, finished, remaining };
}

function turnGroupingFacts(turn: Turn): TurnGroupingFacts {
  const spawnedAgentIDs: string[] = [];
  const wakeAgentIDs: string[] = [];
  let sawAgentHandoff = false;
  let hasRealUserMessage = false;
  let hasSpawnAgentCall = false;
  for (const item of turn.items) {
    if (isSpawnAgentItem(item)) {
      hasSpawnAgentCall = true;
      const id = spawnResultAgentID(item.result);
      if (id) {
        spawnedAgentIDs.push(id);
      }
    }
    if (item.type !== "user_message") {
      continue;
    }
    if (!isAgentHandoffItem(item)) {
      hasRealUserMessage = true;
      continue;
    }
    sawAgentHandoff = true;
    wakeAgentIDs.push(...agentHandoffAgentIDs(item));
  }
  const isAgentWake = sawAgentHandoff && !hasRealUserMessage;
  return {
    isAgentWake,
    hasRealUserMessage,
    hasSpawnAgentCall,
    spawnedAgentIDs,
    wakeAgentIDs: isAgentWake ? wakeAgentIDs : [],
  };
}

function isSpawnAgentItem(item: Turn["items"][number]): boolean {
  return (
    (item.type === "tool_call" || item.type === "collab_agent_tool_call") &&
    item.name === "spawn_agent"
  );
}

function spawnResultIsPending(raw: string | undefined): boolean {
  if (!raw) return true;
  try {
    const parsed = JSON.parse(raw) as { status?: unknown };
    if (typeof parsed.status !== "string") return true;
    switch (parsed.status.trim().toLowerCase()) {
      case "completed":
      case "failed":
      case "cancelled":
      case "canceled":
      case "closed":
        return false;
      default:
        return true;
    }
  } catch {
    return true;
  }
}

function spawnResultAgentID(result: string | undefined): string | undefined {
  const trimmed = result?.trim();
  if (!trimmed) {
    return undefined;
  }
  try {
    const parsed: unknown = JSON.parse(trimmed);
    if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
      const id = (parsed as { agent_id?: unknown }).agent_id;
      if (typeof id === "string" && id.trim()) {
        return id.trim();
      }
    }
  } catch {
    // A failed spawn can return plain error text; no agent to attribute.
  }
  return undefined;
}

export function groupConversationTurns(
  turns: Turn[],
  options?: {
    lastGroupOpen?: boolean;
    runningAgentIDs?: readonly string[];
  },
): TurnGroup[] {
  const count = turns.length;
  if (count === 0) return [];

  const runningIDs = new Set(options?.runningAgentIDs ?? []);
  const lastGroupOpen = options?.lastGroupOpen ?? runningIDs.size > 0;
  const facts = new Array<TurnGroupingFacts>(count);
  const firstSpawnIndexByAgentID = new Map<string, number>();
  const lastWakeIndexByAgentID = new Map<string, number>();
  for (let index = 0; index < count; index += 1) {
    const turnFacts = turnGroupingFacts(turns[index]);
    facts[index] = turnFacts;
    for (const id of turnFacts.spawnedAgentIDs) {
      if (!firstSpawnIndexByAgentID.has(id)) {
        firstSpawnIndexByAgentID.set(id, index);
      }
    }
    for (const id of turnFacts.wakeAgentIDs) {
      lastWakeIndexByAgentID.set(id, index);
    }
  }

  const openIntervalDelta = new Int32Array(count + 1);
  for (const [agentID, spawnIndex] of firstSpawnIndexByAgentID) {
    const openFrom = spawnIndex + 1;
    const openUntil = runningIDs.has(agentID)
      ? count
      : (lastWakeIndexByAgentID.get(agentID) ?? 0);
    if (openFrom >= openUntil) {
      continue;
    }
    openIntervalDelta[openFrom] += 1;
    openIntervalDelta[openUntil] -= 1;
  }
  const earlierOrchestrationOpen = new Uint8Array(count);
  let openOrchestrationCount = 0;
  for (let index = 0; index < count; index += 1) {
    openOrchestrationCount += openIntervalDelta[index];
    earlierOrchestrationOpen[index] = openOrchestrationCount > 0 ? 1 : 0;
  }

  const joinsPrevious = new Array<boolean>(count).fill(false);
  for (let index = count - 1; index >= 0; index -= 1) {
    const turnFacts = facts[index];
    if (turnFacts.isAgentWake) {
      joinsPrevious[index] = true;
      continue;
    }
    if (!turnFacts.hasRealUserMessage) {
      continue;
    }
    if (turnFacts.hasSpawnAgentCall) {
      joinsPrevious[index] = earlierOrchestrationOpen[index] === 1;
      continue;
    }
    const nextJoins =
      index + 1 < count ? joinsPrevious[index + 1] : lastGroupOpen;
    joinsPrevious[index] = nextJoins;
  }

  const groups: TurnGroup[] = [];
  for (let index = 0; index < count; index += 1) {
    const turn = turns[index];
    const current = groups[groups.length - 1];
    if (current && joinsPrevious[index]) {
      current.turns.push(turn);
    } else {
      groups.push({ id: turn.id, turns: [turn] });
    }
  }
  return groups;
}
