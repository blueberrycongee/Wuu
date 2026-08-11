import type { EventSource, JsonValue, SessionRecord } from "@wuu-v2/contracts";
import {
  Service,
  type Context,
  type Plugin,
  type ToolPolicyService,
} from "@wuu-v2/kernel";

export type PermissionMode = "read-only" | "workspace-write" | "full-access";
export interface PermissionSessionConfig {
  defaultMode: PermissionMode;
}

type PermissionSelectedRecord = SessionRecord<"permission/selected", { mode: PermissionMode }>;
const source: EventSource = { pluginId: "permission-session", generation: "v1" };
const modes: PermissionMode[] = ["read-only", "workspace-write", "full-access"];

function objectInput(input: JsonValue): Record<string, JsonValue> {
  if (!input || typeof input !== "object" || Array.isArray(input)) {
    throw new Error("action input must be an object");
  }
  return input;
}

function stringField(input: Record<string, JsonValue>, field: string): string {
  const value = input[field];
  if (typeof value !== "string" || !value) throw new Error(`missing string field: ${field}`);
  return value;
}

function permissionMode(value: string): PermissionMode {
  if (value !== "read-only" && value !== "workspace-write" && value !== "full-access") {
    throw new Error(`unknown permission mode: ${value}`);
  }
  return value;
}

export class PermissionSessionService extends Service implements ToolPolicyService {
  constructor(ctx: Context, private readonly defaultMode: PermissionMode) {
    super(ctx, "toolPolicy");
    ctx.projections.register("permission", (current, event) => {
      if (event.record.type !== "permission/selected") return current;
      const record = event.record as PermissionSelectedRecord;
      return this.projection(record.data.mode);
    }, () => this.projection(defaultMode));
    ctx.hostActions.register("permission/select", async (input) => {
      const value = objectInput(input);
      const sessionId = stringField(value, "sessionId");
      const mode = permissionMode(stringField(value, "mode"));
      if (ctx.agentRuns.isActive(sessionId)) {
        throw new Error("cannot change permission during an active run");
      }
      const event = await ctx.sessions.append(sessionId, source, {
        type: "permission/selected",
        data: { mode },
      } satisfies PermissionSelectedRecord);
      return { mode, acceptedSeq: event.seq };
    });
  }

  private projection(mode: PermissionMode): JsonValue {
    return {
      selected: mode,
      options: [
        { id: "read-only", label: "Read only" },
        { id: "workspace-write", label: "Workspace write" },
        { id: "full-access", label: "Full access" },
      ],
    };
  }

  async resolve(sessionId: string): Promise<PermissionMode> {
    let selected = this.defaultMode;
    for (const event of await this.ctx.sessions.load(sessionId)) {
      if (event.record.type === "permission/selected") {
        selected = permissionMode((event.record as PermissionSelectedRecord).data.mode);
      }
    }
    return selected;
  }

  async allowedTools(sessionId: string, available: readonly string[]): Promise<ReadonlySet<string>> {
    const mode = await this.resolve(sessionId);
    if (mode === "full-access") return new Set(available);
    const allowed = mode === "read-only" ? new Set(["read"]) : new Set(["read", "write"]);
    return new Set(available.filter((name) => {
      const tool = this.ctx.tools.get(name);
      return tool ? allowed.has(tool.access) : false;
    }));
  }

  async initialize(sessionId: string, preset: string): Promise<void> {
    const mode = permissionMode(preset);
    const events = await this.ctx.sessions.load(sessionId);
    if (events.some((event) => event.record.type === "permission/selected")) return;
    await this.ctx.sessions.append(sessionId, source, {
      type: "permission/selected",
      data: { mode },
    } satisfies PermissionSelectedRecord);
  }
}

const permissionSessionHost: Plugin<PermissionSessionConfig> = function permissionSession(ctx, config) {
  if (!modes.includes(config.defaultMode)) throw new Error(`unknown default permission: ${config.defaultMode}`);
  new PermissionSessionService(ctx, config.defaultMode);
};

permissionSessionHost.inject = ["agentRuns", "hostActions", "projections", "sessions", "tools"];
permissionSessionHost.provide = "toolPolicy";
export default permissionSessionHost;
