import type { JsonValue, SessionEvent } from "@wuu-v2/contracts";
import { type Context, type Plugin } from "@wuu-v2/kernel";
import type { GoalRecord, GoalValue } from "./shared.js";

const source = { pluginId: "goal", generation: "v1" } as const;

function objectInput(input: JsonValue): Record<string, JsonValue> {
  if (!input || typeof input !== "object" || Array.isArray(input)) {
    throw new Error("goal input must be an object");
  }
  return input;
}

function stringField(input: Record<string, JsonValue>, field: string): string {
  const value = input[field];
  if (typeof value !== "string" || !value.trim()) throw new Error(`missing string field: ${field}`);
  return value.trim();
}

function foldGoal(current: GoalValue | undefined, event: SessionEvent): GoalValue | undefined {
  if (event.record.type === "goal/activated") {
    return {
      objective: (event.record as GoalRecord & { type: "goal/activated" }).data.objective,
      status: "active",
    };
  }
  if (event.record.type === "goal/completed" && current) {
    return { ...current, status: "completed" };
  }
  return current;
}

async function currentGoal(ctx: Context, sessionId: string): Promise<GoalValue | undefined> {
  let value: GoalValue | undefined;
  for (const event of await ctx.sessions.load(sessionId)) value = foldGoal(value, event);
  return value;
}

const goalHost: Plugin = function goal(ctx) {
  ctx.hostActions.register("goal/activate", async (input) => {
    const value = objectInput(input);
    const sessionId = stringField(value, "sessionId");
    if (ctx.agentRuns.isActive(sessionId)) throw new Error("cannot activate a goal during an active run");
    const objective = stringField(value, "objective");
    const event = await ctx.sessions.append(sessionId, source, {
      type: "goal/activated",
      data: { objective },
    } satisfies GoalRecord);
    return { objective, acceptedSeq: event.seq };
  });
  ctx.hostActions.register("goal/complete", async (input) => {
    const value = objectInput(input);
    const sessionId = stringField(value, "sessionId");
    if (ctx.agentRuns.isActive(sessionId)) throw new Error("cannot complete a goal during an active run");
    const current = await currentGoal(ctx, sessionId);
    if (!current || current.status !== "active") throw new Error("session has no active goal");
    const event = await ctx.sessions.append(sessionId, source, {
      type: "goal/completed",
      data: {},
    } satisfies GoalRecord);
    return { acceptedSeq: event.seq };
  });
  ctx.projections.register("goal", (current, event) =>
    foldGoal(current as GoalValue | undefined, event));
  ctx.prompts.register("goal", async (sessionId) => {
    const goal = await currentGoal(ctx, sessionId);
    if (!goal || goal.status !== "active") return undefined;
    return [
      `Active session goal: ${goal.objective}`,
      "Keep work aligned with this goal across turns. Report concrete progress and do not claim completion prematurely.",
    ].join("\n");
  });
};

goalHost.inject = ["agentRuns", "hostActions", "projections", "prompts", "sessions"];
export default goalHost;
