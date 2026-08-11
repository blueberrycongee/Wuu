import { useState } from "react";
import { useProjection, type Context, type Plugin } from "@wuu-v2/client-runtime";
import type { SlashCommand } from "@wuu-v2/plugin-slash/client-api";
import type { GoalValue } from "./shared.js";

function GoalCard({
  client,
  sessionId,
  ownerProps,
}: {
  client: Context;
  sessionId?: string;
  ownerProps?: unknown;
}) {
  const goal = useProjection<GoalValue>(client, sessionId ?? "", "goal");
  const [completing, setCompleting] = useState(false);
  const locked = (ownerProps as { locked?: boolean } | undefined)?.locked ?? false;
  if (!sessionId || !goal || goal.status !== "active") return null;
  return (
    <section className="wuu-goal-card" aria-label="Active goal">
      <span>{goal.objective}</span>
      <button
        type="button"
        disabled={locked || completing}
        onClick={async () => {
          setCompleting(true);
          try {
            await client.clientActions.execute("goal/complete", { sessionId });
          } finally {
            setCompleting(false);
          }
        }}
      >
        Complete
      </button>
    </section>
  );
}

const styles = `
.wuu-goal-card {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 0 0 8px;
  padding: 9px 12px;
  border: 1px solid var(--hairline);
  border-radius: 12px;
  background: var(--surface-2);
  color: var(--ink-muted);
  font-size: 12px;
}
.wuu-goal-card span { min-width: 0; flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
`;

const goalClient: Plugin = function goal(client) {
  client.slots.contribute("composer/above", {
    id: "goal",
    order: -10,
    component: GoalCard,
  });
  void client.inject(["slashCommands"], (slashClient) => {
    const command: SlashCommand = {
      id: "goal.activate",
      name: "goal",
      title: "Activate a session goal",
      description: "Keep an objective active across turns",
      disabled: ({ running }) => running ? "The agent is running" : undefined,
      execute: async ({ client: commandClient, sessionId, args }) => {
        const objective = args.trim();
        if (!objective) throw new Error("Usage: /goal <objective>");
        await commandClient.clientActions.execute("goal/activate", { sessionId, objective });
        return { type: "replace", draft: "" };
      },
    };
    return slashClient.slashCommands.register(command);
  });
  client.effect(() => {
    if (typeof document === "undefined") return () => {};
    const style = document.createElement("style");
    style.dataset.wuuPluginStyle = "goal";
    style.textContent = styles;
    document.head.append(style);
    return () => style.remove();
  }, "install Goal styles");
};

goalClient.inject = ["clientActions", "clientProjections", "slots"];
export default goalClient;
