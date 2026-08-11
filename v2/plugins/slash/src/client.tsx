import { useEffect, useMemo, useRef, useState, useSyncExternalStore } from "react";
import { type Context, type Plugin } from "@wuu-v2/client-runtime";
import type { ComposerCommandSurface } from "@wuu-v2/plugin-composer/client-api";
import { SlashCommandsService, type SlashCommand } from "./service.js";

interface SlashDraft {
  query: string;
  args: string;
}

function parseSlashDraft(value: string): SlashDraft | undefined {
  if (!value.startsWith("/") || value.startsWith("//") || value.includes("\n")) return undefined;
  const body = value.slice(1);
  if (/^\s/.test(body)) return undefined;
  const match = /^(\S*)(?:\s+(.*))?$/.exec(body);
  return match ? { query: (match[1] ?? "").toLowerCase(), args: match[2] ?? "" } : undefined;
}

function searchText(command: SlashCommand): string {
  return [command.name, command.title, command.description, ...(command.aliases ?? []), ...(command.keywords ?? [])]
    .filter(Boolean)
    .join(" ")
    .toLowerCase();
}

function SlashMenu({ client, sessionId, ownerProps }: { client: Context; sessionId?: string; ownerProps?: unknown }) {
  const surface = ownerProps as ComposerCommandSurface;
  const revision = useSyncExternalStore(
    client.slashCommands.subscribe.bind(client.slashCommands),
    client.slashCommands.snapshot,
    client.slashCommands.snapshot,
  );
  const parsed = parseSlashDraft(surface.draft);
  const [dismissedDraft, setDismissedDraft] = useState<string>();
  const [selected, setSelected] = useState(0);
  const [error, setError] = useState<string>();
  const menu = useRef<HTMLDivElement>(null);
  const commands = useMemo(() => {
    const query = parsed?.query ?? "";
    return client.slashCommands.entries()
      .filter((command) => !query || searchText(command).includes(query))
      .slice(0, 8);
  }, [client, parsed?.query, revision]);
  const open = Boolean(sessionId && parsed && surface.draft !== dismissedDraft && commands.length);

  const disabledReason = (command: SlashCommand): string | undefined => sessionId
    ? command.disabled?.({ client, sessionId, running: surface.running })
    : "No active session";
  const nextEnabled = (current: number, direction: 1 | -1): number => {
    for (let step = 1; step <= commands.length; step += 1) {
      const index = (current + direction * step + commands.length) % commands.length;
      const command = commands[index];
      if (command && !disabledReason(command)) return index;
    }
    return current;
  };

  useEffect(() => {
    const first = commands.findIndex((command) => !disabledReason(command));
    setSelected(first < 0 ? 0 : first);
  }, [parsed?.query, revision, sessionId, surface.running]);

  const apply = async (command: SlashCommand) => {
    if (!sessionId || !parsed) return;
    if (disabledReason(command)) return;
    setError(undefined);
    try {
      const result = await command.execute({ client, sessionId, args: parsed.args });
      if (result.type === "replace") {
        surface.setDraft(result.draft);
        setDismissedDraft(result.draft);
      }
      if (result.type === "submit") {
        if (surface.running) throw new Error("The agent is already running");
        await client.clientActions.execute("agent/prompt", { sessionId, text: result.text });
        surface.setDraft("");
      }
      if (result.type === "handled") setDismissedDraft(surface.draft);
      queueMicrotask(surface.focus);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    }
  };

  useEffect(() => {
    if (!open) return undefined;
    const onKeyDown = (event: KeyboardEvent) => {
      if (document.activeElement !== surface.input()) return;
      if (event.isComposing || event.shiftKey || event.metaKey || event.ctrlKey || event.altKey) return;
      if (event.key === "Escape") {
        event.preventDefault();
        setDismissedDraft(surface.draft);
      } else if (event.key === "ArrowDown" || event.key === "ArrowUp") {
        event.preventDefault();
        const direction = event.key === "ArrowDown" ? 1 : -1;
        setSelected((current) => nextEnabled(current, direction));
      } else if (event.key === "Enter" || event.key === "Tab") {
        event.preventDefault();
        const command = commands[selected] ?? commands[0];
        if (command) void apply(command);
      }
    };
    const onPointerDown = (event: PointerEvent) => {
      const target = event.target;
      if (!(target instanceof Node)) return;
      if (menu.current?.contains(target) || surface.input()?.contains(target)) return;
      setDismissedDraft(surface.draft);
    };
    document.addEventListener("keydown", onKeyDown, true);
    document.addEventListener("pointerdown", onPointerDown, true);
    return () => {
      document.removeEventListener("keydown", onKeyDown, true);
      document.removeEventListener("pointerdown", onPointerDown, true);
    };
  }, [commands, open, parsed, selected, sessionId, surface]);

  if (!open) return null;
  return (
    <div ref={menu} className="slash-command-menu" role="listbox" aria-label="Slash commands">
      {commands.map((command, index) => {
        const reason = disabledReason(command);
        return (
          <button
            key={command.id}
            type="button"
            role="option"
            aria-selected={index === selected}
            disabled={Boolean(reason)}
            onMouseDown={(event) => event.preventDefault()}
            onMouseEnter={() => setSelected(index)}
            onClick={() => void apply(command)}
          >
            <span>/{command.name}</span>
            <span>{reason ?? command.title}</span>
          </button>
        );
      })}
      {error ? <div role="alert">{error}</div> : null}
    </div>
  );
}

const slashClient: Plugin = function slash(client) {
  new SlashCommandsService(client);
  client.slots.contribute("composer/command-surface", {
    id: "slash-menu",
    component: SlashMenu,
  });
};

slashClient.inject = ["slots"];
slashClient.provide = "slashCommands";
export default slashClient;
