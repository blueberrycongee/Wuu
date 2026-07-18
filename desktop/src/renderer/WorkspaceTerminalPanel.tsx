import { FitAddon } from "@xterm/addon-fit";
import { Terminal as XtermTerminal, type ITheme } from "@xterm/xterm";
import "@xterm/xterm/css/xterm.css";
import { CheckCircle2, Clock3, SquareTerminal, Terminal, XCircle } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import type { RuntimeContext, TerminalSessionEvent, Thread } from "../shared/protocol";
import { currentAppliedTheme, observeAppliedTheme, type AppliedTheme } from "./Theme";
import {
  agentRunGroupsForThread,
  selectAgentRun,
  type AgentRunLocator,
  type AgentRunRecord,
} from "./TerminalRuns";
import { WorkspacePanelEmpty } from "./WorkspaceFiles";
import { desktopApiErrorMessage } from "./WorkspaceReviewHelpers";
import { translateCurrent, useI18n } from "./i18n";

const WORKSPACE_TERMINAL_PENDING_EVENT_IDS = 12;
const WORKSPACE_TERMINAL_PENDING_EVENTS_PER_ID = 256;
const WORKSPACE_TERMINAL_PENDING_TEXT_PER_ID = 512 * 1024;

type WorkspaceTerminalState = "starting" | "ready" | "exited" | "error";

function workspaceTerminalTheme(theme: AppliedTheme): ITheme {
  if (theme === "dark") {
    return {
      background: "#1d2024",
      black: "#141618",
      blue: "#58a6ff",
      cursor: "#f2f3f4",
      foreground: "#e4e6e8",
      green: "#4cc38a",
      red: "#f0705f",
      selectionBackground: "#3a4046",
      yellow: "#d9a84e",
    };
  }
  return {
    background: "#ffffff",
    black: "#24292f",
    blue: "#2f98ff",
    cursor: "#202427",
    foreground: "#1f2328",
    green: "#1f9d46",
    red: "#b42318",
    selectionBackground: "#d7e9ff",
    yellow: "#ffc21a",
  };
}

export function appendPendingTerminalEvent(
  events: TerminalSessionEvent[],
  event: TerminalSessionEvent,
): TerminalSessionEvent[] {
  const normalized =
    event.type === "data" &&
    event.text.length > WORKSPACE_TERMINAL_PENDING_TEXT_PER_ID
      ? {
          ...event,
          text: event.text.slice(-WORKSPACE_TERMINAL_PENDING_TEXT_PER_ID),
        }
      : event;
  const next = [...events, normalized];
  let textLength = next.reduce(
    (total, pending) =>
      total + (pending.type === "data" ? pending.text.length : 0),
    0,
  );
  while (
    next.length > WORKSPACE_TERMINAL_PENDING_EVENTS_PER_ID ||
    textLength > WORKSPACE_TERMINAL_PENDING_TEXT_PER_ID
  ) {
    const dataIndex = next.findIndex((pending) => pending.type === "data");
    const removeIndex = dataIndex >= 0 ? dataIndex : 0;
    const [removed] = next.splice(removeIndex, 1);
    if (removed?.type === "data") {
      textLength -= removed.text.length;
    }
  }
  return next;
}

function formatTerminalDuration(ms: number): string {
  const totalSeconds = Math.max(0, Math.floor(ms / 1000));
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  if (hours > 0) {
    return translateCurrent("workspace.terminal.durationHours", { hours, minutes, seconds });
  }
  if (minutes > 0) {
    return translateCurrent("workspace.terminal.durationMinutes", { minutes, seconds });
  }
  return translateCurrent("workspace.terminal.durationSeconds", { seconds });
}

function terminalExitText(event: Extract<TerminalSessionEvent, { type: "exit" }>): string {
  const duration = formatTerminalDuration(event.duration_ms);
  if (event.signal) {
    return translateCurrent("workspace.terminal.stoppedBy", { signal: event.signal, duration });
  }
  return translateCurrent("workspace.terminal.exited", {
    code: event.exit_code ?? translateCurrent("workspace.terminal.unknown"),
    duration,
  });
}

export type WorkspaceTerminalRunRequest = AgentRunLocator & { requestID: number };

