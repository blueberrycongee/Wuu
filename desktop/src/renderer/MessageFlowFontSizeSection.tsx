import {
  type ChangeEvent,
  type PointerEvent,
  useEffect,
  useState,
} from "react";
import {
  MESSAGE_FLOW_FONT_SIZE_RANGE,
  type MessageFlowFontSize,
} from "../shared/protocol";
import { StreamingMarkdown } from "./StreamingMarkdown";
import { useI18n } from "./i18n";

// Sample reply the Settings preview shows underneath the slider. It is
// rendered through the exact pipeline the conversation pane uses
// (.agent-block > .agent-text > StreamingMarkdown), so the slider
// previews real message-flow typography instead of a look-alike
// placeholder. Mixed CJK/Latin prose, inline code, and a short list
// cover the shapes a typical agent reply takes.
const { min, max, step, default: defaultSize } = MESSAGE_FLOW_FONT_SIZE_RANGE;

function clampSize(value: unknown): MessageFlowFontSize {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    return defaultSize;
  }
  return Math.min(max, Math.max(min, value));
}

function formatSize(size: number): string {
  return `${size.toFixed(1)}px`;
}

/**
 * Stamp --conversation-message-font-size on <html> from the chosen px
 * value. The inline style on the document element wins over the
 * `:root` declaration in conversation-shell.css so the change cascades
 * through every message-flow surface (turns.css, chat.css,
 * participants.css, and the settings-page preview). Mirrors the
 * applyThemePreference shape — caller-friendly side-effect helper.
 */
export function applyMessageFlowFontSize(size: MessageFlowFontSize): void {
  const clamped = clampSize(size);
  try {
    document.documentElement.style.setProperty(
      "--conversation-message-font-size",
      `${clamped}px`,
    );
  } catch {
    // Same fall-back story as Theme.ts — losing the inline stamp only
    // costs a one-frame flash at the default size.
  }
}

/**
 * Settings row body for the user-facing message-stream reading size:
 * a slider, the current px value, and a live preview block below that
 * reads the same CSS variables the conversation pane uses. Reads and
 * persists through `window.wuu` directly and applies the choice on
 * every drag tick.
 */
export function MessageFlowFontSizeControl(): JSX.Element {
  const { locale, t } = useI18n();
  const [size, setSize] = useState<MessageFlowFontSize>(() =>
    clampSize(window.wuu?.initialMessageFlowFontSize),
  );

  useEffect(() => {
    let cancelled = false;
    void window.wuu
      ?.getMessageFlowFontSize?.()
      .then((stored) => {
        if (!cancelled) {
          setSize(clampSize(stored));
        }
      })
      .catch(() => {
        // Keep the preload-provided initial value.
      });
    return () => {
      cancelled = true;
    };
  }, []);

  function liveUpdate(value: number): void {
    const clamped = clampSize(value);
    setSize(clamped);
    // Write the CSS variable on every onChange tick so the
    // .message-flow-preview below reacts immediately while the user
    // drags. The actual persistence happens on release
    // (commitOnRelease) so we don't spam the JSON file with each
    // pixel of drag motion.
    applyMessageFlowFontSize(clamped);
  }

  function persist(value: number): void {
    void window.wuu?.setMessageFlowFontSize?.(clampSize(value)).catch(() => {
      // Persistence failure leaves the applied size for this window;
      // the next launch falls back to the stored value.
    });
  }

  return (
    <div className="message-flow-font-size-control">
      <div className="message-flow-font-size-slider-row">
        <input
          type="range"
          min={min}
          max={max}
          step={step}
          value={size}
          onChange={(event: ChangeEvent<HTMLInputElement>) =>
            liveUpdate(Number.parseFloat(event.currentTarget.value))
          }
          onPointerUp={(event: PointerEvent<HTMLInputElement>) =>
            persist(Number.parseFloat(event.currentTarget.value))
          }
          onBlur={(event) =>
            persist(Number.parseFloat(event.currentTarget.value))
          }
          aria-label={t("settings.messageFontSize")}
          data-testid="settings-message-flow-font-size-slider"
        />
        <span
          className="message-flow-font-size-value"
          aria-live="polite"
          data-testid="settings-message-flow-font-size-value"
        >
          {formatSize(size)}
        </span>
      </div>
      <div
        className="message-flow-preview"
        aria-label={t("settings.messageFontSizePreview")}
        data-testid="settings-message-flow-font-size-preview"
      >
        <article className="agent-block">
          <div className="agent-text">
            <StreamingMarkdown
              streamKey={`settings-message-flow-font-size-preview-${locale}`}
              initialText={[
                t("settings.messageFontSizeSampleIntro"),
                "",
                t("settings.messageFontSizeSampleScope"),
                t("settings.messageFontSizeSampleTests"),
              ].join("\n")}
              isLive={false}
              phase="final_answer"
            />
          </div>
        </article>
      </div>
    </div>
  );
}
