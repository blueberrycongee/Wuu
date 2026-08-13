import { AlertCircle, Check, Copy, FileText, GitFork, PencilLine, SquareTerminal, X } from "lucide-react";
import { type KeyboardEvent as ReactKeyboardEvent, useEffect, useRef, useState } from "react";
import type { InputFile, InputImage } from "../shared/protocol";
import { imageSource } from "./ComposerMessages";
import { useImagePreview } from "./ImagePreview";
import { useI18n } from "./i18n";

export function AgentMessageActions({
  getText,
  onFork,
  onOpenRuns,
  placement,
}: {
  getText: () => string;
  onFork?: () => void;
  onOpenRuns?: () => void;
  placement: "overlay" | "persistent";
}): JSX.Element {
  const { t } = useI18n();

  return (
    <div
      className="message-actions agent-message-actions"
      data-wuu-component="message-actions"
      data-wuu-placement={placement}
      aria-label={t("message.assistantActions")}
    >
      <MessageCopyButton getText={getText} className="message-action-button" iconSize={15} />
      {onOpenRuns ? <TurnRunButton onOpenRuns={onOpenRuns} /> : null}
      <button
        className="message-action-button"
        type="button"
        aria-label={t("message.fork")}
        title={t("message.fork")}
        disabled={!onFork}
        onClick={onFork}
      >
        <GitFork className="icon" />
      </button>
    </div>
  );
}

function TurnRunButton({ onOpenRuns }: { onOpenRuns: () => void }): JSX.Element {
  const { t } = useI18n();
  const label = t("message.viewTurnRuns");
  return (
    <button
      className="message-action-button"
      type="button"
      aria-label={label}
      title={label}
      onClick={onOpenRuns}
    >
      <SquareTerminal className="icon" />
    </button>
  );
}

export function MessageCopyButton({
  getText,
  className = "",
  iconSize = 14,
  idleLabel,
  copiedLabel,
  failedLabel,
}: {
  getText: () => string;
  className?: string;
  iconSize?: number;
  idleLabel?: string;
  copiedLabel?: string;
  failedLabel?: string;
}): JSX.Element {
  const { t } = useI18n();
  const resolvedIdleLabel = idleLabel ?? t("message.copy");
  const resolvedCopiedLabel = copiedLabel ?? t("message.copied");
  const resolvedFailedLabel = failedLabel ?? t("common.copyFailed");
  const [copyState, setCopyState] = useState<"idle" | "copied" | "failed">("idle");
  const resetTimerRef = useRef<number | undefined>(undefined);
  const label = copyState === "copied" ? resolvedCopiedLabel : copyState === "failed" ? resolvedFailedLabel : resolvedIdleLabel;

  useEffect(() => {
    return () => {
      if (resetTimerRef.current !== undefined) {
        window.clearTimeout(resetTimerRef.current);
      }
    };
  }, []);

  function showCopyState(nextState: "copied" | "failed"): void {
    if (resetTimerRef.current !== undefined) {
      window.clearTimeout(resetTimerRef.current);
    }
    setCopyState(nextState);
    resetTimerRef.current = window.setTimeout(() => {
      setCopyState("idle");
      resetTimerRef.current = undefined;
    }, 1200);
  }

  async function handleCopy(): Promise<void> {
    const text = getText();
    if (text.trim() === "") {
      showCopyState("failed");
      return;
    }
    try {
      const clipboard = navigator.clipboard;
      if (!clipboard?.writeText) {
        throw new Error("Clipboard API unavailable");
      }
      await clipboard.writeText(text);
      showCopyState("copied");
    } catch {
      showCopyState("failed");
    }
  }

  return (
    <button
      className={`message-copy-button ${className} ${copyState}`}
      type="button"
      aria-label={label}
      title={label}
      onClick={() => void handleCopy()}
    >
      {copyState === "copied" ? (
        <Check size={iconSize} />
      ) : copyState === "failed" ? (
        <AlertCircle size={iconSize} />
      ) : (
        <Copy size={iconSize} />
      )}
    </button>
  );
}

export function MessageEditButton({
  onEdit,
  className = "",
  iconSize = 14
}: {
  onEdit: () => void;
  className?: string;
  iconSize?: number;
}): JSX.Element {
  const { t } = useI18n();
  return (
    <button
      className={`message-edit-button ${className}`}
      type="button"
      aria-label={t("message.editAndRetry")}
      title={t("message.editAndRetry")}
      onClick={onEdit}
    >
      <PencilLine size={iconSize} />
    </button>
  );
}

export function MessageImageGrid({
  images,
  onRemove,
}: {
  images: InputImage[];
  /** When provided, each image gets a remove button (used inside the inline editor). */
  onRemove?: (index: number) => void;
}): JSX.Element {
  const { t } = useI18n();
  const { openPreview } = useImagePreview();
  return (
    <div className={`message-images${onRemove ? " message-images-editable" : ""}`}>
      {images.map((image, index) => {
        const src = imageSource(image);
        const label = t("composer.imageNumber", { number: index + 1 });
        const handleOpen = (): void => {
          openPreview({ src, alt: label, title: label });
        };
        const handleKeyDown = (event: ReactKeyboardEvent<HTMLImageElement>): void => {
          if (event.key === "Enter" || event.key === " ") {
            event.preventDefault();
            handleOpen();
          }
        };
        return (
          <div className="message-image-frame" key={`${image.media_type}-${index}`}>
            <img
              className="message-image"
              src={src}
              alt={label}
              role="button"
              tabIndex={0}
              aria-label={t("composer.enlargeNamed", { name: label })}
              onClick={handleOpen}
              onKeyDown={handleKeyDown}
            />
            {onRemove ? (
              <button
                type="button"
                className="message-image-remove"
                aria-label={t("composer.removeImage", { number: index + 1 })}
                title={t("common.remove")}
                onClick={(event) => {
                  event.stopPropagation();
                  onRemove(index);
                }}
              >
                <X size={12} aria-hidden="true" />
              </button>
            ) : null}
          </div>
        );
      })}
    </div>
  );
}

export function MessageFileList({
  files,
  onRemove,
}: {
  files: InputFile[];
  /** When provided, each file gets a remove button (used inside the inline editor). */
  onRemove?: (index: number) => void;
}): JSX.Element {
  const { t } = useI18n();
  return (
    <div className={`message-files${onRemove ? " message-files-editable" : ""}`}>
      {files.map((file, index) => (
        <div className="message-file-frame" key={`${file.media_type}-${file.filename ?? index}-${index}`}>
          <div className="message-file">
            <FileText className="icon" aria-hidden="true" />
            <span>{file.filename?.trim() || t("message.fileNumber", { number: index + 1 })}</span>
          </div>
          {onRemove ? (
            <button
              type="button"
              className="message-file-remove"
              aria-label={t("message.removeFileNamed", { name: file.filename?.trim() || index + 1 })}
              title={t("common.remove")}
              onClick={() => onRemove(index)}
            >
              <X size={12} aria-hidden="true" />
            </button>
          ) : null}
        </div>
      ))}
    </div>
  );
}
