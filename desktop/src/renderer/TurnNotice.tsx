import { useState } from "react";
import type { ThreadItemStatus } from "../shared/protocol";
import { isUnchangedContextCompaction, type TurnEventDisplay } from "./TurnEvents";
import type { UserFacingErrorDisplay, UserFacingErrorTone } from "./UserFacingErrors";
import { formatCurrentNumber, translateCurrent as t } from "./i18n";
import { ProcessSurfaceFold } from "./ProcessSurfaceFold";
import { ProcessSurfaceMascot } from "./ProcessSurface";
import { useLiveTextWave } from "./LiveTextWave";
import type { TurnStreamStatus } from "./AppState";

export type SystemEventDisplay = {
  label: string;
  detail?: string;
  tone?: UserFacingErrorTone;
  state?: "settled" | "in_progress";
  expandedDetail?: string;
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
  const [expanded, setExpanded] = useState(false);
  const description = event.detail
    ? `${event.label} — ${event.detail}`
    : event.label;
  const summaryText = event.detail
    ? `${event.label} · ${event.detail}`
    : event.label;
  const waveRef = useLiveTextWave<HTMLSpanElement>(inProgress);
  const hasExpandedDetail = Boolean(event.expandedDetail);
  return (
    <aside
      className={`turn-notice process-surface system-event-notice${inProgress ? " is-progress" : ""}${className ? ` ${className}` : ""}`}
      role={tone === "error" || tone === "auth" ? "alert" : "status"}
      aria-label={description}
      aria-live={inProgress ? "polite" : undefined}
    >
      <ProcessSurfaceFold
        summary={
          <span className="process-surface-summary-line" aria-label={description}>
            {inProgress ? (
              <ProcessSurfaceMascot active activity="idle" />
            ) : null}
            <span
              ref={waveRef}
              className={`process-surface-summary-text system-event-copy${inProgress ? " wuu-live-text-wave" : ""}`}
              data-text={summaryText}
            >
              <span className="process-surface-segment system-event-title">
                {event.label}
              </span>
              {event.detail ? (
                <>
                  <span className="process-surface-separator">·</span>
                  <span className="process-surface-segment system-event-detail">
                    {event.detail}
                  </span>
                </>
              ) : null}
            </span>
          </span>
        }
        disabled={!hasExpandedDetail}
        open={expanded}
        onToggle={(toggleEvent) => setExpanded(toggleEvent.currentTarget.open)}
        rowClassName={inProgress ? " is-live-gray is-streaming" : ""}
      >
        <div className="system-event-expanded-detail">{event.expandedDetail}</div>
      </ProcessSurfaceFold>
    </aside>
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
        expandedDetail: display.detail,
        tone: display.tone,
      }}
    />
  );
}

export function StreamStatusNotice({
  status,
}: {
  status: TurnStreamStatus;
}): JSX.Element {
  const detail = streamStatusDetail(status);
  return (
    <SystemEventNotice
      event={{
        label: status.event?.label ?? status.text,
        detail,
        state: status.liveProgress ? "in_progress" : "settled",
      }}
      className="stream-status-notice"
    />
  );
}

function streamStatusDetail(status: TurnStreamStatus): string | undefined {
  const event = status.event;
  if (!event) return undefined;
  const parts: string[] = [];
  parts.push(
    event.maxAttempts
      ? t("appState.attemptOf", {
          attempt: formatCurrentNumber(event.attempt),
          max: formatCurrentNumber(event.maxAttempts),
        })
      : t("appState.attempt", { attempt: formatCurrentNumber(event.attempt) }),
  );
  if (event.retryCount !== undefined) {
    parts.push(
      event.maxRetries !== undefined
        ? t("appState.retryOf", {
            count: formatCurrentNumber(event.retryCount),
            max: formatCurrentNumber(event.maxRetries),
          })
        : t("appState.retryCount", {
            count: formatCurrentNumber(event.retryCount),
          }),
    );
  }
  if (event.submissionCount) {
    parts.push(
      t("appState.requestsSent", {
        count: formatCurrentNumber(event.submissionCount),
      }),
    );
  }
  if (event.waitText) parts.push(event.waitText);
  return parts.join(" · ");
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
}): JSX.Element | null {
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
  const waveRef = useLiveTextWave<HTMLSpanElement>(inProgress);
  const handleToggle = (
    event: React.SyntheticEvent<HTMLDetailsElement>,
  ): void => {
    setExpanded(event.currentTarget.open);
  };
  if (!inProgress && isUnchangedContextCompaction(text)) {
    return null;
  }
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
            className="process-surface-summary-line"
            aria-label={description}
          >
            {inProgress ? (
              <span className="context-compaction-mascot" aria-hidden="true">
                <ProcessSurfaceMascot active activity="compact" />
              </span>
            ) : null}
            <span
              ref={waveRef}
              className={`process-surface-summary-text context-compaction-copy${inProgress ? " wuu-live-text-wave" : ""}`}
              data-text={title}
            >
              <span className="process-surface-segment context-compaction-title">
                {title}
              </span>
              {detail ? (
                <>
                  <span className="process-surface-separator">·</span>
                  <span className="process-surface-segment context-compaction-detail">
                    {detail}
                  </span>
                </>
              ) : null}
            </span>
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
  if (isUnchangedContextCompaction(normalized)) {
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
  if (isUnchangedContextCompaction(normalized)) {
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

function isManualCompact(reason: string | undefined, text: string): boolean {
  return (
    reason === "manual" ||
    /^Manual(?:ly)?\s+(?:context\s+)?compact/i.test(text)
  );
}

function parseContextCompactionNotice(text: string): string | undefined {
  const match = text.match(
    /^(Recovered from context overflow\s+[—-]\s+compacted|Manually compacted|Compacted)\s+history:\s*(\d+)\s*(?:→|->)\s*(\d+)\s+messages(?:\s+\(([^)]+)\))?$/i,
  );
  if (!match) {
    return undefined;
  }
  const [, , before, after, tokenDetail] = match;
  const tokenRange = tokenDetail?.match(
    /^~?([\d.]+[kM]?)\s*(?:→|->)\s*~?([\d.]+[kM]?)\s+tokens$/i,
  );
  if (tokenRange) {
    return t("compaction.summaryWithTokenRange", {
      before,
      after,
      beforeTokens: tokenRange[1],
      afterTokens: tokenRange[2],
    });
  }
  const previousTokens = tokenDetail?.match(/^was\s+~?(.+)$/i)?.[1]?.trim();
  return previousTokens
    ? t("compaction.summaryWithTokens", { before, after, tokens: previousTokens })
    : t("compaction.summary", { before, after });
}
