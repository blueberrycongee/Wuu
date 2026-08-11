import { useState } from "react";
import { useProjection, type Context, type Plugin } from "@wuu-v2/client-runtime";
import type { SlashCommand } from "@wuu-v2/plugin-slash/client-api";
import type { SubagentValue } from "./shared.js";

type ConversationValue = {
  messages: Array<{ id: string; role: string; text: string; status: string }>;
  running: boolean;
};

function SubagentRow({
  client,
  parentSessionId,
  subagent,
}: {
  client: Context;
  parentSessionId: string;
  subagent: SubagentValue;
}) {
  const [cancelling, setCancelling] = useState<string>();
  const conversation = useProjection<ConversationValue>(
    client,
    subagent.childSessionId,
    "conversation",
  );
  const answer = conversation?.messages.findLast((message) =>
    message.role === "assistant" && message.text.trim())?.text;
  return (
    <div>
      <span>
        <strong>{subagent.status}</strong>
        {subagent.task}
        {answer ? <small>{answer}</small> : null}
      </span>
      {subagent.status === "running" ? (
        <button
          type="button"
          disabled={cancelling === subagent.id}
          onClick={async () => {
            setCancelling(subagent.id);
            try {
              await client.clientActions.execute("subagent/cancel", {
                sessionId: parentSessionId,
                id: subagent.id,
              });
            } finally {
              setCancelling(undefined);
            }
          }}
        >
          Stop
        </button>
      ) : null}
    </div>
  );
}

function SubagentCard({ client, sessionId }: { client: Context; sessionId?: string }) {
  const subagents = useProjection<SubagentValue[]>(client, sessionId ?? "", "subagents");
  if (!sessionId || !subagents?.length) return null;
  return (
    <section className="wuu-subagent-card" aria-label="Subagents">
      {subagents.map((subagent) => (
        <SubagentRow
          key={subagent.id}
          client={client}
          parentSessionId={sessionId}
          subagent={subagent}
        />
      ))}
    </section>
  );
}

const styles = `
.wuu-subagent-card {
  display: grid;
  gap: 6px;
  margin: 0 0 8px;
  padding: 9px 12px;
  border: 1px solid var(--hairline);
  border-radius: 12px;
  background: var(--surface-2);
  color: var(--ink-muted);
  font-size: 12px;
}
.wuu-subagent-card > div { display: flex; align-items: center; gap: 10px; }
.wuu-subagent-card span { display: grid; min-width: 0; flex: 1; grid-template-columns: auto minmax(0, 1fr); gap: 2px 8px; }
.wuu-subagent-card strong { flex: none; font-weight: 600; text-transform: capitalize; }
.wuu-subagent-card small { grid-column: 2; overflow: hidden; color: var(--ink); text-overflow: ellipsis; white-space: nowrap; }
`;

const subagentClient: Plugin = function subagent(client) {
  client.slots.contribute("composer/above", {
    id: "subagent",
    order: 10,
    component: SubagentCard,
  });
  void client.inject(["slashCommands"], (slashClient) => {
    const command: SlashCommand = {
      id: "subagent.start",
      name: "subagent",
      title: "Start a focused subagent",
      description: "Run a delegated task in an independent child session",
      execute: async ({ client: commandClient, sessionId, args }) => {
        const task = args.trim();
        if (!task) throw new Error("Usage: /subagent <task>");
        await commandClient.clientActions.execute("subagent/start", { sessionId, task });
        return { type: "replace", draft: "" };
      },
    };
    return slashClient.slashCommands.register(command);
  });
  client.effect(() => {
    if (typeof document === "undefined") return () => {};
    const style = document.createElement("style");
    style.dataset.wuuPluginStyle = "subagent";
    style.textContent = styles;
    document.head.append(style);
    return () => style.remove();
  }, "install Subagent styles");
};

subagentClient.inject = ["clientActions", "clientProjections", "slots"];
export default subagentClient;
