import type { DefaultProfileProvider } from "./host.js";

function object(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    throw new Error("WUU_V2_MODELS entries must be objects");
  }
  return value as Record<string, unknown>;
}

function required(value: Record<string, unknown>, key: string): string {
  const field = value[key];
  if (typeof field !== "string" || !field) {
    throw new Error(`WUU_V2_MODELS entry requires ${key}`);
  }
  return field;
}

export function providersFromEnvironment(
  environment: NodeJS.ProcessEnv,
  allowScripted = false,
): DefaultProfileProvider[] {
  if (environment.WUU_V2_MODELS) {
    const parsed: unknown = JSON.parse(environment.WUU_V2_MODELS);
    if (!Array.isArray(parsed) || !parsed.length || parsed.length > 16) {
      throw new Error("WUU_V2_MODELS must contain between 1 and 16 models");
    }
    const ids = new Set<string>();
    return parsed.map((candidate): DefaultProfileProvider => {
      const value = object(candidate);
      const id = required(value, "id");
      if (ids.has(id)) throw new Error(`duplicate WUU_V2_MODELS id: ${id}`);
      ids.add(id);
      const baseUrl = value.baseUrl;
      if (baseUrl !== undefined && (typeof baseUrl !== "string" || !baseUrl)) {
        throw new Error(`WUU_V2_MODELS ${id} has an invalid baseUrl`);
      }
      return {
        kind: "openai",
        config: {
          id,
          model: required(value, "model"),
          apiKey: required(value, "apiKey"),
          ...(baseUrl === undefined ? {} : { baseUrl }),
        },
      };
    });
  }
  if (allowScripted && environment.WUU_V2_PROVIDER === "scripted") {
    return [{
      kind: "scripted",
      config: {
        rounds: [
          {
            toolCalls: [{
              type: "tool_call",
              callId: "desktop-smoke-read",
              name: "read",
              input: { path: "package.json" },
            }],
          },
          { text: "Desktop Harness smoke completed." },
        ],
      },
    }];
  }
  const apiKey = environment.WUU_V2_OPENAI_API_KEY;
  const model = environment.WUU_V2_MODEL;
  if (!apiKey || !model) {
    throw new Error("Configure WUU_V2_MODELS or WUU_V2_OPENAI_API_KEY and WUU_V2_MODEL");
  }
  return [{
    kind: "openai",
    config: {
      apiKey,
      model,
      ...(environment.WUU_V2_OPENAI_BASE_URL
        ? { baseUrl: environment.WUU_V2_OPENAI_BASE_URL }
        : {}),
    },
  }];
}
