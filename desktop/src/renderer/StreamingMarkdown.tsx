import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState
} from "react";
import {
  MarkdownContent,
  type RichTextRenderer,
} from "./RichContent";
import {
  streamTextStore,
  useStreamedTextHasValue,
  useStreamedText
} from "./StreamText";
import { useConversationRenderActive } from "./ConversationRenderActivity";

/**
 * Progressive Markdown renderer used while assistant text is arriving.
 *
 * Single source of truth: the parent owns `isLive`, derived from the
 * back-end thread item. We do not maintain an internal
 * streaming/settling/settled state machine — `isLive` flips the renderer
 * between two modes:
 *   - `isLive=true`: server-streamed chunks render on the store's coalesced
 *                    cadence with a stable cursor marking the output edge.
 *   - `isLive=false`: text remains rendered in full. The cursor fades out
 *                     and `onSettled` fires once the final snapshot lands.
 *
 * `phase` is accepted so callers can pass the same semantic state they use
 * elsewhere, but typography and streaming affordances stay stable across
 * commentary and final-answer text.
 */
type StreamingMarkdownProps = {
  streamKey: string;
  initialText?: string;
  cwd?: string;
  onOpenFile?: (path: string) => void;
  className?: string;
  /** Whether the source item is still receiving deltas. */
  isLive: boolean;
  /**
   * The thread item phase. It is semantic metadata for the parent layout;
   * this renderer keeps commentary and final-answer text visually identical.
   */
  phase: "commentary" | "final_answer";
  onFrame?: () => void;
  onSettled?: () => void;
};

type StreamPhase = "streaming" | "settled";

const DEFAULT_CLASS_NAME = "streaming-markdown rich-content";
const CURSOR_CLASS_NAME = "stream-cursor";
const CURSOR_BLOCK_TAIL_CLASS_NAME = "stream-cursor-block-tail";
const CURSOR_SENTINEL = "";
const CURSOR_MARKDOWN_BOUNDARY = " ";
const CURSOR_PULSE_MAX_DELTA = 64;
const REDUCED_MOTION_QUERY = "(prefers-reduced-motion: reduce)";