export function WorkspaceTerminalPanel({
  activeContext,
  thread,
  requestedRun,
}: {
  activeContext?: RuntimeContext;
  thread?: Thread;
  requestedRun?: WorkspaceTerminalRunRequest;
}): JSX.Element {
  const { t } = useI18n();
  const groups = useMemo(
    () => (thread ? agentRunGroupsForThread(thread) : []),
    [thread],
  );
  const requestedRecord = requestedRun ? selectAgentRun(groups, requestedRun) : undefined;
  const [selectedResourceID, setSelectedResourceID] = useState(
    () => requestedRecord?.toolCallID ?? "user-terminal",
  );
  const [userTerminalOpened, setUserTerminalOpened] = useState(
    () => !requestedRecord,
  );
  const selectedRun = groups
    .flatMap((group) => group.runs)
    .find((run) => run.toolCallID === selectedResourceID);

  useEffect(() => {
    if (!requestedRun) {
      return;
    }
    const next = selectAgentRun(groups, requestedRun);
    if (next) {
      setSelectedResourceID(next.toolCallID);
    }
  }, [groups, requestedRun?.requestID, requestedRun?.threadID, requestedRun?.turnID, requestedRun?.toolCallID]);

  useEffect(() => {
    if (selectedResourceID === "user-terminal" || selectedRun) {
      return;
    }
    setSelectedResourceID("user-terminal");
    setUserTerminalOpened(true);
  }, [selectedResourceID, selectedRun]);

  function openUserTerminal(): void {
    setUserTerminalOpened(true);
    setSelectedResourceID("user-terminal");
  }

  return (
    <div className="workspace-terminal-workspace">
      <nav className="workspace-terminal-navigation" aria-label={t("workspace.terminal.resources")}>
        <button
          className={`workspace-terminal-resource${selectedResourceID === "user-terminal" ? " active" : ""}`}
          type="button"
          onClick={openUserTerminal}
        >
          <SquareTerminal className="icon" />
          <span>{t("workspace.terminal.userTerminal")}</span>
        </button>
        <div className="workspace-terminal-run-list">
          <div className="workspace-terminal-nav-heading">{t("workspace.terminal.agentRuns")}</div>
          {groups.length > 0 ? groups.slice().reverse().map((group) => (
            <section className="workspace-terminal-turn-group" key={group.turnID}>
              <div className="workspace-terminal-turn-heading">
                {t("workspace.terminal.turnLabel", { number: group.turnNumber })}
              </div>
              {group.runs.map((run) => (
                <button
                  className={`workspace-terminal-resource workspace-terminal-run${selectedResourceID === run.toolCallID ? " active" : ""}`}
                  type="button"
                  key={run.toolCallID}
                  title={run.command}
                  onClick={() => setSelectedResourceID(run.toolCallID)}
                >
                  <RunStatusIcon run={run} />
                  <span>{run.command}</span>
                </button>
              ))}
            </section>
          )) : (
            <div className="workspace-terminal-no-runs">{t("workspace.terminal.noRuns")}</div>
          )}
        </div>
      </nav>
      <div className="workspace-terminal-content">
        {userTerminalOpened ? (
          <UserTerminalPane
            active={selectedResourceID === "user-terminal"}
            activeContext={activeContext}
          />
        ) : null}
        {selectedRun ? <AgentRunPane run={selectedRun} /> : null}
      </div>
    </div>
  );
}

function RunStatusIcon({ run }: { run: AgentRunRecord }): JSX.Element {
  if (run.status === "failed") {
    return <XCircle className="icon failed" />;
  }
  if (run.status === "completed") {
    return <CheckCircle2 className="icon completed" />;
  }
  return <Clock3 className="icon" />;
}

