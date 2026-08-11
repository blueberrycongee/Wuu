import { useState } from "react";
import { useProjection, type Context, type Plugin } from "@wuu-v2/client-runtime";

type ModelProjection = {
  selected: string;
  options: Array<{ id: string; label: string }>;
};

function ModelControl({
  client,
  sessionId,
  ownerProps,
}: {
  client: Context;
  sessionId?: string;
  ownerProps?: unknown;
}) {
  const model = useProjection<ModelProjection>(client, sessionId ?? "", "model");
  const [changing, setChanging] = useState(false);
  const [error, setError] = useState<string>();
  const locked = (ownerProps as { locked?: boolean } | undefined)?.locked ?? false;
  if (!sessionId || !model) return null;
  return (
    <label className="wuu-composer-model" title={error}>
      <select
        aria-label="Model"
        value={model.selected}
        disabled={locked || changing}
        onChange={async (event) => {
          setChanging(true);
          setError(undefined);
          try {
            await client.clientActions.execute("model/select", {
              sessionId,
              providerId: event.currentTarget.value,
            });
          } catch (cause) {
            setError(cause instanceof Error ? cause.message : String(cause));
          } finally {
            setChanging(false);
          }
        }}
      >
        {model.options.map((option) => (
          <option key={option.id} value={option.id}>{option.label}</option>
        ))}
      </select>
    </label>
  );
}

const modelSessionClient: Plugin = function modelSession(client) {
  client.slots.contribute("composer/toolbar-right", {
    id: "model-session",
    order: 10,
    component: ModelControl,
  });
};

modelSessionClient.inject = ["clientActions", "clientProjections", "slots"];
export default modelSessionClient;
