import { useRef, useState } from "react";
import { SlotOutlet, type Context, type Plugin, type SlotHandle } from "@wuu-v2/client-runtime";
import type { ComposerCommandSurface } from "./client-api.js";

const composerClient: Plugin = function composer(client) {
  let commandSurfaceSlot: SlotHandle;
  function Composer({ client: componentClient, sessionId, ownerProps }: { client: Context; sessionId?: string; ownerProps?: unknown }) {
    const [draft, setDraft] = useState("");
    const textarea = useRef<HTMLTextAreaElement>(null);
    const running = (ownerProps as { running?: boolean } | undefined)?.running ?? false;
    const commandSurface: ComposerCommandSurface = {
      draft,
      running,
      setDraft,
      focus: () => textarea.current?.focus(),
      input: () => textarea.current,
    };
    return (
      <form className="composer" onSubmit={async (event) => {
        event.preventDefault();
        const text = draft.trim();
        if (!sessionId || !text || running) return;
        await componentClient.clientActions.execute("agent/prompt", { sessionId, text });
        setDraft("");
      }}>
        <SlotOutlet
          client={componentClient}
          slot={commandSurfaceSlot}
          {...(sessionId ? { sessionId } : {})}
          ownerProps={commandSurface}
        />
        <textarea
          ref={textarea}
          aria-label="Message Wuu"
          value={draft}
          onChange={(event) => setDraft(event.currentTarget.value)}
        />
        <button type="submit" disabled={!draft.trim() || running}>{running ? "Running" : "Send"}</button>
      </form>
    );
  }

  const registration = client.slots.contribute("conversation/composer", {
    id: "default-composer",
    component: Composer,
    children: [{ name: "composer/command-surface", kind: "single", scope: "session" }],
  });
  commandSurfaceSlot = registration.children.get("composer/command-surface")!;
};
composerClient.inject = ["clientActions", "slots"];
export default composerClient;
