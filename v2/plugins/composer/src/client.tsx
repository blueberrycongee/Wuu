import { useState } from "react";
import { type Context, type Plugin } from "@wuu-v2/client-runtime";

function Composer({ client, sessionId, ownerProps }: { client: Context; sessionId?: string; ownerProps?: unknown }) {
  const [draft, setDraft] = useState("");
  const running = (ownerProps as { running?: boolean } | undefined)?.running ?? false;
  return (
    <form className="composer" onSubmit={async (event) => {
      event.preventDefault();
      const text = draft.trim();
      if (!sessionId || !text || running) return;
      await client.clientActions.execute("agent/prompt", { sessionId, text });
      setDraft("");
    }}>
      <textarea aria-label="Message Wuu" value={draft} onChange={(event) => setDraft(event.currentTarget.value)} />
      <button type="submit" disabled={!draft.trim() || running}>{running ? "Running" : "Send"}</button>
    </form>
  );
}

const composerClient: Plugin = function composer(client) {
  client.slots.contribute("conversation/composer", { id: "default-composer", component: Composer });
};
composerClient.inject = ["clientActions", "slots"];
export default composerClient;