export function StreamingMarkdown({
  streamKey,
  initialText = "",
  cwd,
  onOpenFile,
  className = DEFAULT_CLASS_NAME,
  isLive,
  onFrame,
  onSettled
}: StreamingMarkdownProps): JSX.Element {
  /* ------------------------- External store wiring ------------------------ */
  const renderActive = useConversationRenderActive();
  const hasStreamValue = useStreamedTextHasValue(streamKey, renderActive);
  const targetText = useStreamedText(streamKey, initialText, renderActive);

  /* ----------------------------- Sticky text ------------------------------ */
  // The text we actually render. The store may be cleared (in `onSettled`)
  // before the parent unmounts us, so the hook falls back to `initialText`
  // instead of blanking the visible message.
  const [renderedText, setRenderedText] = useState(
    isLive ? initialText : targetText,
  );
  const renderedReplacementVersionRef = useRef(
    streamTextStore.replacementVersion(streamKey),
  );
  const acceptedStreamValueRef = useRef(hasStreamValue);
  useLayoutEffect(() => {
    const replacementVersion = streamTextStore.replacementVersion(streamKey);
    if (hasStreamValue) {
      acceptedStreamValueRef.current = true;
    }
    if (targetText !== renderedText) {
      if (
        acceptedStreamValueRef.current &&
        !hasStreamValue &&
        targetText.length < renderedText.length
      ) {
        return;
      }
      renderedReplacementVersionRef.current = replacementVersion;
      setRenderedText(targetText);
      return;
    }
    if (hasStreamValue) {
      renderedReplacementVersionRef.current = replacementVersion;
    }
  }, [hasStreamValue, renderedText, streamKey, targetText]);

  /* ------------------------------ Phase ----------------------------------- */
  // Single internal phase: streaming while upstream is live, settled once
  // it isn't. The back-end message phase never gates rendering of the text.
  const phase: StreamPhase = isLive ? "streaming" : "settled";

  /* ------------------------------- Refs ---------------------------------- */
  const surfaceRef = useRef<HTMLDivElement | null>(null);
  const previousRenderedTextRef = useRef(renderedText);
  const cursorPulseAnimationRef = useRef<Animation | null>(null);
  const onFrameRef = useRef(onFrame);
  const onSettledRef = useRef(onSettled);
  const settledNotifiedRef = useRef(false);

  /* ----------------------- Refs always track props ------------------------ */
  useLayoutEffect(() => {
    onFrameRef.current = onFrame;
    onSettledRef.current = onSettled;
  }, [onFrame, onSettled]);

  /* -------------------------- Settle notification ------------------------ */
  // Fire `onSettled` once the upstream is no longer live AND the visible
  // cursor has caught up to the target text. The parent uses this to drop
  // any external "live" tracking (we don't manage it ourselves anymore).
  const trySettle = useCallback((): void => {
    if (settledNotifiedRef.current) return;
    settledNotifiedRef.current = true;
    onSettledRef.current?.();
  }, []);

  // The store already coalesces provider deltas to one notification per
  // visual update. Rendering those committed chunks directly avoids a
  // second client-side character chase that used to keep React and Markdown
  // busy for seconds after the provider had already delivered the text.
  useLayoutEffect(() => {
    if (renderActive) {
      onFrameRef.current?.();
    }
  }, [renderActive, renderedText]);

  useEffect(() => {
    if (isLive) {
      settledNotifiedRef.current = false;
      return;
    }
    cursorPulseAnimationRef.current?.cancel();
    cursorPulseAnimationRef.current = null;
    trySettle();
  }, [isLive, trySettle]);

  /* ----------------------- Cursor arrival feedback ----------------------- */
  // Streamed prose becomes fully legible immediately. Only the stable cursor
  // receives a short arrival cue, keeping previously rendered lines visually
  // inert while rapid wrapping and auto-follow move them through the viewport.
  // Dense batches and line breaks already carry enough motion, so they skip
  // the pulse rather than stacking another signal on top.
  useLayoutEffect(() => {
    const previousText = previousRenderedTextRef.current;
    previousRenderedTextRef.current = renderedText;
    cursorPulseAnimationRef.current?.cancel();
    cursorPulseAnimationRef.current = null;

    if (
      !renderActive ||
      !isLive ||
      !renderedText.startsWith(previousText) ||
      renderedText.length <= previousText.length
    ) {
      return;
    }

    const appendedText = renderedText.slice(previousText.length);
    if (
      appendedText.length > CURSOR_PULSE_MAX_DELTA ||
      appendedText.includes("\n") ||
      window.matchMedia?.(REDUCED_MOTION_QUERY).matches
    ) {
      return;
    }

    const cursor = surfaceRef.current?.querySelector<HTMLElement>(
      `.${CURSOR_CLASS_NAME}`,
    );
    if (!cursor || typeof cursor.animate !== "function") {
      return;
    }
    cursorPulseAnimationRef.current = cursor.animate(
      [
        { opacity: 0.68, transform: "scaleX(1)" },
        { opacity: 1, transform: "scaleX(1.45)", offset: 0.35 },
        { opacity: 1, transform: "scaleX(1)" },
      ],
      {
        duration: 140,
        easing: "cubic-bezier(0.16, 1, 0.3, 1)",
      },
    );
  }, [isLive, renderActive, renderedText]);

  useEffect(() => () => {
    cursorPulseAnimationRef.current?.cancel();
  }, []);

  /* ------------------------- Derived view data -------------------------- */
  const visibleText = renderedText;
  const cursorState = isLive ? "shown" : "fading";
  // The cursor appears for all live assistant text. Commentary and final
  // answers share the same visual treatment so a later phase resolution does
  // not cause a typography or affordance jump.
  // Always render the cursor span so the fold body height stays stable
  // when the cursor fades out. Removing the
  // cursor from DOM shrinks scrollHeight by ~1 line (1.05em), which
  // clamps scrollTop in ConversationScrollState and creates a visible
  // UP shift that combines with the next item's auto-follow re-anchor
  // into a V-shape jitter. Visibility is controlled by the parent
  // data-cursor-state attribute (see turns.css) instead.
  const showCursor = true;
  const cursorTextRenderer = useMemo(() => createCursorTextRenderer(), []);
  // Mermaid is expensive; do not flip the markdown renderer at settle for
  // ordinary text. Only messages that actually contain a Mermaid fence enter
  // the diagram renderer after streaming ends.
  const renderMermaid =
    phase === "settled" && containsMermaidFence(visibleText);

  // Split the visible text into stable blocks + an open tail. Every
  // stable block is its own memoized markdown surface, so promoting a
  // new block from the tail only costs that single block's parse — the
  // earlier blocks stay mounted as-is. This caps per-tick work at
  // O(tail) and per-promotion work at O(one block), independent of the
  // total answer length.
  //
  // The settled phase uses the same split as streaming: keeping the
  // block layout stable across the streaming → settled transition means
  // React reconciles by updating props in place instead of unmounting
  // every previously memoized block and remounting one big tail. That
  // reconciliation jump is what caused the visible "settle flick" on
  // long answers (block-level memo would be wiped the instant the
  // upstream went idle).
  const split = useIncrementalStableBlocks(
    visibleText,
    streamKey,
    renderedReplacementVersionRef.current,
    streamTextStore.has(streamKey),
  );
  // Keep the synthetic cursor separated from the Markdown source. Appending
  // the private-use sentinel directly after a closing emphasis delimiter can
  // make that delimiter non-right-flanking (for example `。**`), exposing
  // the raw `**` in the rendered message. The text renderer removes this
  // parsing-only boundary before it reaches the DOM.
  //
  // Some completed blocks have no inline tail for the sentinel. A closed
  // fence is one example; prose ending in blank lines is another because the
  // complete paragraph has already moved into `split.blocks`. Appending the
  // sentinel to either tail parses it as a new empty paragraph, leaving a
  // permanent blank row after a historical message. Keep the cursor as the
  // existing zero-flow-height sibling in both cases — same settle-stable DOM
  // slot, no reserved vertical space.
  const cursorNeedsBlockTail = showCursor && (
    endsWithFenceCloser(split.tail) ||
    (split.blocks.length > 0 && split.tail.trim().length === 0)
  );
  const tailText = showCursor && !cursorNeedsBlockTail
    ? `${split.tail}${CURSOR_MARKDOWN_BOUNDARY}${CURSOR_SENTINEL}`
    : split.tail;

  /* ------------------------------- Render -------------------------------- */
  return (
    <div
      ref={surfaceRef}
      className={className}
      data-stream-state={phase}
      data-cursor-state={cursorState}
    >
      {split.blocks.map((block, index) => (
        // Keep stable blocks keyed separately so settled text does not remount
        // into one large markdown tree when streaming ends.
        <div className="streaming-markdown-block" key={index}>
          <MemoMarkdownContent
            text={block}
            cwd={cwd}
            onOpenFile={onOpenFile}
            renderMermaid={renderMermaid}
          />
        </div>
      ))}
      <MarkdownContent
        text={tailText}
        cwd={cwd}
        onOpenFile={onOpenFile}
        renderText={cursorTextRenderer}
        renderMermaid={renderMermaid}
      />
      {cursorNeedsBlockTail ? (
        <span
          className={`${CURSOR_CLASS_NAME} ${CURSOR_BLOCK_TAIL_CLASS_NAME}`}
          aria-hidden="true"
        />
      ) : null}
    </div>
  );
}

