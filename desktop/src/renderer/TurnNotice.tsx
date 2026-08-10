import { useState } from "react";
import type { ThreadItemStatus } from "../shared/protocol";
import type { TurnEventDisplay } from "./TurnEvents";
import type { UserFacingErrorDisplay, UserFacingErrorTone } from "./UserFacingErrors";
import { translateCurrent as t } from "./i18n";
import { ProcessSurfaceFold } from "./ProcessSurfaceFold";
import { Tooltip } from "./Tooltip";

export type SystemEventDisplay = {
  label: string;
  detail?: string;
  tone?: UserFacingErrorTone;
  state?: "settled" | "in_progress";
};

export function SystemEventNotice({
  event,
  className,
}: {
  event: SystemEventDisplay;
  className?: string;
}): JSX.Element {
  const tone = event.tone ?? "neutral";
  const inProgress = event.state === "in_progress";
  const description = event.detail
    ? `${event.label} — ${event.detail}`
    : event.label;
  return (
    // The visible line carries only the label; when a detail exists the
    // hover tooltip reveals the composed "label — detail" (bounded to the
    // tooltip length cap), and aria-label carries it for screen readers.
    <Tooltip content={event.detail ? description : undefined}>
      <aside
        className={`turn-notice turn-event-notice ${tone}${inProgress ? " is-progress" : ""}${className ? ` ${className}` : ""}`}
        role={tone === "error" || tone === "auth" ? "alert" : "status"}
        aria-label={description}
        aria-live={inProgress ? "polite" : undefined}
      >
        <span className="turn-event-content">
          <strong
            className={`turn-event-title${inProgress ? " wuu-live-text-wave" : ""}`}
            data-text={event.label}
          >
            {event.label}
          </strong>
        </span>
      </aside>
    </Tooltip>
  );
}

export function SystemEventDivider({
  text,
  className,
}: {
  text: string;
  className?: string;
}): JSX.Element {
  return <SystemEventNotice event={{ label: text }} className={className} />;
}

export function TurnEventNotice({
  event,
}: {
  event: TurnEventDisplay;
}): JSX.Element {
  if (event.presentation === "context_compaction") {
    return <ContextCompactionNotice text={event.text} reason={event.reason} status={event.status} summary={event.summary} />;
  }
  return <TurnNotice display={event.notice} />;
}

export function TurnNotice({
  display,
}: {
  display: UserFacingErrorDisplay;
}): JSX.Element {
  return (
    <SystemEventNotice
      event={{
        label: display.title,
        detail: display.detail,
        tone: display.tone,
      }}
    />
  );
}

export function StreamReconnectNotice({
  text,
}: {
  text: string;
}): JSX.Element {
  return (
    <SystemEventNotice
      event={{ label: text, state: "in_progress" }}
      className="context-compaction-notice"
    />
  );
}

export function ContextCompactionNotice({
  text,
  reason,
  status,
  summary,
}: {
  text?: string;
  reason?: string;
  status?: ThreadItemStatus;
  /**
   * Replacement-context body produced by this compaction pass. When present
   * on a settled notice the row becomes expandable (same fold as tool
   * activity) and reveals the compacted context, height-bounded by the
   * shared process-surface body limit.
   */
  summary?: string;
}): JSX.Element {
  const normalized = normalizeContextCompactionText(text);
  const failed = status === "failed" || isFailedCompactNotice(normalized);
  const inProgress = status === "in_progress";
  const title = inProgress
    ? contextCompactionProgressTitle(text, reason)
    : contextCompactionTitle(text, reason, status);
  const detail = inProgress ? undefined : contextCompactionDetail(text, reason, status);
  const state = failed ? "failed" : inProgress ? "in_progress" : "completed";
  const description = detail ? `${title} — ${detail}` : title;
  const hasSummary = !failed && !inProgress && Boolean(summary);
  const [expanded, setExpanded] = useState(false);
  const handleToggle = (
    event: React.SyntheticEvent<HTMLDetailsElement>,
  ): void => {
    setExpanded(event.currentTarget.open);
  };
  return (
    <aside
      className={`process-surface context-compaction-notice ${state}`}
      role={failed ? "alert" : "status"}
      aria-label={description}
      aria-live={inProgress ? "polite" : undefined}
    >
      <ProcessSurfaceFold
        summary={
          <span
            className={`process-surface-summary-line${inProgress ? " wuu-live-text-wave" : ""}`}
            aria-label={description}
          >
            <strong className="process-surface-segment context-compaction-title">
              {title}
            </strong>
            {detail ? (
              <>
                <span className="process-surface-separator">·</span>
                <span className="process-surface-segment context-compaction-detail">
                  {detail}
                </span>
              </>
            ) : null}
          </span>
        }
        disabled={!hasSummary}
        open={expanded}
        onToggle={handleToggle}
        rowClassName={inProgress ? " is-live-gray is-streaming" : ""}
      >
        <div className="context-compaction-summary">{summary}</div>
      </ProcessSurfaceFold>
    </aside>
  );
}