function AgentRunPane({ run }: { run: AgentRunRecord }): JSX.Element {
  const { t } = useI18n();
  const meta = [
    run.exitCode !== undefined ? t("workspace.terminal.exitCode", { code: run.exitCode }) : undefined,
    run.durationMs !== undefined ? formatTerminalDuration(run.durationMs) : undefined,
    run.timedOut ? t("workspace.terminal.timedOut") : undefined,
  ].filter((value): value is string => Boolean(value));
  const hasOutput = Boolean(run.stdout || run.stderr || run.output);

  return (
    <article className="workspace-agent-run" data-tool-call-id={run.toolCallID}>
      <header className="workspace-agent-run-header">
        <div>
          <div className="workspace-agent-run-kicker">{t("workspace.terminal.readOnlyRun")}</div>
          <h2>{run.command}</h2>
        </div>
        <span className={`workspace-agent-run-status ${run.status}`}>
          {runStatusLabel(run)}
        </span>
      </header>
      {meta.length > 0 ? <div className="workspace-agent-run-meta">{meta.join(" · ")}</div> : null}
      <div className="workspace-agent-run-output">
        {run.stdout ? <RunOutputSection label="stdout" text={run.stdout} /> : null}
        {run.stderr ? <RunOutputSection label="stderr" text={run.stderr} error /> : null}
        {run.output ? <RunOutputSection label={t("workspace.terminal.output")} text={run.output} /> : null}
        {!hasOutput ? <div className="workspace-agent-run-empty">{t("workspace.terminal.noOutput")}</div> : null}
      </div>
      {run.truncated ? (
        <div className="workspace-agent-run-notice">{t("workspace.terminal.retainedOutputTruncated")}</div>
      ) : null}
    </article>
  );
}

function runStatusLabel(run: AgentRunRecord): string {
  switch (run.status) {
    case "completed":
      return translateCurrent("workspace.terminal.status.completed");
    case "failed":
      return translateCurrent("workspace.terminal.status.failed");
    case "interrupted":
      return translateCurrent("workspace.terminal.status.interrupted");
    case "in_progress":
      return translateCurrent("workspace.terminal.status.inProgress");
  }
}

function RunOutputSection({
  label,
  text,
  error = false,
}: {
  label: string;
  text: string;
  error?: boolean;
}): JSX.Element {
  return (
    <section className={`workspace-agent-run-stream${error ? " error" : ""}`}>
      <div className="workspace-agent-run-stream-label">{label}</div>
      <pre>{text}</pre>
    </section>
  );
}

