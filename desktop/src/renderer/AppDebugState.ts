import {
  type Dispatch,
  type RefObject,
  type SetStateAction,
  useEffect,
  useRef,
  useState
} from "react";
import type { ServerEvent } from "../shared/protocol";
import type { AppState } from "./AppState";
import { translateCurrent } from "./i18n";
import type {
  ComposerFile,
  ComposerImage,
  QueuedComposerMessage
} from "./ComposerMessages";
import {
  buildRunDebugSnapshot,
  runDebugEventFromServerEvent,
  type RunDebugEvent
} from "./RunDebugPanel";

const DEBUG_CONTROLS_KEY = "wuu.desktop.debugControlsEnabled";

function initialDebugControlsEnabled(enabled: boolean, forced: boolean): boolean {
  if (!enabled) {
    return false;
  }
  if (forced) {
    return true;
  }
  return window.localStorage.getItem(DEBUG_CONTROLS_KEY) === "true";
}

export function useAppDebugState({
  enabled,
  forced,
  onHideDebugControls
}: {
  enabled: boolean;
  forced: boolean;
  onHideDebugControls: () => void;
}): {
  debugControlsEnabled: boolean;
  setDebugControlsEnabled: Dispatch<SetStateAction<boolean>>;
  debugControlsVisible: boolean;
  runDebugOpen: boolean;
  setRunDebugOpen: Dispatch<SetStateAction<boolean>>;
  /**
   * Whether the "Chip 图鉴" dev panel (ChipGalleryPanel) is open. The
   * toggle lives in the sidebar "开发样例" section and the panel
   * itself renders as a centered modal listing every chip variant in
   * isolation plus four in-conversation mock turns. See
   * `ChipGalleryPanel.tsx`.
   */
  chipGalleryOpen: boolean;
  setChipGalleryOpen: Dispatch<SetStateAction<boolean>>;
  runDebugEvents: RunDebugEvent[];
  runDebugCopied: boolean;
  runDebugRef: RefObject<HTMLDivElement | null>;
  appendRunDebugEvent: (entry: Omit<RunDebugEvent, "id" | "at">) => void;
  resetRunDebugEvents: (entry: Omit<RunDebugEvent, "id" | "at">) => void;
  recordRunDebugEvent: (event: ServerEvent) => void;
  copyRunDebugInfo: (params: {
    state: AppState;
    queuedMessages: QueuedComposerMessage[];
    guideMessages: QueuedComposerMessage[];
    composerImages: ComposerImage[];
    composerFiles: ComposerFile[];
  }) => Promise<void>;
} {
  const [debugControlsEnabled, setDebugControlsEnabled] = useState(() =>
    initialDebugControlsEnabled(enabled, forced)
  );
  const [runDebugOpen, setRunDebugOpen] = useState(false);
  const [chipGalleryOpen, setChipGalleryOpen] = useState(false);
  const [runDebugEvents, setRunDebugEvents] = useState<RunDebugEvent[]>([]);
  const runDebugEventsRef = useRef<RunDebugEvent[]>([]);
  const [runDebugCopied, setRunDebugCopied] = useState(false);
  const runDebugRef = useRef<HTMLDivElement>(null);
  const runDebugEventIDRef = useRef(0);
  const runDebugDeltaSeenRef = useRef(new Set<string>());
  const debugControlsVisible = enabled && debugControlsEnabled;

  function appendRunDebugEvent(entry: Omit<RunDebugEvent, "id" | "at">): void {
    const next: RunDebugEvent = {
      ...entry,
      id: ++runDebugEventIDRef.current,
      at: Date.now()
    };
    const events = [...runDebugEventsRef.current, next].slice(-80);
    runDebugEventsRef.current = events;
    if (runDebugOpen) {
      setRunDebugEvents(events);
    }
  }

  function resetRunDebugEvents(entry: Omit<RunDebugEvent, "id" | "at">): void {
    runDebugDeltaSeenRef.current.clear();
    const next: RunDebugEvent = {
      ...entry,
      id: ++runDebugEventIDRef.current,
      at: Date.now()
    };
    runDebugEventsRef.current = [next];
    if (runDebugOpen) {
      setRunDebugEvents([next]);
    }
  }

  function recordRunDebugEvent(event: ServerEvent): void {
    if (!debugControlsVisible) {
      return;
    }
    const entry = runDebugEventFromServerEvent(event, runDebugDeltaSeenRef.current);
    if (entry) {
      appendRunDebugEvent(entry);
    }
  }

  async function copyRunDebugInfo({
    state,
    queuedMessages,
    guideMessages,
    composerImages,
    composerFiles
  }: {
    state: AppState;
    queuedMessages: QueuedComposerMessage[];
    guideMessages: QueuedComposerMessage[];
    composerImages: ComposerImage[];
    composerFiles: ComposerFile[];
  }): Promise<void> {
    const snapshot = buildRunDebugSnapshot({
      state,
      events: runDebugEventsRef.current,
      queuedMessages,
      guideMessages,
      composerImages,
      composerFiles
    });
    try {
      await navigator.clipboard.writeText(snapshot);
      setRunDebugCopied(true);
      window.setTimeout(() => setRunDebugCopied(false), 1200);
    } catch (error) {
      appendRunDebugEvent({
        source: "client",
        method: "debug/copy",
        detail: error instanceof Error ? error.message : translateCurrent("common.copyFailed"),
        tone: "error"
      });
    }
  }

  useEffect(() => {
    if (enabled) {
      window.localStorage.setItem(DEBUG_CONTROLS_KEY, String(debugControlsEnabled));
    }
  }, [debugControlsEnabled, enabled]);

  useEffect(() => {
    if (debugControlsVisible) {
      return;
    }
    setRunDebugOpen(false);
    setChipGalleryOpen(false);
    onHideDebugControls();
  }, [debugControlsVisible, onHideDebugControls]);

  useEffect(() => {
    if (runDebugOpen) {
      setRunDebugEvents(runDebugEventsRef.current);
    }
  }, [runDebugOpen]);

  return {
    debugControlsEnabled,
    setDebugControlsEnabled,
    debugControlsVisible,
    runDebugOpen,
    setRunDebugOpen,
    chipGalleryOpen,
    setChipGalleryOpen,
    runDebugEvents,
    runDebugCopied,
    runDebugRef,
    appendRunDebugEvent,
    resetRunDebugEvents,
    recordRunDebugEvent,
    copyRunDebugInfo
  };
}