function contextCompactionProgressTitle(text?: string, reason?: string): string {
  if (isManualCompact(reason, normalizeContextCompactionText(text))) {
    return t("compaction.compacting");
  }
  const normalized = normalizeContextCompactionText(text);
  return normalized || t("compaction.autoCompacting");
}

function contextCompactionTitle(
  text?: string,
  reason?: string,
  status?: ThreadItemStatus,
): string {
  const normalized = normalizeContextCompactionText(text);
  if (status === "failed") {
    return t("compaction.failed");
  }
  if (isFailedCompactNotice(normalized)) {
    return t("compaction.failed");
  }
  if (isUnchangedCompactNotice(normalized)) {
    return t("compaction.notNeeded");
  }
  if (isManualCompact(reason, normalized)) {
    return t("compaction.manualComplete");
  }
  return t("compaction.complete");
}

function contextCompactionDetail(
  text?: string,
  reason?: string,
  status?: ThreadItemStatus,
): string {
  const normalized = normalizeContextCompactionText(text);
  if (status === "failed") {
    return t("compaction.failedDetail");
  }
  if (!normalized) {
    return t("compaction.completeDetail");
  }
  if (isFailedCompactNotice(normalized)) {
    return t("compaction.failedDetail");
  }
  if (isUnchangedCompactNotice(normalized)) {
    return t("compaction.notNeededDetail");
  }
  if (/^Compacted history$/i.test(normalized)) {
    return t("compaction.completeDetail");
  }
  const compactNotice = parseContextCompactionNotice(normalized);
  if (compactNotice) {
    return compactNotice;
  }
  return normalized.replace(/^上下文已压缩[:：]\s*/, "");
}

function normalizeContextCompactionText(text?: string): string {
  return (text ?? "").trim().replace(/^[✦*•]\s*/, "");
}

function isFailedCompactNotice(text: string): boolean {
  return /^(?:Manual context compaction|Context compaction|Proactive compact|Context-overflow compact|Compact) failed\b/i.test(
    text,
  );
}

function isUnchangedCompactNotice(text: string): boolean {
  return /^Nothing to compact yet\b/i.test(text);
}

function isManualCompact(reason: string | undefined, text: string): boolean {
  return (
    reason === "manual" ||
    /^Manual(?:ly)?\s+(?:context\s+)?compact/i.test(text)
  );
}

function parseContextCompactionNotice(text: string): string | undefined {
  const match = text.match(
    /^(Recovered from context overflow\s+[—-]\s+compacted|Manually compacted|Compacted)\s+history:\s*(\d+)\s*(?:→|->)\s*(\d+)\s+messages(?:\s+\(was\s+~?([^)]+)\))?$/i,
  );
  if (!match) {
    return undefined;
  }
  const [, , before, after, tokens] = match;
  return tokens
    ? t("compaction.summaryWithTokens", { before, after, tokens: tokens.trim() })
    : t("compaction.summary", { before, after });
}
