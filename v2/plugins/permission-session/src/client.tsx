import { useState } from "react";
import { useProjection, type Context, type Plugin } from "@wuu-v2/client-runtime";

type PermissionProjection = {
  selected: string;
  options: Array<{ id: string; label: string }>;
};

function PermissionControl({
  client,
  sessionId,
  ownerProps,
}: {
  client: Context;
  sessionId?: string;
  ownerProps?: unknown;
}) {
  const permission = useProjection<PermissionProjection>(client, sessionId ?? "", "permission");
  const [changing, setChanging] = useState(false);
  const [error, setError] = useState<string>();
  const locked = (ownerProps as { locked?: boolean } | undefined)?.locked ?? false;
  if (!sessionId || !permission) return null;
  return (
    <label className="wuu-composer-permission" title={error}>
      <select
        aria-label="Permission"
        value={permission.selected}
        disabled={locked || changing}
        onChange={async (event) => {
          setChanging(true);
          setError(undefined);
          try {
            await client.clientActions.execute("permission/select", {
              sessionId,
              mode: event.currentTarget.value,
            });
          } catch (cause) {
            setError(cause instanceof Error ? cause.message : String(cause));
          } finally {
            setChanging(false);
          }
        }}
      >
        {permission.options.map((option) => (
          <option key={option.id} value={option.id}>{option.label}</option>
        ))}
      </select>
    </label>
  );
}

const permissionSessionClient: Plugin = function permissionSession(client) {
  client.slots.contribute("composer/toolbar-left", {
    id: "permission-session",
    order: 10,
    component: PermissionControl,
  });
};

permissionSessionClient.inject = ["clientActions", "clientProjections", "slots"];
export default permissionSessionClient;