/**
 * Memoized markdown surface. Stable blocks are passed in by value;
 * React.memo's default shallow compare on the `text` string is exactly
 * what we want — identical text means identical render.
 */
const MemoMarkdownContent = MarkdownContent;

function createCursorTextRenderer(): RichTextRenderer {
  return (text, keyPrefix) => {
    const cursorIndex = text.indexOf(CURSOR_SENTINEL);
    const textBeforeCursor = cursorIndex >= 0 ? text.slice(0, cursorIndex) : text;
    const visibleText = cursorIndex >= 0 && textBeforeCursor.endsWith(CURSOR_MARKDOWN_BOUNDARY)
      ? textBeforeCursor.slice(0, -CURSOR_MARKDOWN_BOUNDARY.length)
      : textBeforeCursor;
    const output: Array<JSX.Element | string> = visibleText ? [visibleText] : [];

    if (cursorIndex >= 0) {
      output.push(
        <span
          key={`${keyPrefix}-cursor`}
          className={CURSOR_CLASS_NAME}
          aria-hidden="true"
        />
      );
      const trailingText = text.slice(cursorIndex + CURSOR_SENTINEL.length);
      if (trailingText) {
        output.push(trailingText);
      }
    }
    return output;
  };
}

export function containsMermaidFence(text: string): boolean {
  return /(^|\n)```[ \t]*mermaid[ \t]*\r?\n/i.test(text);
}

function endsWithFenceCloser(text: string): boolean {
  if (!text || text.endsWith("\n")) {
    return false;
  }

  // Most snapshots do not end on a fence marker. Check only the final line
  // first so ordinary prose does not allocate and scan every preceding line.
  const finalLineStart = text.lastIndexOf("\n") + 1;
  const finalLine = text.slice(finalLineStart);
  const finalMatch = /^ {0,3}(`{3,}|~{3,})(.*)$/.exec(finalLine);
  if (!finalMatch || finalMatch[2].trim() !== "") {
    return false;
  }

  const finalMarker = finalMatch[1][0] as "`" | "~";
  const finalMarkerLength = finalMatch[1].length;
  let activeFence: { marker: "`" | "~"; length: number } | undefined;
  const precedingLines = text.slice(0, finalLineStart).split("\n");
  for (const line of precedingLines) {
    const match = /^ {0,3}(`{3,}|~{3,})(.*)$/.exec(line);
    if (!match) {
      continue;
    }

    const marker = match[1][0] as "`" | "~";
    if (!activeFence) {
      if (marker === "`" && match[2].includes("`")) {
        continue;
      }
      activeFence = { marker, length: match[1].length };
      continue;
    }

    const isCloser =
      marker === activeFence.marker &&
      match[1].length >= activeFence.length &&
      match[2].trim() === "";
    if (!isCloser) {
      continue;
    }
    activeFence = undefined;
  }
  return Boolean(
    activeFence &&
    activeFence.marker === finalMarker &&
    finalMarkerLength >= activeFence.length,
  );
}

