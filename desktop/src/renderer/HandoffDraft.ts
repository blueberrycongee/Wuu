export type HandoffTargetStatus = "pending" | "resolved" | "unverified" | "unavailable";

export type HandoffModelCandidate = {
  providerId: string;
  providerLabel: string;
  modelId: string;
  modelLabel: string;
  catalogued: boolean;
};

export type HandoffDraft = {
  raw: string;
  revision: number;
  providerId: string;
  modelId: string;
  intent: string;
  status: HandoffTargetStatus;
  targetLabel: string;
  candidates: HandoffModelCandidate[];
};

export type HandoffCatalog = {
  providers: Array<{
    id: string;
    label: string;
    models: Array<{ id: string; label: string }>;
    allowCustomModel?: boolean;
  }>;
};

const MODEL_TOKEN = "model";

export function emptyHandoffDraft(revision = 1): HandoffDraft {
  return {
    raw: "",
    revision,
    providerId: "",
    modelId: "",
    intent: "",
    status: "pending",
    targetLabel: "",
    candidates: [],
  };
}

export function parseHandoffArgs(raw: string): { providerPrefix: string; modelPrefix: string; intent: string; complete: boolean } {
  const text = raw.replace(/^\s+/, "");
  if (!text) {
    return { providerPrefix: "", modelPrefix: "", intent: "", complete: false };
  }
  const remainder = text.toLowerCase().startsWith(`${MODEL_TOKEN} `) || text.toLowerCase() === MODEL_TOKEN
    ? text.slice(MODEL_TOKEN.length).replace(/^\s+/, "")
    : text;
  const [targetPart, ...intentParts] = splitOnce(remainder, " -- ");
  const intent = intentParts.join(" -- ").trim();
  const { token, rest, complete } = readQuotedOrBare(targetPart);
  const slash = token.indexOf("/");
  if (slash < 0) {
    return { providerPrefix: token, modelPrefix: "", intent, complete: complete && token.includes("/") };
  }
  return {
    providerPrefix: token.slice(0, slash),
    modelPrefix: token.slice(slash + 1),
    intent,
    complete: complete && rest.trim() === "",
  };
}

export function reduceHandoffDraft(raw: string, catalog: HandoffCatalog, revision: number): HandoffDraft {
  const parsed = parseHandoffArgs(raw);
  const providers = catalog.providers ?? [];
  const matchingProviders = providers.filter((provider) =>
    provider.id.toLowerCase().startsWith(parsed.providerPrefix.toLowerCase())
    || provider.label.toLowerCase().includes(parsed.providerPrefix.toLowerCase()),
  );
  const provider = matchingProviders.find((item) => item.id.toLowerCase() === parsed.providerPrefix.toLowerCase())
    ?? (matchingProviders.length === 1 ? matchingProviders[0] : undefined);
  const candidates: HandoffModelCandidate[] = [];
  for (const item of provider ? [provider] : matchingProviders) {
    for (const model of item.models) {
      if (parsed.modelPrefix && !model.id.toLowerCase().startsWith(parsed.modelPrefix.toLowerCase()) && !model.label.toLowerCase().includes(parsed.modelPrefix.toLowerCase())) {
        continue;
      }
      candidates.push({
        providerId: item.id,
        providerLabel: item.label,
        modelId: model.id,
        modelLabel: model.label,
        catalogued: true,
      });
    }
  }
  let status: HandoffTargetStatus = "pending";
  let providerId = provider?.id ?? parsed.providerPrefix;
  let modelId = "";
  if (provider && parsed.modelPrefix) {
    const exact = provider.models.find((model) => model.id === parsed.modelPrefix);
    if (exact) {
      modelId = exact.id;
      status = "resolved";
    } else if (provider.allowCustomModel) {
      modelId = parsed.modelPrefix;
      status = "unverified";
    } else if (candidates.length === 0) {
      status = "unavailable";
    }
  }
  const targetLabel = providerId && modelId ? `${provider?.label ?? providerId} / ${modelId}` : parsed.providerPrefix;
  return {
    raw,
    revision,
    providerId,
    modelId,
    intent: parsed.intent,
    status,
    targetLabel,
    candidates,
  };
}

export function canSubmitHandoffDraft(draft: HandoffDraft): boolean {
  return (draft.status === "resolved" || draft.status === "unverified") && Boolean(draft.providerId && draft.modelId);
}

export function handoffPromptFromIntent(intent: string): string {
  const trimmed = intent.trim();
  return trimmed ? `/handoff -- ${trimmed}` : "/handoff";
}

export function parseRequestedHandoffIntent(payload: string): string | null {
  const trimmed = payload.trim();
  if (!trimmed.startsWith("{")) {
    return null;
  }
  try {
    const parsed = JSON.parse(trimmed) as { awaiting_user_configuration?: unknown; intent?: unknown };
    if (parsed.awaiting_user_configuration !== true) {
      return null;
    }
    return typeof parsed.intent === "string" ? parsed.intent : "";
  } catch {
    return null;
  }
}

function splitOnce(value: string, separator: string): string[] {
  const index = value.indexOf(separator);
  if (index < 0) {
    return [value];
  }
  return [value.slice(0, index), value.slice(index + separator.length)];
}

function readQuotedOrBare(value: string): { token: string; rest: string; complete: boolean } {
  const trimmed = value.trim();
  if (!trimmed.startsWith("\"")) {
    const [token, ...rest] = trimmed.split(/\s+/);
    return { token: token ?? "", rest: rest.join(" "), complete: true };
  }
  let escaped = false;
  let token = "";
  for (let index = 1; index < trimmed.length; index += 1) {
    const char = trimmed[index];
    if (escaped) {
      token += char;
      escaped = false;
      continue;
    }
    if (char === "\\") {
      escaped = true;
      continue;
    }
    if (char === "\"") {
      return { token, rest: trimmed.slice(index + 1), complete: true };
    }
    token += char;
  }
  return { token, rest: "", complete: false };
}