function UserTerminalPane({
  active,
  activeContext,
}: {
  active: boolean;
  activeContext?: RuntimeContext;
}): JSX.Element {
  const { t } = useI18n();
  const containerRef = useRef<HTMLDivElement | null>(null);
  const terminalRef = useRef<XtermTerminal | null>(null);
  const sessionIDRef = useRef<string | undefined>(undefined);
  const pendingTerminalEventsRef = useRef(new Map<string, TerminalSessionEvent[]>());
  const [terminalState, setTerminalState] = useState<WorkspaceTerminalState>("starting");
  const [restartKey, setRestartKey] = useState(0);
  const workspaceRoot = activeContext?.cwd;

  useEffect(() => {
    if (active) {
      terminalRef.current?.focus();
    }
  }, [active]);

  useEffect(() => {
    const container = containerRef.current;
    if (!workspaceRoot || !container) {
      return undefined;
    }

    let disposed = false;
    let resizeFrame: number | undefined;
    setTerminalState("starting");
    pendingTerminalEventsRef.current.clear();

    const terminal = new XtermTerminal({
      allowTransparency: false,
      convertEol: false,
      cursorBlink: true,
      fontFamily: '"SFMono-Regular", Consolas, "Liberation Mono", monospace',
      fontSize: 12,
      lineHeight: 1.45,
      scrollback: 5000,
      theme: workspaceTerminalTheme(currentAppliedTheme()),
    });
    const fitAddon = new FitAddon();
    terminal.loadAddon(fitAddon);
    terminal.open(container);
    terminal.focus();
    terminalRef.current = terminal;
    const stopObservingTheme = observeAppliedTheme((theme) => {
      terminal.options.theme = workspaceTerminalTheme(theme);
    });

    function fitAndResize(): void {
      if (disposed) {
        return;
      }
      try {
        fitAddon.fit();
      } catch {
        return;
      }
      const id = sessionIDRef.current;
      if (id) {
        void window.wuu.resizeTerminalSession(id, terminal.cols, terminal.rows);
      }
    }

    const dataDisposable = terminal.onData((data) => {
      const id = sessionIDRef.current;
      if (id) {
        void window.wuu.writeTerminalSession(id, data);
      }
    });
    const resizeObserver = new ResizeObserver(() => {
      if (resizeFrame !== undefined) {
        window.cancelAnimationFrame(resizeFrame);
      }
      resizeFrame = window.requestAnimationFrame(fitAndResize);
    });
    resizeObserver.observe(container);
    resizeFrame = window.requestAnimationFrame(fitAndResize);

    function bufferTerminalEvent(event: TerminalSessionEvent): void {
      const events = pendingTerminalEventsRef.current.get(event.id) ?? [];
      pendingTerminalEventsRef.current.set(
        event.id,
        appendPendingTerminalEvent(events, event),
      );
      while (pendingTerminalEventsRef.current.size > WORKSPACE_TERMINAL_PENDING_EVENT_IDS) {
        const firstID = pendingTerminalEventsRef.current.keys().next().value;
        if (!firstID) {
          break;
        }
        pendingTerminalEventsRef.current.delete(firstID);
      }
    }

    function handleTerminalEvent(event: TerminalSessionEvent): void {
      if (event.type === "data") {
        terminal.write(event.text);
        return;
      }
      if (event.type === "exit") {
        terminal.writeln("");
        terminal.writeln(`[${terminalExitText(event)}]`);
        setTerminalState("exited");
        sessionIDRef.current = undefined;
        return;
      }
      terminal.writeln("");
      terminal.writeln(`[${translateCurrent("workspace.terminal.error", { message: event.message })}]`);
      setTerminalState("error");
      sessionIDRef.current = undefined;
    }

    function flushPendingTerminalEvents(id: string): void {
      const events = pendingTerminalEventsRef.current.get(id);
      if (!events) {
        return;
      }
      pendingTerminalEventsRef.current.delete(id);
      for (const event of events) {
        handleTerminalEvent(event);
      }
    }

    const unsubscribeTerminal = window.wuu.onTerminalEvent((event) => {
      if (event.id !== sessionIDRef.current) {
        bufferTerminalEvent(event);
        return;
      }
      handleTerminalEvent(event);
    });

    async function startSession(): Promise<void> {
      try {
        fitAndResize();
        const started = await window.wuu.startTerminalSession({
          cols: terminal.cols,
          rows: terminal.rows,
          cwd: workspaceRoot
        });
        if (disposed) {
          void window.wuu.stopTerminalSession(started.id);
          return;
        }
        sessionIDRef.current = started.id;
        setTerminalState("ready");
        flushPendingTerminalEvents(started.id);
        fitAndResize();
        terminal.focus();
      } catch (error) {
        terminal.writeln(desktopApiErrorMessage(error, translateCurrent("workspace.terminal.startFailed")));
        setTerminalState("error");
      }
    }

    void startSession();

    return () => {
      disposed = true;
      if (resizeFrame !== undefined) {
        window.cancelAnimationFrame(resizeFrame);
      }
      const sessionID = sessionIDRef.current;
      sessionIDRef.current = undefined;
      if (sessionID) {
        void window.wuu.stopTerminalSession(sessionID);
      }
      unsubscribeTerminal();
      stopObservingTheme();
      dataDisposable.dispose();
      resizeObserver.disconnect();
      pendingTerminalEventsRef.current.clear();
      terminal.dispose();
      terminalRef.current = null;
    };
  }, [restartKey, workspaceRoot]);

  if (!workspaceRoot) {
    return <WorkspacePanelEmpty title={t("workspace.files.noProject")} description={t("workspace.terminal.noProjectDescription")} icon={<Terminal size={24} />} />;
  }

  return (
    <div className="workspace-terminal-panel" hidden={!active}>
      <div className="workspace-terminal-screen" onMouseDown={() => terminalRef.current?.focus()}>
        <div className="workspace-terminal-host" ref={containerRef} />
      </div>
      {terminalState === "exited" || terminalState === "error" ? (
        <button className="workspace-terminal-restart" type="button" onClick={() => setRestartKey((current) => current + 1)}>
          {t("workspace.terminal.restart")}
        </button>
      ) : null}
    </div>
  );
}
