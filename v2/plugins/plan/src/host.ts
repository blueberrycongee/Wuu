import type { JsonValue, ToolDefinition } from "@wuu-v2/contracts";
import { type Context, type Plugin } from "@wuu-v2/kernel";
import type {
  PlanActivatedRecord,
  PlanStep,
  PlanStepStatus,
  PlanUpdatedRecord,
  PlanValue,
} from "./shared.js";

const source = { pluginId: "plan", generation: "v1" } as const;
const statuses = new Set<PlanStepStatus>(["pending", "in_progress", "completed"]);

function objectValue(value: JsonValue): Record<string, JsonValue> {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    throw new Error("plan input must be an object");
  }
  return value;
}

function planValue(input: JsonValue): PlanValue {
  const value = objectValue(input);
  if (!Array.isArray(value.steps) || !value.steps.length) {
    throw new Error("plan requires at least one step");
  }
  let active = 0;
  const ids = new Set<string>();
  const steps: PlanStep[] = value.steps.map((candidate, index) => {
    const step = objectValue(candidate);
    const id = typeof step.id === "string" && step.id.trim()
      ? step.id.trim()
      : `step-${index + 1}`;
    const text = typeof step.text === "string" ? step.text.trim() : "";
    const status = step.status;
    if (!text) throw new Error(`plan step ${id} requires text`);
    if (typeof status !== "string" || !statuses.has(status as PlanStepStatus)) {
      throw new Error(`plan step ${id} has an invalid status`);
    }
    if (ids.has(id)) throw new Error(`duplicate plan step id: ${id}`);
    ids.add(id);
    if (status === "in_progress") active += 1;
    return { id, text, status: status as PlanStepStatus };
  });
  if (active > 1) throw new Error("plan can have at most one in-progress step");
  const explanation = typeof value.explanation === "string"
    ? value.explanation.trim()
    : "";
  return { ...(explanation ? { explanation } : {}), steps };
}

async function planActive(ctx: Context, sessionId: string): Promise<boolean> {
  return (await ctx.sessions.load(sessionId))
    .some((event) => event.record.type === "plan/activated");
}

function planTool(ctx: Context): ToolDefinition {
  return {
    name: "update_plan",
    description: "Publish the current task plan. Keep it short and update step statuses as work progresses.",
    access: "write",
    available: (sessionId) => planActive(ctx, sessionId),
    inputSchema: {
      type: "object",
      properties: {
        explanation: { type: "string" },
        steps: {
          type: "array",
          minItems: 1,
          items: {
            type: "object",
            properties: {
              id: { type: "string" },
              text: { type: "string" },
              status: { type: "string", enum: ["pending", "in_progress", "completed"] },
            },
            required: ["text", "status"],
            additionalProperties: false,
          },
        },
      },
      required: ["steps"],
      additionalProperties: false,
    },
    async execute(input, execution) {
      execution.signal.throwIfAborted();
      if (!await planActive(ctx, execution.sessionId)) {
        throw new Error("Plan is not active in this Session");
      }
      const data = planValue(input);
      execution.signal.throwIfAborted();
      const event = await ctx.sessions.append(execution.sessionId, source, {
        type: "plan/updated",
        data,
      } satisfies PlanUpdatedRecord);
      return {
        content: [{ type: "text", text: `Plan updated at sequence ${event.seq}.` }],
      };
    },
  };
}

const planHost: Plugin = function plan(ctx) {
  ctx.hostActions.register("plan/activate", async (input) => {
    const value = objectValue(input);
    const sessionId = typeof value.sessionId === "string" ? value.sessionId : "";
    if (!sessionId) throw new Error("plan activation requires sessionId");
    if (ctx.agentRuns.isActive(sessionId)) throw new Error("cannot activate Plan during an active run");
    if (await planActive(ctx, sessionId)) return { active: true };
    const event = await ctx.sessions.append(sessionId, source, {
      type: "plan/activated",
      data: {},
    } satisfies PlanActivatedRecord);
    return { active: true, acceptedSeq: event.seq };
  });
  ctx.tools.register("update_plan", planTool(ctx));
  ctx.prompts.register("plan", async (sessionId) =>
    await planActive(ctx, sessionId)
      ? "Use update_plan only when a task benefits from a visible multi-step plan; do not create a plan for trivial work."
      : undefined);
  ctx.projections.register("plan", (current, event) => {
    if (event.record.type !== "plan/updated") return current;
    return (event.record as PlanUpdatedRecord).data;
  });
};

planHost.inject = ["agentRuns", "hostActions", "projections", "prompts", "sessions", "tools"];
export default planHost;