type StableBlockSplit = { blocks: string[]; tail: string };

type StableBlockScanState = StableBlockSplit & {
  sourceKey: string;
  replacementVersion: number;
  textLength: number;
  scanOffset: number;
  blockStart: number;
  inFence: boolean;
};

function useIncrementalStableBlocks(
  text: string,
  sourceKey: string,
  replacementVersion: number,
  appendOnly: boolean,
): StableBlockSplit {
  const committedScanRef = useRef<StableBlockScanState | undefined>(undefined);
  const scan = useMemo(() => {
    const previous = committedScanRef.current;
    const canExtend =
      appendOnly &&
      previous?.sourceKey === sourceKey &&
      previous.replacementVersion === replacementVersion &&
      text.length >= previous.textLength;
    return scanStableBlocks(
      text,
      sourceKey,
      replacementVersion,
      canExtend ? previous : undefined,
    );
  }, [appendOnly, replacementVersion, sourceKey, text]);

  useLayoutEffect(() => {
    committedScanRef.current = scan;
  }, [scan]);

  return scan;
}

function scanStableBlocks(
  text: string,
  sourceKey = "",
  replacementVersion = 0,
  previous?: StableBlockScanState,
): StableBlockScanState {
  let blocks = previous?.blocks ?? [];
  const previousBlocks = blocks;
  let blocksCopied = false;
  let inFence = previous?.inFence ?? false;
  let blockStart = previous?.blockStart ?? 0;
  let scanOffset = previous?.scanOffset ?? 0;

  // Only complete lines are scanned. The final partial line may still grow
  // into a fence opener/closer, so deferring it avoids rescanning or rolling
  // back parser state when the next provider chunk arrives.
  for (;;) {
    const lineEnd = text.indexOf("\n", scanOffset);
    if (lineEnd < 0) {
      break;
    }
    const lineStart = scanOffset;
    const startsBacktickFence =
      lineEnd - lineStart >= 3 &&
      text.charCodeAt(lineStart) === 96 &&
      text.charCodeAt(lineStart + 1) === 96 &&
      text.charCodeAt(lineStart + 2) === 96;
    if (startsBacktickFence) {
      if (!inFence) {
        inFence = true;
      } else {
        let isCloser = true;
        for (let index = lineStart + 3; index < lineEnd; index += 1) {
          const code = text.charCodeAt(index);
          if (code !== 32 /* space */ && code !== 9 /* tab */) {
            isCloser = false;
            break;
          }
        }
        if (isCloser) {
          inFence = false;
        }
      }
    }

    scanOffset = lineEnd + 1;
    if (!inFence && lineEnd === lineStart) {
      if (!blocksCopied && blocks === previousBlocks && previous) {
        blocks = [...blocks];
        blocksCopied = true;
      }
      blocks.push(text.slice(blockStart, scanOffset));
      blockStart = scanOffset;
    }
  }

  return {
    sourceKey,
    replacementVersion,
    textLength: text.length,
    scanOffset,
    blockStart,
    inFence,
    blocks,
    tail: text.slice(blockStart),
  };
}

/**
 * Split `text` into a sequence of "stable" markdown blocks plus an
 * open tail. A block is everything between two blank-line boundaries
 * (`\n\n`). Blocks inside an unclosed fenced code section are deferred
 * to the tail — they aren't yet stable.
 *
 * Each stable block has self-contained markdown semantics: prepending
 * or appending more text to the overall document cannot change how the
 * block parses. That property is what makes block-level memoization
 * safe.
 *
 * Exported for tests.
 */
export function splitIntoStableBlocks(
  text: string
): StableBlockSplit {
  const { blocks, tail } = scanStableBlocks(text);
  return { blocks, tail };
}
