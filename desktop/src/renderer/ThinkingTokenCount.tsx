import {
  useEffect,
  useRef,
  useState,
  useSyncExternalStore,
  type JSX,
} from "react";
import { turnTelemetryStore } from "./TurnTelemetryStore";
import { useLiveTextWave } from "./LiveTextWave";

/**
 * Live token counter rendered right after the "正在思考" process label.
 *
 * Subscribes to the same turn-telemetry store as the composer meters and
 * shows the turn's cumulative input and output tokens, provider-reported
 * when available (input) or estimated from reasoning deltas (output). The
 * arrows encode direction without label text: ↑ tokens consumed by the
 * model, ↓ tokens streamed out. Digits ease toward each new sample so rapid
 * usage snapshots read as a climbing number instead of a strobing label,
 * and each increment triggers a one-shot jump on the value.
 */

function formatTokenCount(value: number, locale: string): string {
  const safe = Math.max(0, Math.round(value));
  const format = (scaled: number, fractionDigits: number) =>
    new Intl.NumberFormat(locale, { maximumFractionDigits: fractionDigits }).format(
      scaled,
    );
  if (safe >= 1_000_000_000) return `${format(safe / 1_000_000_000, 1)}B`;
  if (safe >= 1_000_000) return `${format(safe / 1_000_000, 1)}M`;
  if (safe >= 1_000) return `${format(safe / 1_000, 1)}k`;
  return format(safe, 0);
}

function prefersReducedMotion(): boolean {
  return (
    typeof window !== "undefined" &&
    typeof window.matchMedia === "function" &&
    window.matchMedia("(prefers-reduced-motion: reduce)").matches
  );
}

// Eases the displayed digits toward each new cumulative sample so the
// counter climbs instead of jumping between snapshots.
function useAnimatedCount(target: number): number {
  const [displayed, setDisplayed] = useState(target);
  const displayedRef = useRef(target);
  const frameRef = useRef(0);

  useEffect(() => {
    const start = displayedRef.current;
    if (start === target) {
      return undefined;
    }
    if (prefersReducedMotion()) {
      displayedRef.current = target;
      setDisplayed(target);
      return undefined;
    }
    const began = performance.now();
    const durationMs = 420;
    const step = (now: number) => {
      const progress = Math.min(1, (now - began) / durationMs);
      const eased = 1 - Math.pow(1 - progress, 3);
      const current = Math.round(start + (target - start) * eased);
      displayedRef.current = current;
      setDisplayed(current);
      if (progress < 1) {
        frameRef.current = window.requestAnimationFrame(step);
      }
    };
    frameRef.current = window.requestAnimationFrame(step);
    return () => {
      if (frameRef.current) {
        window.cancelAnimationFrame(frameRef.current);
      }
    };
  }, [target]);

  return displayed;
}

// One-shot jump highlight on each increment, the "跳动" cue while thinking.
function useFlashOnChange(value: number): boolean {
  const [changing, setChanging] = useState(false);
  const previous = useRef(value);
  useEffect(() => {
    if (previous.current === value) {
      return undefined;
    }
    previous.current = value;
    setChanging(true);
    const timeoutId = window.setTimeout(() => setChanging(false), 180);
    return () => window.clearTimeout(timeoutId);
  }, [value]);
  return changing;
}

function TokenStat({
  target,
  arrow,
  direction,
  locale,
}: {
  target: number;
  arrow: string;
  direction: "input" | "output";
  locale: string;
}): JSX.Element {
  const displayed = useAnimatedCount(target);
  const changing = useFlashOnChange(target);
  const formatted = formatTokenCount(displayed, locale);
  const ariaLabel =
    direction === "input"
      ? locale === "zh-CN"
        ? `输入 ${formatTokenCount(target, locale)} tokens`
        : `Input ${formatTokenCount(target, locale)} tokens`
      : locale === "zh-CN"
        ? `输出 ${formatTokenCount(target, locale)} tokens`
        : `Output ${formatTokenCount(target, locale)} tokens`;

  return (
    <span className="thinking-token-stat" aria-label={ariaLabel}>
      <span
        className={`thinking-token-value${changing ? " is-changing" : ""}`}
      >
        {formatted}
      </span>
      <span className="thinking-token-suffix">{`toks ${arrow}`}</span>
    </span>
  );
}

export function ThinkingTokenCount({
  turnID,
  sweeping = false,
}: {
  turnID: string;
  /** When true, the token text joins the process row's live sweep highlight. */
  sweeping?: boolean;
}): JSX.Element | null {
  const snapshot = useSyncExternalStore(
    turnTelemetryStore.subscribe,
    () => turnTelemetryStore.getSnapshot(turnID),
    () => turnTelemetryStore.getSnapshot(turnID),
  );
  const input = snapshot.inputTokens;
  const output = snapshot.outputTokens;
  const locale =
    typeof document !== "undefined" && document.documentElement?.lang === "zh-CN"
      ? "zh-CN"
      : "en-US";
  const waveRef = useLiveTextWave<HTMLSpanElement>(sweeping);

  if (input <= 0 && output <= 0) {
    return null;
  }

  const stats: string[] = [];
  if (input > 0) stats.push(`${formatTokenCount(input, locale)} toks ↑`);
  if (output > 0) stats.push(`${formatTokenCount(output, locale)} toks ↓`);
  const waveText = ` · ${stats.join(" · ")}`;

  return (
    <span
      ref={waveRef}
      className={`thinking-token-count${sweeping ? " wuu-live-text-wave" : ""}`}
      data-text={waveText}
    >
      <span className="thinking-token-separator" aria-hidden>
        {" · "}
      </span>
      {input > 0 ? (
        <TokenStat target={input} arrow="↑" direction="input" locale={locale} />
      ) : null}
      {input > 0 && output > 0 ? (
        <span className="thinking-token-separator" aria-hidden>
          {" · "}
        </span>
      ) : null}
      {output > 0 ? (
        <TokenStat target={output} arrow="↓" direction="output" locale={locale} />
      ) : null}
    </span>
  );
}
