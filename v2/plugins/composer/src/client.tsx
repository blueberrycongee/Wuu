import { createElement, useEffect, useRef, useState } from "react";
import {
  Service,
  SlotOutlet,
  type Context,
  type Plugin,
  type SlotHandle,
} from "@wuu-v2/client-runtime";
import type {
  ComposerCommandSurface,
  ComposerSurfaceProps,
  ComposerSurfaceRenderer,
} from "./client-api.js";
import { composerStyles } from "./styles.js";

interface SurfaceComponentProps extends ComposerSurfaceProps {
  client: Context;
  commandSurfaceSlot: SlotHandle;
}

function ComposerSurface({
  client,
  commandSurfaceSlot,
  sessionId,
  draft,
  running,
  ariaLabel,
  autoFocus = false,
  placeholder,
  commands = true,
  commandContext,
  footer,
  onDraftChange,
  onSubmit,
  onCancel,
}: SurfaceComponentProps) {
  const textarea = useRef<HTMLTextAreaElement>(null);
  const [expanded, setExpanded] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string>();
  const busy = running || submitting;

  useEffect(() => {
    if (autoFocus) textarea.current?.focus({ preventScroll: true });
  }, [autoFocus]);

  const submit = async (candidate = draft) => {
    const text = candidate.trim();
    if (!text || busy) return;
    setSubmitting(true);
    setError(undefined);
    try {
      await onSubmit(text);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setSubmitting(false);
    }
  };

  const commandSurface: ComposerCommandSurface = {
    draft,
    running: busy,
    ...(commandContext === undefined ? {} : { context: commandContext }),
    setDraft: onDraftChange,
    submit,
    focus: () => textarea.current?.focus(),
    input: () => textarea.current,
  };

  return (
    <div className={`wuu-composer-stack${expanded ? " is-expanded" : ""}`}>
      {commands ? (
        <SlotOutlet
          client={client}
          slot={commandSurfaceSlot}
          sessionId={sessionId}
          ownerProps={commandSurface}
        />
      ) : null}
      <form className="wuu-composer-surface" onSubmit={(event) => {
        event.preventDefault();
        void submit();
      }}>
        <textarea
          ref={textarea}
          aria-label={ariaLabel}
          placeholder={placeholder}
          value={draft}
          onChange={(event) => onDraftChange(event.currentTarget.value)}
          onKeyDown={(event) => {
            if (
              event.key !== "Enter" ||
              event.shiftKey ||
              event.metaKey ||
              event.ctrlKey ||
              event.altKey ||
              event.nativeEvent.isComposing
            ) return;
            event.preventDefault();
            void submit();
          }}
        />
        <button
          className="wuu-composer-expand"
          type="button"
          aria-label={expanded ? "Collapse input" : "Expand input"}
          aria-pressed={expanded}
          onClick={() => setExpanded((value) => !value)}
        >
          {expanded ? "↓" : "↑"}
        </button>
        <div className="wuu-composer-toolbar">
          <div className="wuu-composer-toolbar-left">
            <SlotOutlet
              client={client}
              slot={client.composerSurfaces.toolbarLeftSlot}
              sessionId={sessionId}
              ownerProps={{ locked: submitting }}
            />
            <span className="wuu-composer-status" role={error ? "alert" : undefined}>
              {error ?? ""}
            </span>
          </div>
          <div className="wuu-composer-toolbar-right">
            <SlotOutlet
              client={client}
              slot={client.composerSurfaces.toolbarRightSlot}
              sessionId={sessionId}
              ownerProps={{ locked: submitting }}
            />
            <button
              className="wuu-composer-send"
              type={running ? "button" : "submit"}
              aria-label={running ? "Stop" : "Send"}
              disabled={running ? !onCancel : !draft.trim() || submitting}
              onClick={running ? () => void onCancel?.() : undefined}
            >
              {running ? "■" : "↑"}
            </button>
          </div>
        </div>
      </form>
      {footer}
    </div>
  );
}

export class ComposerSurfacesService extends Service implements ComposerSurfaceRenderer {
  constructor(
    ctx: Context,
    private readonly commandSurfaceSlot: SlotHandle,
    readonly toolbarLeftSlot: SlotHandle,
    readonly toolbarRightSlot: SlotHandle,
  ) {
    super(ctx, "composerSurfaces");
  }

  render(props: ComposerSurfaceProps) {
    return createElement(ComposerSurface, {
      client: this.ctx,
      commandSurfaceSlot: this.commandSurfaceSlot,
      ...props,
    });
  }
}

declare module "cordis" {
  interface Context {
    composerSurfaces: ComposerSurfacesService;
  }
}

const composerClient: Plugin = function composer(client) {
  let commandSurfaceSlot: SlotHandle;
  function ConversationComposer({
    client: componentClient,
    sessionId,
    ownerProps,
  }: {
    client: Context;
    sessionId?: string;
    ownerProps?: unknown;
  }) {
    const [draft, setDraft] = useState("");
    const running = (ownerProps as { running?: boolean } | undefined)?.running ?? false;
    if (!sessionId) return null;
    return componentClient.composerSurfaces.render({
      sessionId,
      draft,
      running,
      ariaLabel: "Message Wuu",
      placeholder: "Ask Wuu anything",
      onDraftChange: setDraft,
      onSubmit: async (text) => {
        await componentClient.clientActions.execute("agent/prompt", { sessionId, text });
        setDraft("");
      },
      onCancel: async () => {
        await componentClient.clientActions.execute("agent/cancel", { sessionId });
      },
    });
  }

  const registration = client.slots.contribute("conversation/composer", {
    id: "default-composer",
    component: ConversationComposer,
    children: [
      { name: "composer/command-surface", kind: "single", scope: "session" },
      { name: "composer/toolbar-left", kind: "list", scope: "session" },
      { name: "composer/toolbar-right", kind: "list", scope: "session" },
    ],
  });
  commandSurfaceSlot = registration.children.get("composer/command-surface")!;
  new ComposerSurfacesService(
    client,
    commandSurfaceSlot,
    registration.children.get("composer/toolbar-left")!,
    registration.children.get("composer/toolbar-right")!,
  );

  client.effect(() => {
    if (typeof document === "undefined") return () => {};
    const style = document.createElement("style");
    style.dataset.wuuPluginStyle = "composer";
    style.textContent = composerStyles;
    document.head.append(style);
    return () => style.remove();
  }, "install composer styles");
};

composerClient.inject = ["clientActions", "slots"];
composerClient.provide = "composerSurfaces";
export default composerClient;
