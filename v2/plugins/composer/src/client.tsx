import { createElement, useEffect, useRef, useState } from "react";
import {
  Service,
  SlotOutlet,
  useScopedStore,
  type Context,
  type Plugin,
  type ScopedStoreSeat,
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
  aboveSlot: SlotHandle;
  commandSurfaceSlot: SlotHandle;
  expandedSeat: ScopedStoreSeat<boolean>;
}

function ComposerSurface({
  client,
  aboveSlot,
  commandSurfaceSlot,
  expandedSeat,
  sessionId,
  draft,
  running,
  ariaLabel,
  autoFocus = false,
  placeholder,
  commands = true,
  commandContext,
  footer,
  onVisualHeightChange,
  onDraftChange,
  onSubmit,
  onCancel,
}: SurfaceComponentProps) {
  const stack = useRef<HTMLDivElement>(null);
  const textarea = useRef<HTMLTextAreaElement>(null);
  const [expanded, setExpanded] = useScopedStore(expandedSeat, sessionId);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string>();
  const busy = running || submitting;

  useEffect(() => {
    if (autoFocus) textarea.current?.focus({ preventScroll: true });
  }, [autoFocus, sessionId]);

  useEffect(() => {
    const element = textarea.current;
    if (!element) return;
    if (expanded) {
      element.style.height = "";
      return;
    }
    element.style.height = "0px";
    element.style.height = `${Math.max(60, Math.min(180, element.scrollHeight))}px`;
  }, [draft, expanded]);

  useEffect(() => {
    const element = stack.current;
    if (!element || !onVisualHeightChange) return;
    const emit = () => onVisualHeightChange(element.getBoundingClientRect().height);
    emit();
    if (typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(emit);
    observer.observe(element);
    return () => observer.disconnect();
  }, [onVisualHeightChange]);

  const submit = async (candidate = draft) => {
    const text = candidate.trim();
    if (!text || busy) return;
    const restoreFocus = typeof document !== "undefined" && (
      document.activeElement === document.body ||
      !!stack.current?.contains(document.activeElement)
    );
    setSubmitting(true);
    setError(undefined);
    try {
      await onSubmit(text);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setSubmitting(false);
      if (
        restoreFocus &&
        typeof document !== "undefined" &&
        (document.activeElement === document.body || !!stack.current?.contains(document.activeElement))
      ) textarea.current?.focus();
    }
  };

  const cancel = async () => {
    if (!onCancel) return;
    setError(undefined);
    try {
      await onCancel();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
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
    <div ref={stack} className={`wuu-composer-stack${expanded ? " is-expanded" : ""}`}>
      <SlotOutlet
        client={client}
        slot={aboveSlot}
        sessionId={sessionId}
        ownerProps={{ locked: busy }}
      />
      {commands ? (
        <div className="wuu-composer-command-host">
          <SlotOutlet
            client={client}
            slot={commandSurfaceSlot}
            sessionId={sessionId}
            ownerProps={commandSurface}
          />
        </div>
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
              event.defaultPrevented ||
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
              ownerProps={{ locked: busy }}
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
              ownerProps={{ locked: busy }}
            />
            <button
              className="wuu-composer-send"
              type={running ? "button" : "submit"}
              aria-label={running ? "Stop" : "Send"}
              disabled={running ? !onCancel : !draft.trim() || submitting}
              onClick={running ? () => void cancel() : undefined}
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
    private readonly aboveSlot: SlotHandle,
    private readonly commandSurfaceSlot: SlotHandle,
    private readonly expandedSeat: ScopedStoreSeat<boolean>,
    readonly toolbarLeftSlot: SlotHandle,
    readonly toolbarRightSlot: SlotHandle,
  ) {
    super(ctx, "composerSurfaces");
  }

  render(props: ComposerSurfaceProps) {
    return createElement(ComposerSurface, {
      client: this.ctx,
      aboveSlot: this.aboveSlot,
      commandSurfaceSlot: this.commandSurfaceSlot,
      expandedSeat: this.expandedSeat,
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
  const drafts = client.scopedStores.define("composer/draft", () => "");
  const expandedStates = client.scopedStores.define("composer/expanded", () => false);
  function SessionComposer({
    componentClient,
    sessionId,
    running,
    onVisualHeightChange,
  }: {
    componentClient: Context;
    sessionId: string;
    running: boolean;
    onVisualHeightChange?: (height: number) => void;
  }) {
    const [draft, setDraft] = useScopedStore(drafts, sessionId);
    return componentClient.composerSurfaces.render({
      sessionId,
      draft,
      running,
      autoFocus: true,
      ariaLabel: "Message Wuu",
      placeholder: "Ask Wuu anything",
      ...(onVisualHeightChange ? { onVisualHeightChange } : {}),
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
  function ConversationComposer({
    client: componentClient,
    sessionId,
    ownerProps,
  }: {
    client: Context;
    sessionId?: string;
    ownerProps?: unknown;
  }) {
    const running = (ownerProps as { running?: boolean } | undefined)?.running ?? false;
    const onVisualHeightChange = (
      ownerProps as { onVisualHeightChange?: (height: number) => void } | undefined
    )?.onVisualHeightChange;
    if (!sessionId) return null;
    return (
      <SessionComposer
        componentClient={componentClient}
        sessionId={sessionId}
        running={running}
        {...(onVisualHeightChange ? { onVisualHeightChange } : {})}
      />
    );
  }

  const registration = client.slots.contribute("conversation/composer", {
    id: "default-composer",
    component: ConversationComposer,
    children: [
      { name: "composer/above", kind: "list", scope: "session" },
      { name: "composer/command-surface", kind: "single", scope: "session" },
      { name: "composer/toolbar-left", kind: "list", scope: "session" },
      { name: "composer/toolbar-right", kind: "list", scope: "session" },
    ],
  });
  commandSurfaceSlot = registration.children.get("composer/command-surface")!;
  new ComposerSurfacesService(
    client,
    registration.children.get("composer/above")!,
    commandSurfaceSlot,
    expandedStates,
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

composerClient.inject = ["clientActions", "scopedStores", "slots"];
composerClient.provide = "composerSurfaces";
export default composerClient;
