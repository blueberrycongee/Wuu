import { Bot, ChevronDown, ChevronUp, ClipboardList, ImagePlus, MessageCircle, Network, PanelLeftClose, PanelLeftOpen, Plus, Reply, Settings2, X } from "lucide-react";
import { type CSSProperties, type KeyboardEvent, type PointerEvent as ReactPointerEvent, useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import ReactMarkdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";
import remarkCjkStrongBoundary from "./remarkCjkStrongBoundary";
import type { ChannelAgentInsight, ChannelMessage, ChannelRoom, EngineInfo, InitializeResult, NamedAgent } from "../shared/protocol";
import { AgentAvatarMark, randomAgentAvatarKey } from "./AgentAvatarMark";
import { AgentAvatarCreator } from "./AgentAvatarCreator";
import { AgentRelationshipGraph } from "./AgentRelationshipGraph";
import { AUTO_FOLLOW_BOTTOM_THRESHOLD_PX, useAutoFollowScrollContainer } from "./AutoFollowScroll";
import { ChannelComposer, type ChannelComposerHandle } from "./ChannelComposer";
import { ChannelGroupAvatar } from "./ChannelGroupAvatar";
import { ChannelMemberPicker } from "./ChannelMemberPicker";
import { buildComposerAttachments } from "./ComposerDraftState";
import { ComposerAttachmentStrip } from "./ComposerInputSections";
import { HumanAvatarMark } from "./DefaultAvatar";
import { FieldError } from "./FieldError";
import {
  awaitComposerImages,
  inputFilesFromComposer,
  inputImagesFromComposer,
  type ComposerFile,
  type ComposerImage,
} from "./ComposerMessages";
import { useI18n } from "./i18n";
import { JumpToLatestPill } from "./JumpToLatestPill";
import { useLongTextCollapse } from "./LongTextCollapse";
import { MessageBubble, MessageBubbleRow } from "./MessageBubbleFlow";
import { SelectMenu, type SelectMenuGroup } from "./SelectMenu";
import { SidebarNameDialog } from "./SidebarNameDialog";
import { RichContent } from "./RichContent";
import { effortLabel, providerModelEffortOptions } from "./RuntimeHelpers";
import { showErrorToast, toastErrorMessage } from "./Toast";

type SetupPanel = "agent" | "room" | "task" | null;
type RoomMemberMode = "add" | null;

type AgentDetailDraft = {
  name: string;
  role: string;
  avatarKey: string;
  avatarImage: string;
  engine: string;
  model: string;
  effort: string;
};
export type ChannelSection = "rooms" | "agents" | "tasks";
type AgentActivityStatus = "idle" | "thinking" | "sending";

const CHANNEL_SPLIT_WIDTH_KEY = "wuu.channels.splitPaneWidth";
const LEGACY_CHANNEL_LIST_WIDTH_KEY = "wuu.channels.listWidth";
const CHANNEL_LIST_COLLAPSED_KEY = "wuu.channels.listCollapsed";
const CHANNEL_SPLIT_MIN_WIDTH = 156;
const CHANNEL_SPLIT_MAX_WIDTH = 360;
const CHANNEL_SPLIT_DEFAULT_WIDTH = 208;
const CHANNEL_SPLIT_COLLAPSED_WIDTH = 44;
const CHANNEL_SPLIT_WIDTH_STEP = 16;
const THREAD_PANEL_WIDTH_KEY = "wuu.channels.threadPanelWidth";
const THREAD_PANEL_MIN_WIDTH = 320;
const MAX_ROOM_AGENTS = 6;
const THREAD_PANEL_MAX_WIDTH = 720;
const THREAD_PANEL_DEFAULT_WIDTH = 420;
const THREAD_PANEL_WIDTH_STEP = 24;
const AGENT_AVATAR_SOURCE_MAX_BYTES = 10 * 1024 * 1024;
const AGENT_AVATAR_SIZE = 256;
const AGENT_AVATAR_TYPES = new Set(["image/png", "image/jpeg", "image/webp"]);

export function formatChannelUnreadCount(count: number): string {
  return count > 99 ? "99+" : String(Math.max(0, count));
}

async function squareAvatarImageFromFile(file: File): Promise<string> {
  if (!AGENT_AVATAR_TYPES.has(file.type) || file.size === 0 || file.size > AGENT_AVATAR_SOURCE_MAX_BYTES) throw new Error("invalid-avatar-image");
  const bitmap = await createImageBitmap(file);
  try {
    const canvas = document.createElement("canvas");
    canvas.width = AGENT_AVATAR_SIZE;
    canvas.height = AGENT_AVATAR_SIZE;
    const context = canvas.getContext("2d");
    if (!context) throw new Error("invalid-avatar-image");
    const scale = Math.max(AGENT_AVATAR_SIZE / bitmap.width, AGENT_AVATAR_SIZE / bitmap.height);
    const width = bitmap.width * scale;
    const height = bitmap.height * scale;
    context.drawImage(bitmap, (AGENT_AVATAR_SIZE - width) / 2, (AGENT_AVATAR_SIZE - height) / 2, width, height);
    return canvas.toDataURL("image/webp", 0.86);
  } finally {
    bitmap.close();
  }
}

function AgentAvatar({ id, name, avatarKey, avatarImage, status, statusText, model, modelLabel, compact = false, expressive = false }: {
  id: string;
  name: string;
  avatarKey: string;
  avatarImage?: string;
  status: AgentActivityStatus;
  statusText: string;
  model?: string;
  modelLabel: string;
  compact?: boolean;
  expressive?: boolean;
}): JSX.Element {
  const accessibleDescription = model ? `${name}: ${statusText}, ${modelLabel}: ${model}` : `${name}: ${statusText}`;
  return (
    <span className={`channel-agent-avatar${compact ? " compact" : ""}`} tabIndex={0} aria-label={accessibleDescription}>
      <AgentAvatarMark seed={id} avatarKey={avatarKey} avatarImage={avatarImage} status={expressive ? status : "idle"} />
      <span className={`channel-agent-status-dot ${status}`} aria-hidden="true" />
      <span className="channel-agent-status-card" role="tooltip">
        <span><i className={`channel-agent-status-swatch ${status}`} />{statusText}</span>
        {model ? <span className="channel-agent-model">{model}</span> : null}
      </span>
    </span>
  );
}

function ChannelAuthorName({ name, mentionLabel, onMention }: {
  name: string;
  mentionLabel?: string;
  onMention?: () => void;
}): JSX.Element {
  if (!onMention) return <strong>{name}</strong>;
  return (
    <button className="channel-author-mention" type="button" aria-label={mentionLabel} onClick={onMention}>
      <span aria-hidden="true">@</span>
      {name}
    </button>
  );
}

const CHANNEL_THREAD_DIGEST_MAX_REPLIES = 3;

const CHANNEL_THREAD_PREVIEW_COMPONENTS = {
  a({ children }) { return <>{children}</>; },
  blockquote({ children }) { return <>{children} </>; },
  br() { return <> </>; },
  code({ children }) { return <>{children}</>; },
  del({ children }) { return <>{children}</>; },
  em({ children }) { return <>{children}</>; },
  h1({ children }) { return <>{children} </>; },
  h2({ children }) { return <>{children} </>; },
  h3({ children }) { return <>{children} </>; },
  h4({ children }) { return <>{children} </>; },
  h5({ children }) { return <>{children} </>; },
  h6({ children }) { return <>{children} </>; },
  hr() { return <> </>; },
  img({ alt }) { return <>{alt ?? ""}</>; },
  input() { return null; },
  li({ children }) { return <>{children} </>; },
  ol({ children }) { return <>{children}</>; },
  p({ children }) { return <>{children} </>; },
  pre({ children }) { return <>{children} </>; },
  strong({ children }) { return <>{children}</>; },
  table({ children }) { return <>{children} </>; },
  tbody({ children }) { return <>{children}</>; },
  td({ children }) { return <>{children} </>; },
  th({ children }) { return <>{children} </>; },
  thead({ children }) { return <>{children}</>; },
  tr({ children }) { return <>{children} </>; },
  ul({ children }) { return <>{children}</>; },
} satisfies Components;

function ChannelThreadMarkdownPreview({ text }: { text: string }): JSX.Element {
  return (
    <ReactMarkdown
      components={CHANNEL_THREAD_PREVIEW_COMPONENTS}
      remarkPlugins={[remarkGfm, remarkCjkStrongBoundary]}
    >
      {text}
    </ReactMarkdown>
  );
}

function ChannelThreadDigest({
  replies,
  agents,
  onOpen,
  onMention,
}: {
  replies: ChannelMessage[];
  agents: NamedAgent[];
  onOpen: () => void;
  onMention: (name: string) => void;
}): JSX.Element {
  const { formatDate, t } = useI18n();
  const visibleReplies = replies.slice(-CHANNEL_THREAD_DIGEST_MAX_REPLIES);

  return (
    <div className="channel-thread-digest">
      <button
        className="channel-thread-digest-open"
        type="button"
        aria-label={t("channels.replyCount", { count: replies.length })}
        onClick={onOpen}
      />
      {replies.length > CHANNEL_THREAD_DIGEST_MAX_REPLIES ? (
        <span className="channel-thread-digest-heading">
          <strong>{t("channels.replyCount", { count: replies.length })}</strong>
        </span>
      ) : null}
      <span className="channel-thread-digest-rows">
        {visibleReplies.map((reply) => {
          const own = reply.author_type === "human";
          const agent = own
            ? undefined
            : agents.find((candidate) => candidate.id === reply.author_id);
          const author = own ? t("channels.you") : (agent?.name ?? reply.author_id);
          const markdownPreview = reply.body.trim();
          const fallbackPreview = reply.files?.[0]?.filename
            || (reply.images?.length
              ? t("appState.images", { count: reply.images.length })
              : "");
          return (
            <span className="channel-thread-digest-row" key={reply.id}>
              <span className="channel-thread-digest-avatar" aria-hidden="true">
                {own ? (
                  <HumanAvatarMark />
                ) : (
                  <AgentAvatarMark
                    seed={reply.author_id}
                    avatarKey={agent?.avatar_key ?? "abstract-1"}
                    avatarImage={agent?.avatar_image}
                  />
                )}
              </span>
              <ChannelAuthorName
                name={author}
                mentionLabel={!own ? t("channels.mentionAgent", { name: author }) : undefined}
                onMention={!own ? () => onMention(author) : undefined}
              />
              <span className="channel-thread-digest-preview">
                {markdownPreview ? (
                  <ChannelThreadMarkdownPreview text={markdownPreview} />
                ) : fallbackPreview}
              </span>
              <time dateTime={reply.created_at}>
                {formatDate(reply.created_at, { hour: "2-digit", minute: "2-digit" })}
              </time>
            </span>
          );
        })}
      </span>
    </div>
  );
}

type ChannelTimelineItem =
  | { kind: "message"; message: ChannelMessage }
  | { kind: "orchestration"; tasks: ChannelMessage[] };

function buildChannelTimeline(messages: ChannelMessage[]): ChannelTimelineItem[] {
  const timeline: ChannelTimelineItem[] = [];
  for (const message of messages) {
    if (message.thread_id) continue;
    // Tasks are Work/activity facts, not messages authored by a hidden
    // coordinator. Render every root task as an owner-facing Work Card.
    if (message.kind === "task") {
      const previous = timeline[timeline.length - 1];
      const previousTask = previous?.kind === "orchestration" ? previous.tasks[previous.tasks.length - 1] : undefined;
      if (previous?.kind === "orchestration" && previousTask && previousTask.seq + 1 === message.seq) {
        previous.tasks.push(message);
      } else {
        timeline.push({ kind: "orchestration", tasks: [message] });
      }
      continue;
    }
    timeline.push({ kind: "message", message });
  }
  return timeline;
}

export function assignmentState(state?: string): "open" | "doing" | "checking" | "revising" | "needs_human" | "done" {
  if (state === "checking") return "checking";
  if (state === "revising") return "revising";
  if (state === "needs_human") return "needs_human";
  if (state === "doing") return "doing";
  if (state === "done") return "done";
  return "open";
}

function assignmentStatusKey(state: ReturnType<typeof assignmentState>): "open" | "doing" | "checking" | "revising" | "needsHuman" | "done" {
  return state === "needs_human" ? "needsHuman" : state;
}

function ChannelOrchestrationCluster({
  room,
  tasks,
  agents,
  repliesByThread,
  onOpenThread,
  onOpenSession,
  onMention,
}: {
  room: ChannelRoom;
  tasks: ChannelMessage[];
  agents: NamedAgent[];
  repliesByThread: Map<string, ChannelMessage[]>;
  onOpenThread: (messageID: string) => void;
  onOpenSession?: (sessionID: string) => void;
  onMention: (name: string) => void;
}): JSX.Element {
  const { formatDate, t } = useI18n();
  const agentByID = useMemo(() => new Map(agents.map((agent) => [agent.id, agent])), [agents]);
  const ownerIDs = [...new Set(tasks.map((task) => task.task_owner).filter(Boolean))] as string[];
  const doneCount = tasks.filter((task) => assignmentState(task.task_state) === "done").length;
  const allDone = tasks.length > 0 && doneCount === tasks.length;
  const [expanded, setExpanded] = useState(!allDone);
  const previousAllDone = useRef(allDone);
  const latestTask = tasks[tasks.length - 1];

  useEffect(() => {
    if (allDone !== previousAllDone.current) setExpanded(!allDone);
    previousAllDone.current = allDone;
  }, [allDone]);

  const coordinationLabel = allDone
    ? t("channels.coordinationComplete", { count: ownerIDs.length, done: doneCount })
    : t("channels.coordinatingAgents", { count: ownerIDs.length });

  return (
      <MessageBubbleRow
        outgoing={false}
        className={`channel-message channel-orchestration-message channel-orchestration-row${allDone ? " complete" : ""}`}
        contentClassName="channel-message-content"
        meta={(
          <div className="channel-message-meta">
            <span className="channel-orchestration-meta">
              <strong>{t("channels.tasks")}</strong>
              <span className="channel-coordinator-state">{coordinationLabel}</span>
              <time dateTime={latestTask.created_at}>
                {formatDate(latestTask.created_at, { hour: "2-digit", minute: "2-digit" })}
              </time>
              {allDone && expanded ? (
                <button className="channel-orchestration-toggle" type="button" onClick={() => setExpanded(false)}>
                  {t("channels.hideAssignments")}
                  <ChevronUp aria-hidden="true" />
                </button>
              ) : null}
            </span>
          </div>
        )}
      >
        {allDone && !expanded ? (
          <MessageBubble outgoing={false} className="channel-message-bubble channel-orchestration-summary">
            <button type="button" onClick={() => setExpanded(true)}>
              <span className="channel-orchestration-owner-stack" aria-hidden="true">
                {ownerIDs.slice(0, 3).map((ownerID) => {
                  const owner = agentByID.get(ownerID);
                  return (
                    <span key={ownerID}>
                      <AgentAvatarMark
                        seed={ownerID}
                        avatarKey={owner?.avatar_key ?? "abstract-1"}
                        avatarImage={owner?.avatar_image}
                      />
                    </span>
                  );
                })}
              </span>
              <span>{coordinationLabel}</span>
              <ChevronDown aria-hidden="true" />
            </button>
          </MessageBubble>
        ) : (
          <div className="channel-assignment-list">
            {tasks.map((task, index) => {
              const owner = agentByID.get(task.task_owner ?? "");
              const ownerName = owner?.name ?? task.task_owner ?? t("channels.taskOwnerLabel");
              const work = task.work;
              const state = assignmentState(task.task_state);
              const workState = work?.state ?? state;
              const statusLabel = workState === "cancelled"
                ? t("channels.workStatus.cancelled")
                : workState === "failed"
                  ? t("channels.workStatus.failed")
                  : workState === "interrupted"
                    ? t("channels.workStatus.interrupted")
                    : workState === "integrating"
                      ? t("channels.workStatus.integrating")
                      : t(`channels.assignmentStatus.${assignmentStatusKey(state)}`);
              const activity = (work?.events ?? [])
                .filter((event) => event.kind === "state" && event.state)
                .map((event) => event.state)
                .filter((eventState, eventIndex, values) => eventIndex === 0 || eventState !== values[eventIndex - 1]);
              const elapsedMilliseconds = work
                ? Math.max(0, Date.parse(work.updated_at) - Date.parse(work.created_at))
                : 0;
              const elapsedMinutes = Math.max(1, Math.round(elapsedMilliseconds / 60_000));
              const elapsed = elapsedMinutes >= 60
                ? `${Math.floor(elapsedMinutes / 60)}h ${elapsedMinutes % 60}m`
                : `${elapsedMinutes}m`;
              const hasEvidence = Boolean(
                work?.checks_summary
                || work?.changed_files_count
                || work?.verification?.report
                || work?.artifacts?.length
                || work?.runs?.length
                || work?.deliveries?.length
                || work?.unresolved_items,
              );
              const title = task.task_title?.trim() || task.body.trim() || t("channels.newTask");
              const body = task.task_title?.trim() && task.body.trim() !== task.task_title.trim()
                ? task.body.trim()
                : "";
              const replies = repliesByThread.get(task.id) ?? [];
              return (
                <div
                  className="channel-assignment-item"
                  data-state={workState}
                  key={task.id}
                  style={{ "--channel-assignment-index": index } as CSSProperties}
                >
                  <MessageBubble outgoing={false} className="channel-message-bubble channel-assignment-bubble">
                    <span className="channel-assignment-target" aria-hidden="true">
                      <AgentAvatarMark
                        seed={owner?.id ?? task.task_owner ?? task.id}
                        avatarKey={owner?.avatar_key ?? "abstract-1"}
                        avatarImage={owner?.avatar_image}
                        status={owner?.activity_status === "thinking" ? "thinking" : "idle"}
                      />
                    </span>
                    <div className="channel-assignment-copy">
                      <span className="channel-assignment-owner"><span aria-hidden="true">→</span>{ownerName}</span>
                      <strong>{title}</strong>
                      {body ? <div className="channel-assignment-body"><RichContent text={body} /></div> : null}
                    </div>
                    <span className="channel-assignment-status" data-state={workState}>
                      <i aria-hidden="true" />
                      {statusLabel}
                    </span>
                    <button
                      className="channel-assignment-thread-button"
                      type="button"
                      aria-label={t("channels.openAssignmentThread", { title })}
                      onClick={() => onOpenThread(task.id)}
                    >
                      <MessageCircle aria-hidden="true" />
                    </button>
                  </MessageBubble>
                  {work ? (
                    <div className="channel-work-card-details">
                      {activity.length > 0 ? (
                        <div className="channel-work-activity" aria-label={activity.join(" → ")}>
                          {activity.join(" → ")}
                        </div>
                      ) : null}
                      <div className="channel-work-summary-line">
                        {work.checks_summary ? <span>{t("channels.workChecks", { summary: work.checks_summary })}</span> : null}
                        {work.changed_files_count ? <span>{t("channels.workFilesChanged", { count: work.changed_files_count })}</span> : null}
                        <span>{t("channels.workElapsed", { duration: elapsed })}</span>
                      </div>
                      {hasEvidence ? (
                        <details className="channel-work-evidence">
                          <summary>{t("channels.workEvidence")}</summary>
                          <div className="channel-work-evidence-body">
                            {work.verification?.report ? (
                              <section>
                                <strong>{t("channels.workVerifierReport")}</strong>
                                <RichContent text={work.verification.report} />
                              </section>
                            ) : null}
                            {work.artifacts?.length ? (
                              <section>
                                <strong>{t("channels.workArtifacts")}</strong>
                                <ul>{work.artifacts.map((artifact) => (
                                  <li key={artifact.id}><a href={artifact.uri}>{artifact.label || artifact.summary || artifact.kind}</a></li>
                                ))}</ul>
                              </section>
                            ) : null}
                            {work.runs?.length ? (
                              <section>
                                <strong>{t("channels.workRuns")}</strong>
                                <ul>{work.runs.map((run) => (
                                  <li key={run.id}>
                                    <span>{run.profile || run.kind} · {run.state}</span>
                                    {run.session_ref ? (
                                      <button className="channel-work-session-link" type="button" onClick={() => onOpenSession?.(run.session_ref ?? "")}>
                                        <code>{run.session_ref}</code>
                                      </button>
                                    ) : null}
                                  </li>
                                ))}</ul>
                              </section>
                            ) : null}
                            {work.deliveries?.length ? (
                              <section>
                                <strong>{t("channels.workPrivateMessages")}</strong>
                                <ul>{work.deliveries.map((delivery) => (
                                  <li key={delivery.id}>
                                    <span>{delivery.kind || "control"}{delivery.invalidated_at ? ` · ${t("channels.workMessageInvalidated")}` : ""}</span>
                                    <RichContent text={delivery.body} />
                                  </li>
                                ))}</ul>
                              </section>
                            ) : null}
                            {work.unresolved_items ? <p>{t("channels.workUnresolved", { items: work.unresolved_items })}</p> : null}
                          </div>
                        </details>
                      ) : null}
                    </div>
                  ) : null}
                  {replies.length > 0 ? (
                    <ChannelThreadDigest
                      replies={replies}
                      agents={agents}
                      onOpen={() => onOpenThread(task.id)}
                      onMention={onMention}
                    />
                  ) : null}
                </div>
              );
            })}
          </div>
        )}
      </MessageBubbleRow>
  );
}

function ChannelMessageBubble({
  message,
  outgoing,
  allowCollapse,
  onExpand,
  attachmentIDPrefix,
  beforeBody,
}: {
  message: ChannelMessage;
  outgoing: boolean;
  allowCollapse: boolean;
  onExpand?: () => void;
  attachmentIDPrefix: string;
  beforeBody?: JSX.Element;
}): JSX.Element {
  const { t } = useI18n();
  const { collapsible, expanded, toggleExpanded } = useLongTextCollapse(message.body);
  const canCollapse = allowCollapse && collapsible;
  const handleToggleExpanded = (): void => {
    if (!expanded) {
      onExpand?.();
    }
    toggleExpanded();
  };
  const hasBubble = Boolean(message.body || beforeBody);

  return (
    <>
      {hasBubble ? (
        <MessageBubble
          outgoing={outgoing}
          className={`channel-message-bubble${canCollapse ? ` long-card ${expanded ? "expanded" : "collapsed"}` : ""}`}
        >
          {beforeBody}
          {message.kind === "task" ? (
            <span className="channel-thread-task-heading">
              <strong>{message.task_title?.trim() || message.body.trim() || t("channels.newTask")}</strong>
              <span className="channel-assignment-status" data-state={assignmentState(message.task_state)}>
                <i aria-hidden="true" />
                {t(`channels.assignmentStatus.${assignmentStatusKey(assignmentState(message.task_state))}`)}
              </span>
            </span>
          ) : null}
          {message.body && (
            message.kind !== "task"
            || Boolean(message.task_title?.trim() && message.body.trim() !== message.task_title.trim())
          ) ? <RichContent text={message.body} /> : null}
          {canCollapse ? (
            <button
              className="channel-message-expand-toggle"
              type="button"
              aria-expanded={expanded}
              onClick={handleToggleExpanded}
            >
              <span>{expanded ? t("common.collapse") : t("common.showMore")}</span>
              {expanded ? <ChevronUp aria-hidden="true" /> : <ChevronDown aria-hidden="true" />}
            </button>
          ) : null}
        </MessageBubble>
      ) : null}
      {message.images?.length || message.files?.length ? (
        <ComposerAttachmentStrip
          images={(message.images ?? []).map((image, index) => ({ id: `${message.id}-${attachmentIDPrefix}-image-${index}`, ...image }))}
          files={(message.files ?? []).map((file, index) => ({ id: `${message.id}-${attachmentIDPrefix}-file-${index}`, ...file }))}
          removable={false}
        />
      ) : null}
    </>
  );
}

function ChannelMessageWithReplyAction({
  outgoing,
  onReply,
  children,
}: {
  outgoing: boolean;
  onReply: () => void;
  children: JSX.Element;
}): JSX.Element {
  const { t } = useI18n();
  return (
    <div className={`channel-message-bubble-line${outgoing ? " outgoing" : ""}`}>
      <div className="channel-message-body">{children}</div>
      <div className="channel-message-actions">
        <button type="button" onClick={onReply}>
          <Reply aria-hidden="true" />
          {t("channels.reply")}
        </button>
      </div>
    </div>
  );
}

function clampChannelSplitWidth(width: number): number {
  return Math.min(CHANNEL_SPLIT_MAX_WIDTH, Math.max(CHANNEL_SPLIT_MIN_WIDTH, Math.round(width)));
}

function initialChannelSplitWidth(): number {
  const storedValue = window.localStorage.getItem(CHANNEL_SPLIT_WIDTH_KEY) ?? window.localStorage.getItem(LEGACY_CHANNEL_LIST_WIDTH_KEY);
  const stored = Number(storedValue);
  return storedValue !== null && Number.isFinite(stored) && stored >= CHANNEL_SPLIT_MIN_WIDTH
    ? clampChannelSplitWidth(stored)
    : CHANNEL_SPLIT_DEFAULT_WIDTH;
}

function initialChannelListCollapsed(): boolean {
  return window.localStorage.getItem(CHANNEL_LIST_COLLAPSED_KEY) === "true";
}

function clampThreadPanelWidth(width: number): number {
  return Math.min(THREAD_PANEL_MAX_WIDTH, Math.max(THREAD_PANEL_MIN_WIDTH, Math.round(width)));
}

function initialThreadPanelWidth(): number {
  const storedValue = window.localStorage.getItem(THREAD_PANEL_WIDTH_KEY);
  const stored = Number(storedValue);
  return storedValue !== null && Number.isFinite(stored) && stored >= THREAD_PANEL_MIN_WIDTH
    ? clampThreadPanelWidth(stored)
    : THREAD_PANEL_DEFAULT_WIDTH;
}

function taskStateKey(state?: string): "channels.taskState.open" | "channels.taskState.doing" | "channels.taskState.done" {
  if (state === "doing") return "channels.taskState.doing";
  if (state === "done") return "channels.taskState.done";
  return "channels.taskState.open";
}

export function ChannelView({ initialized, engines = [], section = "rooms", archivedRoomIDs = [], onSectionChange, selectedRoomID: controlledRoomID, onSelectRoom, onRoomRead, onOpenMemoryDirectory, onOpenSession, composerDraft, onComposerDraftChange, newRoomRequest, onNewRoomRequestHandled, newAgentRequest, onNewAgentRequestHandled, editAgentRequestID, onEditAgentRequestHandled }: {
  initialized?: InitializeResult;
  engines?: EngineInfo[];
  section?: ChannelSection;
  archivedRoomIDs?: string[];
  onSectionChange?: (section: ChannelSection) => void;
  // Optional controlled room selection. App.tsx drives this so the unified
  // sidebar can select rooms; when absent the view manages selection
  // internally (tests and standalone usage).
  selectedRoomID?: string;
  onSelectRoom?: (roomID: string) => void;
  onRoomRead?: (roomID: string) => void;
  onOpenMemoryDirectory?: (path: string) => void;
  onOpenSession?: (sessionID: string) => void;
  composerDraft?: {
    prompt: string;
    images: ComposerImage[];
    files: ComposerFile[];
  };
  onComposerDraftChange?: (draft: {
    prompt: string;
    images: ComposerImage[];
    files: ComposerFile[];
  }) => void;
  // Incremented by the parent (sidebar ＋ button) to request the new-room
  // dialog; the dialog itself stays inside this view.
  newRoomRequest?: number;
  onNewRoomRequestHandled?: () => void;
  newAgentRequest?: number;
  onNewAgentRequestHandled?: () => void;
  editAgentRequestID?: string;
  onEditAgentRequestHandled?: () => void;
}): JSX.Element {
  const { formatDate, locale, t } = useI18n();
  const [agents, setAgents] = useState<NamedAgent[]>([]);
  const [agentInsights, setAgentInsights] = useState<Record<string, ChannelAgentInsight>>({});
  const [rooms, setRooms] = useState<ChannelRoom[]>([]);
  const [internalSelectedRoomID, setInternalSelectedRoomID] = useState("");
  const selectedRoomID = controlledRoomID ?? internalSelectedRoomID;
  const setSelectedRoomID = useCallback((value: string | ((current: string) => string)): void => {
    const base = controlledRoomID ?? internalSelectedRoomID;
    const next = typeof value === "function" ? value(base) : value;
    setInternalSelectedRoomID(next);
    if (next !== base) onSelectRoom?.(next);
  }, [controlledRoomID, internalSelectedRoomID, onSelectRoom]);
  const [messagesByRoomID, setMessagesByRoomID] = useState<Record<string, ChannelMessage[]>>({});
  const [loadedRoomIDs, setLoadedRoomIDs] = useState<Set<string>>(() => new Set());
  const messages = messagesByRoomID[selectedRoomID] ?? [];
  const [trackedTasks, setTrackedTasks] = useState<ChannelMessage[]>([]);
  const [setupPanel, setSetupPanel] = useState<SetupPanel>(null);
  const [splitWidth, setSplitWidth] = useState(initialChannelSplitWidth);
  const [listCollapsed, setListCollapsed] = useState(initialChannelListCollapsed);
  const [resizingSplit, setResizingSplit] = useState(false);
  const [threadWidth, setThreadWidth] = useState(initialThreadPanelWidth);
  const [resizingThread, setResizingThread] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [loading, setLoading] = useState(true);
  const [sending, setSending] = useState(false);
  const [sendingAgentIDs, setSendingAgentIDs] = useState<Set<string>>(() => new Set());
  const [body, setBody] = useState(composerDraft?.prompt ?? "");
  const [composerImages, setComposerImages] = useState<ComposerImage[]>(
    composerDraft?.images ?? [],
  );
  const [composerFiles, setComposerFiles] = useState<ComposerFile[]>(
    composerDraft?.files ?? [],
  );
  const [activeThreadRootID, setActiveThreadRootID] = useState("");
  const [threadReplyTargetID, setThreadReplyTargetID] = useState("");
  const [threadBody, setThreadBody] = useState("");
  const [threadSending, setThreadSending] = useState(false);
  const [threadComposerImages, setThreadComposerImages] = useState<ComposerImage[]>([]);
  const [threadComposerFiles, setThreadComposerFiles] = useState<ComposerFile[]>([]);

  const composerRoomIDRef = useRef(selectedRoomID);
  const skipComposerPublishRef = useRef(false);
  useLayoutEffect(() => {
    if (composerRoomIDRef.current === selectedRoomID) return;
    composerRoomIDRef.current = selectedRoomID;
    skipComposerPublishRef.current = true;
    setBody(composerDraft?.prompt ?? "");
    setComposerImages(composerDraft?.images ?? []);
    setComposerFiles(composerDraft?.files ?? []);
    setActiveThreadRootID("");
  }, [composerDraft, selectedRoomID]);

  useEffect(() => {
    if (!selectedRoomID) return;
    if (skipComposerPublishRef.current) {
      skipComposerPublishRef.current = false;
      return;
    }
    onComposerDraftChange?.({
      prompt: body,
      images: composerImages,
      files: composerFiles,
    });
  }, [body, composerFiles, composerImages, onComposerDraftChange, selectedRoomID]);
  const [agentName, setAgentName] = useState("");
  const [agentRole, setAgentRole] = useState("");
  const [agentAvatarKey, setAgentAvatarKey] = useState<string>(() => randomAgentAvatarKey());
  const [agentAvatarImage, setAgentAvatarImage] = useState("");
  const [agentAvatarError, setAgentAvatarError] = useState("");
  const [agentEngine, setAgentEngine] = useState("wuu");
  const [agentModel, setAgentModel] = useState("");
  const [agentEffort, setAgentEffort] = useState("");
  const [editingAgentID, setEditingAgentID] = useState("");
  const [selectedAgentID, setSelectedAgentID] = useState("");
  const [savingAgentID, setSavingAgentID] = useState("");
  const [resettingAgentID, setResettingAgentID] = useState("");
  const [agentResetStatus, setAgentResetStatus] = useState("");
  const [roomName, setRoomName] = useState("");
  const [roomAgentIDs, setRoomAgentIDs] = useState<string[]>([]);
  const [roomAvatarImage, setRoomAvatarImage] = useState("");
  const [editingRoomID, setEditingRoomID] = useState("");
  const [roomMemberMode, setRoomMemberMode] = useState<RoomMemberMode>(null);
  const [roomMemberSelectionIDs, setRoomMemberSelectionIDs] = useState<string[]>([]);
  const [updatingRoomMembers, setUpdatingRoomMembers] = useState(false);
  const [taskTitle, setTaskTitle] = useState("");
  const [taskRoomID, setTaskRoomID] = useState("");
  const [taskOwnerID, setTaskOwnerID] = useState("");
  const [composerFooterNode, setComposerFooterNode] = useState<HTMLDivElement | null>(null);
  const agentDetailDraftRef = useRef<AgentDetailDraft>({ name: "", role: "", avatarKey: "", avatarImage: "", engine: "wuu", model: "", effort: "" });
  const selectedAgentIDRef = useRef("");
  const previousSectionRef = useRef(section);
  agentDetailDraftRef.current = { name: agentName, role: agentRole, avatarKey: agentAvatarKey, avatarImage: agentAvatarImage, engine: agentEngine, model: agentModel, effort: agentEffort };
  selectedAgentIDRef.current = selectedAgentID;
  const conversationRef = useRef<HTMLDivElement | null>(null);
  const composerFooterRef = useRef<HTMLDivElement | null>(null);
  const roomComposerRef = useRef<ChannelComposerHandle | null>(null);
  const threadComposerRef = useRef<ChannelComposerHandle | null>(null);
  const agentAvatarInputRef = useRef<HTMLInputElement | null>(null);
  const agentDetailAvatarInputRef = useRef<HTMLInputElement | null>(null);
  const roomAvatarInputRef = useRef<HTMLInputElement | null>(null);
  const roomAvatarTargetRef = useRef<string>("");
  const savedRoomNameRef = useRef("");
  const messagesByRoomIDRef = useRef<Map<string, ChannelMessage[]>>(new Map());
  const messageRefreshGenerationByRoomRef = useRef<Map<string, number>>(new Map());
  const markedMessageSeqByRoomRef = useRef<Map<string, number>>(new Map());
  const visibleRoomIDRef = useRef("");
  visibleRoomIDRef.current = section === "rooms" ? selectedRoomID : "";
  const sendingTimersRef = useRef<Map<string, number>>(new Map());
  const splitResizeStartRef = useRef({ x: 0, width: CHANNEL_SPLIT_DEFAULT_WIDTH });
  const threadResizeStartRef = useRef({ x: 0, width: THREAD_PANEL_DEFAULT_WIDTH });
  const messageScroll = useAutoFollowScrollContainer({
    open: section === "rooms" && Boolean(selectedRoomID),
    observeKey: selectedRoomID,
  });
  const setComposerFooter = useCallback((node: HTMLDivElement | null): void => {
    composerFooterRef.current = node;
    setComposerFooterNode(node);
  }, []);

  const updateSplitWidth = useCallback((width: number): void => {
    const nextWidth = clampChannelSplitWidth(width);
    setSplitWidth(nextWidth);
    window.localStorage.setItem(CHANNEL_SPLIT_WIDTH_KEY, String(nextWidth));
  }, []);

  const toggleListCollapsed = useCallback((): void => {
    setListCollapsed((collapsed) => {
      const nextCollapsed = !collapsed;
      window.localStorage.setItem(CHANNEL_LIST_COLLAPSED_KEY, String(nextCollapsed));
      return nextCollapsed;
    });
  }, []);

  const updateThreadWidth = useCallback((width: number): void => {
    const nextWidth = clampThreadPanelWidth(width);
    setThreadWidth(nextWidth);
    window.localStorage.setItem(THREAD_PANEL_WIDTH_KEY, String(nextWidth));
  }, []);

  const closeThread = useCallback((): void => {
    setActiveThreadRootID("");
    setThreadReplyTargetID("");
    setThreadBody("");
    setThreadComposerImages([]);
    setThreadComposerFiles([]);
    setResizingThread(false);
  }, []);

  useEffect(() => {
    if (!resizingSplit) return;
    const handlePointerMove = (event: PointerEvent): void => {
      updateSplitWidth(splitResizeStartRef.current.width + event.clientX - splitResizeStartRef.current.x);
    };
    const handlePointerUp = (): void => setResizingSplit(false);
    window.addEventListener("pointermove", handlePointerMove);
    window.addEventListener("pointerup", handlePointerUp, { once: true });
    return () => {
      window.removeEventListener("pointermove", handlePointerMove);
      window.removeEventListener("pointerup", handlePointerUp);
    };
  }, [resizingSplit, updateSplitWidth]);

  useEffect(() => {
    if (!resizingThread) return;
    const handlePointerMove = (event: PointerEvent): void => {
      const nextWidth = threadResizeStartRef.current.width - (event.clientX - threadResizeStartRef.current.x);
      if (nextWidth <= THREAD_PANEL_MIN_WIDTH) {
        closeThread();
        return;
      }
      updateThreadWidth(nextWidth);
    };
    const handlePointerUp = (): void => setResizingThread(false);
    window.addEventListener("pointermove", handlePointerMove);
    window.addEventListener("pointerup", handlePointerUp, { once: true });
    return () => {
      window.removeEventListener("pointermove", handlePointerMove);
      window.removeEventListener("pointerup", handlePointerUp);
    };
  }, [closeThread, resizingThread, updateThreadWidth]);

  useEffect(() => {
    if (!newRoomRequest) return;
    openNewRoom();
    onNewRoomRequestHandled?.();
  }, [newRoomRequest]);

  useEffect(() => {
    if (!newAgentRequest) return;
    setEditingAgentID("");
    setAgentName("");
    setAgentRole("");
    setAgentAvatarKey(randomAgentAvatarKey());
    setAgentAvatarImage("");
    setAgentAvatarError("");
    setAgentEngine("wuu");
    setAgentModel("");
    setAgentEffort("");
    setSetupPanel("agent");
    onNewAgentRequestHandled?.();
  }, [newAgentRequest]);

  useEffect(() => {
    if (!editAgentRequestID) return;
    const agent = agents.find((candidate) => candidate.id === editAgentRequestID);
    if (!agent) return;
    loadAgentDraft(agent);
    setSetupPanel("agent");
    onEditAgentRequestHandled?.();
  }, [agents, editAgentRequestID]);

  useEffect(() => {
    setActiveThreadRootID("");
    setThreadReplyTargetID("");
    setThreadBody("");
    setThreadComposerImages([]);
    setThreadComposerFiles([]);
  }, [selectedRoomID]);

  function startSplitResize(event: ReactPointerEvent<HTMLButtonElement>): void {
    event.preventDefault();
    splitResizeStartRef.current = { x: event.clientX, width: splitWidth };
    setResizingSplit(true);
  }

  function handleSplitResizeKeyDown(event: KeyboardEvent<HTMLButtonElement>): void {
    if (event.key === "ArrowLeft") {
      event.preventDefault();
      updateSplitWidth(splitWidth - CHANNEL_SPLIT_WIDTH_STEP);
    } else if (event.key === "ArrowRight") {
      event.preventDefault();
      updateSplitWidth(splitWidth + CHANNEL_SPLIT_WIDTH_STEP);
    } else if (event.key === "Home") {
      event.preventDefault();
      updateSplitWidth(CHANNEL_SPLIT_MIN_WIDTH);
    } else if (event.key === "End") {
      event.preventDefault();
      updateSplitWidth(CHANNEL_SPLIT_MAX_WIDTH);
    }
  }

  function startThreadResize(event: ReactPointerEvent<HTMLButtonElement>): void {
    event.preventDefault();
    threadResizeStartRef.current = { x: event.clientX, width: threadWidth };
    setResizingThread(true);
  }

  function handleThreadResizeKeyDown(event: KeyboardEvent<HTMLButtonElement>): void {
    if (event.key === "ArrowLeft") {
      event.preventDefault();
      updateThreadWidth(threadWidth + THREAD_PANEL_WIDTH_STEP);
    } else if (event.key === "ArrowRight") {
      event.preventDefault();
      if (threadWidth - THREAD_PANEL_WIDTH_STEP <= THREAD_PANEL_MIN_WIDTH) closeThread();
      else updateThreadWidth(threadWidth - THREAD_PANEL_WIDTH_STEP);
    } else if (event.key === "Home") {
      event.preventDefault();
      closeThread();
    } else if (event.key === "End") {
      event.preventDefault();
      updateThreadWidth(THREAD_PANEL_MAX_WIDTH);
    }
  }

  const selectedRoom = useMemo(
    () => rooms.find((room) => room.id === selectedRoomID),
    [rooms, selectedRoomID],
  );
  const selectedRoomTitle = useMemo(() => {
    if (!selectedRoom || selectedRoom.kind !== "dm") return selectedRoom?.name ?? "";
    const agentID = selectedRoom.members.find((member) => member.member_type === "agent")?.member_id;
    return agents.find((agent) => agent.id === agentID)?.name ?? selectedRoom.name;
  }, [agents, selectedRoom]);
  const roomIncludesCurrentUser = selectedRoom?.members.some(
    (member) => member.member_type === "human" && member.member_id === "local-user",
  ) ?? false;
  const messageAgents = agents;
  const agentNames = useMemo(
    () => new Map(messageAgents.map((agent) => [agent.id, agent.name])),
    [messageAgents],
  );
  const selectedAgent = useMemo(
    () => agents.find((agent) => agent.id === selectedAgentID),
    [agents, selectedAgentID],
  );
  const selectedAgentRooms = useMemo(
    () => selectedAgent ? rooms.filter((room) => !archivedRoomIDs.includes(room.id) && room.members.some(
      (member) => member.member_type === "agent" && member.member_id === selectedAgent.id,
    )) : [],
    [archivedRoomIDs, rooms, selectedAgent],
  );
  useEffect(() => {
    if (previousSectionRef.current === "agents" && section !== "agents" && selectedAgentIDRef.current) {
      void saveAgentDetails(selectedAgentIDRef.current, agentDetailDraftRef.current);
    }
    previousSectionRef.current = section;
  }, [section]);
  useEffect(() => {
    if (selectedAgentID && !selectedAgent) setSelectedAgentID("");
  }, [selectedAgent, selectedAgentID]);
  const selectedRoomAgents = useMemo(() => {
    const memberIDs = new Set(
      selectedRoom?.members
        .filter((member) => member.member_type === "agent")
        .map((member) => member.member_id) ?? [],
    );
    return agents.filter((agent) => memberIDs.has(agent.id));
  }, [agents, selectedRoom]);
  const taskRoom = useMemo(
    () => rooms.find((room) => room.id === (taskRoomID || selectedRoomID)),
    [rooms, selectedRoomID, taskRoomID],
  );
  const taskOwnerAgents = useMemo(() => {
    const memberIDs = new Set(
      taskRoom?.members
        .filter((member) => member.member_type === "agent")
        .map((member) => member.member_id) ?? [],
    );
    return agents.filter((agent) => memberIDs.has(agent.id));
  }, [agents, taskRoom]);
  useEffect(() => {
    if (setupPanel !== "task") return;
    setTaskOwnerID((current) => (
      taskOwnerAgents.some((agent) => agent.id === current)
        ? current
        : (taskOwnerAgents[0]?.id ?? "")
    ));
  }, [setupPanel, taskOwnerAgents]);
  const messageByID = useMemo(
    () => new Map(messages.map((message) => [message.id, message])),
    [messages],
  );
  const channelTimeline = useMemo(
    () => buildChannelTimeline(messages),
    [messages],
  );
  const repliesByThread = useMemo(() => {
    const replies = new Map<string, ChannelMessage[]>();
    for (const message of messages) {
      if (!message.thread_id || message.kind === "task") continue;
      const current = replies.get(message.thread_id) ?? [];
      current.push(message);
      replies.set(message.thread_id, current);
    }
    return replies;
  }, [messages]);
  const activeThreadRoot = activeThreadRootID ? messageByID.get(activeThreadRootID) : undefined;
  const activeThreadMessages = useMemo(
    () => activeThreadRootID ? (repliesByThread.get(activeThreadRootID) ?? []) : [],
    [activeThreadRootID, repliesByThread],
  );
  const threadConversationMessages = useMemo(
    () => activeThreadRoot ? [activeThreadRoot, ...activeThreadMessages] : [],
    [activeThreadMessages, activeThreadRoot],
  );
  const threadScroll = useAutoFollowScrollContainer({
    open: section === "rooms" && Boolean(activeThreadRootID),
    observeKey: `${activeThreadRootID}:${activeThreadMessages.length}`,
  });
  const activityFor = useCallback((agent?: NamedAgent): AgentActivityStatus => {
    if (!agent) return "idle";
    if (sendingAgentIDs.has(agent.id)) return "sending";
    return agent.activity_status === "thinking" ? "thinking" : "idle";
  }, [sendingAgentIDs]);
  const activityText = useCallback(
    (status: AgentActivityStatus): string => t(`channels.agentStatus.${status}`),
    [t],
  );
  const respondingAgents = useMemo(() => {
    const roomAgentIDs = new Set(
      selectedRoom?.members
        .filter((member) => member.member_type === "agent")
        .map((member) => member.member_id) ?? [],
    );
    return agents
      .filter((agent) => roomAgentIDs.has(agent.id))
      .map((agent) => ({ agent, status: activityFor(agent) }))
      .filter(({ agent, status }) => {
        if (status === "sending") return true;
        if (status !== "thinking") return false;
        const activityRoomIDs = agent.activity_room_ids;
        // Activity observed through another app-server is intentionally coarse
        // and has no room IDs. Keep an active room member visible unless the
        // backend explicitly scopes that activity to a different room.
        return !activityRoomIDs?.length || activityRoomIDs.includes(selectedRoomID);
      });
  }, [activityFor, agents, selectedRoom, selectedRoomID]);
  const respondingAgentNames = useMemo(
    () => new Intl.ListFormat(locale, { style: "short", type: "conjunction" })
      .format(respondingAgents.map(({ agent }) => agent.name)),
    [locale, respondingAgents],
  );
  const selectableEngines = useMemo(
    () => engines.filter((engine) => engine.enabled && engine.binary_ok),
    [engines],
  );
  const engineGroups = useMemo<SelectMenuGroup[]>(() => {
    const options = [
      { value: "wuu", label: "Wuu" },
      ...selectableEngines
        .filter((engine) => engine.id !== "wuu")
        .map((engine) => ({
          value: engine.id,
          label: engine.id === "codex" ? "Codex" : engine.id === "claude" ? "Claude Code" : engine.id,
        })),
    ];
    if (agentEngine !== "wuu" && !options.some((option) => option.value === agentEngine)) {
      options.push({ value: agentEngine, label: agentEngine });
    }
    return [{ options }];
  }, [agentEngine, selectableEngines]);
  const modelGroups = useMemo<SelectMenuGroup[]>(() => {
    if (agentEngine !== "wuu") {
      const engine = selectableEngines.find((candidate) => candidate.id === agentEngine);
      const groups: SelectMenuGroup[] = [{
        label: engine?.id,
        options: (engine?.models ?? []).map((model) => ({
          value: `\u0000${model.id}`,
          label: model.display_name || model.id,
          hint: model.id,
        })),
      }];
      if (agentModel && !groups.some((group) => group.options.some((option) => option.value === agentModel))) {
        const modelID = agentModel.split("\u0000")[1] || agentModel;
        groups.push({ options: [{ value: agentModel, label: modelID, hint: t("channels.providerMissing") }] });
      }
      return groups;
    }
    const inherited = initialized ? `${initialized.provider} · ${initialized.model}` : undefined;
    const groups: SelectMenuGroup[] = [{ options: [{ value: "", label: t("channels.inheritModel"), hint: inherited }] }];
    for (const provider of initialized?.providers ?? []) {
      const models = provider.models?.length ? provider.models : [{ id: provider.model, display_name: provider.model }];
      groups.push({
        label: provider.name,
        options: models.filter((model) => model.id).map((model) => ({
          value: `${provider.name}\u0000${model.id}`,
          label: model.display_name || model.id,
          hint: model.id,
        })),
      });
    }
    // An agent's override can outlive the provider entry it points at (the
    // provider was renamed or removed from config). Surface the stored value
    // as its own group so the trigger reflects reality instead of rendering
    // an empty placeholder.
    if (agentModel && !groups.some((group) => group.options.some((option) => option.value === agentModel))) {
      const [providerName, modelID] = agentModel.split("\u0000");
      groups.push({
        label: providerName || undefined,
        options: [{ value: agentModel, label: modelID || agentModel, hint: t("channels.providerMissing") }],
      });
    }
    return groups;
  }, [agentEngine, initialized, agentModel, selectableEngines, t]);
  const [agentProviderName, agentModelID] = agentModel.split("\u0000");
  const agentProvider = initialized?.providers?.find((provider) => provider.name === agentProviderName);
  const externalAgentModel = selectableEngines
    .find((engine) => engine.id === agentEngine)
    ?.models?.find((model) => model.id === agentModelID);
  const agentEffortOptions = agentEngine === "wuu"
    ? providerModelEffortOptions(agentProvider, agentModelID ?? "", agentEffort)
    : Array.from(new Set([...(externalAgentModel?.supported_efforts ?? []), agentEffort].filter(Boolean)));

  function selectAgentEngine(value: string): void {
    setAgentEngine(value);
    setAgentEffort("");
    if (value === "wuu") {
      setAgentModel("");
      return;
    }
    const engine = selectableEngines.find((candidate) => candidate.id === value);
    const model = engine?.models?.find((candidate) => candidate.is_default) ?? engine?.models?.[0];
    setAgentModel(model ? `\u0000${model.id}` : "");
    setAgentEffort(model?.default_effort ?? "");
  }

  function selectAgentModel(value: string): void {
    setAgentModel(value);
    setAgentEffort("");
  }

  const refreshRoomsAndAgents = useCallback(async (): Promise<void> => {
    if (!window.wuu) return;
    const result = await window.wuu.bootstrapChannels();
    setAgents(result.agents ?? []);
    setRooms(result.rooms ?? []);
    setSelectedRoomID((current) =>
      current && result.rooms.some((room) => room.id === current)
        ? current
        : (result.rooms[0]?.id ?? ""),
    );
  }, []);

  const markRoomRead = useCallback((roomID: string, latestMessageSeq: number): void => {
    if (!roomID || !window.wuu) return;
    const markedSeq = markedMessageSeqByRoomRef.current.get(roomID) ?? 0;
    if (latestMessageSeq <= markedSeq) return;
    markedMessageSeqByRoomRef.current.set(roomID, latestMessageSeq);
    setRooms((current) => current.map((room) => room.id === roomID ? { ...room, unread_count: 0 } : room));
    onRoomRead?.(roomID);
    void window.wuu.markChannelRoomRead({ room_id: roomID }).catch((reason: unknown) => {
      if (markedMessageSeqByRoomRef.current.get(roomID) === latestMessageSeq) {
        markedMessageSeqByRoomRef.current.delete(roomID);
      }
      showErrorToast(reason);
    });
  }, [onRoomRead]);

  const refreshMessages = useCallback(async (roomID: string): Promise<void> => {
    if (!window.wuu || !roomID) return;
    const generation = (messageRefreshGenerationByRoomRef.current.get(roomID) ?? 0) + 1;
    messageRefreshGenerationByRoomRef.current.set(roomID, generation);
    const result = await window.wuu.listChannelMessages({ room_id: roomID, limit: 500 });
    if (messageRefreshGenerationByRoomRef.current.get(roomID) !== generation) return;
    const nextMessages = result.messages ?? [];
    const previousMessages = messagesByRoomIDRef.current.get(roomID) ?? [];
    messagesByRoomIDRef.current.set(roomID, nextMessages);
    setMessagesByRoomID((current) => ({ ...current, [roomID]: nextMessages }));
    setLoadedRoomIDs((current) => {
      if (current.has(roomID)) return current;
      return new Set(current).add(roomID);
    });
    if (visibleRoomIDRef.current !== roomID) return;
    const known = new Set(previousMessages.map((message) => message.id));
    if (known.size > 0) {
      for (const message of nextMessages) {
        if (message.author_type !== "agent" || known.has(message.id)) continue;
        const agentID = message.author_id;
        const previousTimer = sendingTimersRef.current.get(agentID);
        if (previousTimer) window.clearTimeout(previousTimer);
        setSendingAgentIDs((current) => new Set(current).add(agentID));
        const timer = window.setTimeout(() => {
          setSendingAgentIDs((current) => {
            const next = new Set(current);
            next.delete(agentID);
            return next;
          });
          sendingTimersRef.current.delete(agentID);
        }, 1_800);
        sendingTimersRef.current.set(agentID, timer);
      }
    }
    const latestMessageSeq = nextMessages.reduce(
      (latest, message) => Math.max(latest, message.seq),
      0,
    );
    if (latestMessageSeq > 0) markRoomRead(roomID, latestMessageSeq);
  }, [markRoomRead]);

  const refreshTrackedTasks = useCallback(async (): Promise<void> => {
    if (!window.wuu || rooms.length === 0) {
      setTrackedTasks([]);
      return;
    }
    const results = await Promise.all(
      rooms.map((room) => window.wuu!.listChannelMessages({ room_id: room.id, limit: 500 })),
    );
    setTrackedTasks(
      results
        .flatMap((result) => result.messages ?? [])
        .filter((message) => message.kind === "task")
        .sort((left, right) => right.created_at.localeCompare(left.created_at)),
    );
  }, [rooms]);

  useEffect(() => {
    let active = true;
    void refreshRoomsAndAgents()
      .catch((reason: unknown) => {
        if (active) setLoadError(toastErrorMessage(reason));
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [refreshRoomsAndAgents]);

  useEffect(() => {
    if (!window.wuu) return;
    let active = true;
    const refresh = (): void => {
      void Promise.all([
        window.wuu!.listNamedAgents(),
        window.wuu!.listChannelRooms(),
      ]).then(([agentResult, roomResult]) => {
        if (!active) return;
        setLoadError("");
        setAgents(agentResult.agents ?? []);
        setRooms(roomResult.rooms ?? []);
      }).catch((reason: unknown) => {
        if (active) setLoadError(toastErrorMessage(reason));
      });
    };
    refresh();
    const timer = window.setInterval(refresh, 1_000);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, []);

  useEffect(() => {
    if (!window.wuu || typeof window.wuu.getNamedAgentInsights !== "function" || section !== "agents") return;
    let active = true;
    const refresh = (): void => {
      void window.wuu!.getNamedAgentInsights().then((result) => {
        if (!active) return;
        setAgentInsights(Object.fromEntries((result.insights ?? []).map((insight) => [insight.agent_id, insight])));
      }).catch(() => {
        // Activity statistics are an enhancement; the relationship graph and
        // agent management remain usable if historical aggregation fails.
      });
    };
    refresh();
    const timer = window.setInterval(refresh, 60_000);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, [section]);

  useEffect(() => {
    if (section !== "rooms" || !selectedRoomID) return;
    let active = true;
    const refresh = (): void => {
      void refreshMessages(selectedRoomID).catch((reason: unknown) => {
        if (active) setLoadError(toastErrorMessage(reason));
      });
    };
    refresh();
    const timer = window.setInterval(refresh, 2_000);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, [refreshMessages, section, selectedRoomID]);

  useEffect(() => () => {
    for (const timer of sendingTimersRef.current.values()) window.clearTimeout(timer);
  }, []);

  useEffect(() => {
    if (section !== "tasks") return;
    let active = true;
    const refresh = (): void => {
      void refreshTrackedTasks().catch((reason: unknown) => {
        if (active) setLoadError(toastErrorMessage(reason));
      });
    };
    refresh();
    const timer = window.setInterval(refresh, 2_000);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, [refreshTrackedTasks, section]);

  useEffect(() => {
    messageScroll.scrollToBottom();
  }, [messageScroll, messages.length, messages.at(-1)?.id]);

  useLayoutEffect(() => {
    const conversation = conversationRef.current;
    const footer = composerFooterRef.current;
    if (!conversation || !footer) return undefined;

    const updateComposerHeight = (): void => {
      const height = Math.ceil(footer.getBoundingClientRect().height);
      if (height > 0) {
        conversation.style.setProperty("--channel-composer-height", `${height}px`);
      } else {
        conversation.style.removeProperty("--channel-composer-height");
      }
      messageScroll.scheduleScrollToBottom();
    };

    updateComposerHeight();
    if (typeof ResizeObserver === "undefined") return undefined;
    const observer = new ResizeObserver(updateComposerHeight);
    observer.observe(footer);
    return () => observer.disconnect();
  }, [messageScroll]);

  async function submitAgent(): Promise<void> {
    if (!window.wuu || !agentName.trim()) return;
    try {
      const [providerOverride, modelOverride] = agentModel.split("\u0000");
      const params = {
        name: agentName.trim(),
        role: agentRole.trim() || undefined,
        avatar_key: agentAvatarKey,
        avatar_image: agentAvatarImage,
        engine_override: agentEngine === "wuu" ? undefined : agentEngine,
        provider_override: providerOverride || undefined,
        model_override: modelOverride || undefined,
        effort_override: modelOverride && agentEffort ? agentEffort : undefined,
      };
      if (editingAgentID) await window.wuu.updateNamedAgent({ agent_id: editingAgentID, ...params });
      else await window.wuu.createNamedAgent(params);
      closeAgentPanel();
      await refreshRoomsAndAgents();
    } catch (reason) {
      showErrorToast(reason);
    }
  }

  async function submitRoom(): Promise<void> {
    const name = roomName.trim();
    if (!window.wuu || !name) return;
    try {
      if (editingRoomID) {
        const roomID = editingRoomID;
        const result = await window.wuu.updateChannelRoom({
          room_id: roomID,
          name,
          agent_ids: roomAgentIDs,
        });
        if (result.room.id === roomID) {
          setRooms((current) => current.map((room) => room.id === result.room.id ? result.room : room));
        }
        await refreshRoomsAndAgents();
        setSelectedRoomID(roomID);
        closeRoomPanel();
        return;
      }
      const result = await window.wuu.createChannelRoom({
        name,
        avatar_image: roomAvatarImage || undefined,
        agent_ids: roomAgentIDs,
      });
      setRoomName("");
      setRoomAgentIDs([]);
      setRoomAvatarImage("");
      setSetupPanel(null);
      await refreshRoomsAndAgents();
      setSelectedRoomID(result.room.id);
    } catch (reason) {
      showErrorToast(reason);
    }
  }

  async function sendMessage(): Promise<void> {
    const messageBody = body.trim();
    if (!window.wuu || !selectedRoomID || (!messageBody && composerImages.length === 0 && composerFiles.length === 0) || sending) return;
    setSending(true);
    try {
      const resolvedImages = await awaitComposerImages(composerImages);
      await window.wuu.sendChannelMessage({
        room_id: selectedRoomID,
        body: messageBody,
        images: inputImagesFromComposer(resolvedImages),
        files: inputFilesFromComposer(composerFiles),
      });
      setBody("");
      setComposerImages([]);
      setComposerFiles([]);
      await refreshMessages(selectedRoomID);
    } catch (reason) {
      showErrorToast(reason);
    } finally {
      setSending(false);
    }
  }

  function openThread(message: ChannelMessage, replyTargetID = message.id): void {
    setActiveThreadRootID(message.thread_id ?? message.id);
    setThreadReplyTargetID(replyTargetID);
    setThreadBody("");
    setThreadComposerImages([]);
    setThreadComposerFiles([]);
  }

  async function sendThreadReply(): Promise<void> {
    const messageBody = threadBody.trim();
    if (!window.wuu || !selectedRoomID || !activeThreadRootID || (!messageBody && threadComposerImages.length === 0 && threadComposerFiles.length === 0) || threadSending) return;
    setThreadSending(true);
    try {
      const resolvedImages = await awaitComposerImages(threadComposerImages);
      await window.wuu.sendChannelMessage({
        room_id: selectedRoomID,
        thread_id: activeThreadRootID,
        reply_to: threadReplyTargetID || activeThreadRootID,
        body: messageBody,
        images: inputImagesFromComposer(resolvedImages),
        files: inputFilesFromComposer(threadComposerFiles),
      });
      setThreadBody("");
      setThreadComposerImages([]);
      setThreadComposerFiles([]);
      setThreadReplyTargetID(activeThreadRootID);
      await refreshMessages(selectedRoomID);
    } catch (reason) {
      showErrorToast(reason);
    } finally {
      setThreadSending(false);
    }
  }

  async function attachMessageFiles(files: File[]): Promise<void> {
    try {
      await buildComposerAttachments(
        files,
        (placeholder) => setComposerImages((current) => [...current, placeholder]),
        (encoded) => setComposerImages((current) => current.map((image) => image.id === encoded.id ? encoded : image)),
        (file) => setComposerFiles((current) => [...current, file]),
      );
    } catch (reason) {
      showErrorToast(reason);
    }
  }

  async function attachThreadMessageFiles(files: File[]): Promise<void> {
    try {
      await buildComposerAttachments(
        files,
        (placeholder) => setThreadComposerImages((current) => [...current, placeholder]),
        (encoded) => setThreadComposerImages((current) => current.map((image) => image.id === encoded.id ? encoded : image)),
        (file) => setThreadComposerFiles((current) => [...current, file]),
      );
    } catch (reason) {
      showErrorToast(reason);
    }
  }

  async function submitTask(): Promise<void> {
    const title = taskTitle.trim();
    const roomID = taskRoomID || selectedRoomID;
    if (!window.wuu || !roomID || !title || !taskOwnerID) return;
    try {
      await window.wuu.createChannelTask({
        room_id: roomID,
        title,
        owner_id: taskOwnerID,
      });
      setTaskTitle("");
      setTaskRoomID("");
      setTaskOwnerID("");
      setSetupPanel(null);
      await refreshMessages(selectedRoomID);
      if (section === "tasks") await refreshTrackedTasks();
    } catch (reason) {
      showErrorToast(reason);
    }
  }

  async function deleteAgent(agentID: string): Promise<void> {
    if (!window.wuu) return;
    if (!window.confirm(t("channels.deleteAgentConfirm", { name: agentName.trim() }))) return;
    try {
      await window.wuu.deleteNamedAgent({ agent_id: agentID });
      closeAgentPanel();
      await refreshRoomsAndAgents();
    } catch (reason) {
      showErrorToast(reason);
    }
  }

  async function resetAgent(agentID: string): Promise<void> {
    if (!window.wuu) return;
    if (!window.confirm(t("channels.resetAgentConfirm", { name: agentName.trim() }))) return;
    setAgentResetStatus("");
    setResettingAgentID(agentID);
    try {
      const result = await window.wuu.resetNamedAgent({ agent_id: agentID });
      setAgentResetStatus(t(result.requested ? "channels.resetAgentRequested" : "channels.resetAgentIdle"));
    } catch (reason) {
      showErrorToast(reason);
    } finally {
      setResettingAgentID("");
    }
  }

  function loadAgentDraft(agent: NamedAgent): void {
    setEditingAgentID(agent.id);
    setAgentName(agent.name);
    setAgentRole(agent.role ?? "");
    setAgentAvatarKey(agent.avatar_key);
    setAgentAvatarImage(agent.avatar_image ?? "");
    setAgentAvatarError("");
    const engine = agent.engine_override || "wuu";
    setAgentEngine(engine);
    setAgentModel(agent.model_override ? `${engine === "wuu" ? agent.provider_override ?? "" : ""}\u0000${agent.model_override}` : "");
    setAgentEffort(agent.effort_override ?? "");
    setAgentResetStatus("");
  }

  function selectAgentDetails(agent: NamedAgent): void {
    if (selectedAgentID === agent.id) return;
    if (selectedAgentID) void saveAgentDetails(selectedAgentID, agentDetailDraftRef.current);
    setSelectedAgentID(agent.id);
    loadAgentDraft(agent);
  }

  async function saveAgentDetails(agentID: string, draft: AgentDetailDraft = agentDetailDraftRef.current): Promise<void> {
    if (!window.wuu || !draft.name.trim()) return;
    const [providerOverride, modelOverride] = draft.model.split("\u0000");
    const effortOverride = modelOverride && draft.effort ? draft.effort : undefined;
    const currentAgent = agents.find((agent) => agent.id === agentID);
    if (currentAgent
      && currentAgent.name === draft.name.trim()
      && (currentAgent.role ?? "") === draft.role.trim()
      && currentAgent.avatar_key === draft.avatarKey
      && (currentAgent.avatar_image ?? "") === draft.avatarImage
      && (currentAgent.engine_override || "wuu") === draft.engine
      && (currentAgent.provider_override ?? "") === (providerOverride ?? "")
      && (currentAgent.model_override ?? "") === (modelOverride ?? "")
      && (currentAgent.effort_override ?? "") === (effortOverride ?? "")) return;
    setSavingAgentID(agentID);
    try {
      const result = await window.wuu.updateNamedAgent({
        agent_id: agentID,
        name: draft.name.trim(),
        role: draft.role.trim() || undefined,
        avatar_key: draft.avatarKey,
        avatar_image: draft.avatarImage,
        engine_override: draft.engine === "wuu" ? undefined : draft.engine,
        provider_override: providerOverride || undefined,
        model_override: modelOverride || undefined,
        effort_override: effortOverride,
      });
      setAgents((current) => current.map((agent) => agent.id === result.agent.id ? result.agent : agent));
    } catch (reason) {
      showErrorToast(reason);
    } finally {
      setSavingAgentID("");
    }
  }

  function closeAgentPanel(): void {
    setSetupPanel(null);
    setEditingAgentID("");
    setAgentName("");
    setAgentRole("");
    setAgentAvatarKey(randomAgentAvatarKey());
    setAgentAvatarImage("");
    setAgentAvatarError("");
    setAgentEngine("wuu");
    setAgentModel("");
    setAgentEffort("");
    setAgentResetStatus("");
  }

  function toggleRoomAgent(agentID: string): void {
    setRoomAgentIDs((current) =>
      current.includes(agentID)
        ? current.filter((candidate) => candidate !== agentID)
        : current.length < MAX_ROOM_AGENTS ? [...current, agentID] : current,
    );
  }

  function toggleRoomMemberSelection(agentID: string): void {
    setRoomMemberSelectionIDs((current) =>
      current.includes(agentID)
        ? current.filter((candidate) => candidate !== agentID)
        : roomAgentIDs.length + current.length < MAX_ROOM_AGENTS ? [...current, agentID] : current,
    );
  }

  function openRoomMemberMode(mode: Exclude<RoomMemberMode, null>): void {
    setRoomMemberSelectionIDs([]);
    setRoomMemberMode(mode);
  }

  function closeRoomMemberMode(): void {
    setRoomMemberMode(null);
    setRoomMemberSelectionIDs([]);
  }

  function openNewRoom(): void {
    setEditingRoomID("");
    setRoomName("");
    setRoomAgentIDs([]);
    setRoomAvatarImage("");
    closeRoomMemberMode();
    setSetupPanel("room");
  }

  function editRoom(room: ChannelRoom): void {
    setEditingRoomID(room.id);
    setRoomName(room.name);
    setRoomAgentIDs(room.members
      .filter((member) => member.member_type === "agent")
      .map((member) => member.member_id));
    setRoomAvatarImage(room.avatar_image ?? "");
    savedRoomNameRef.current = room.name;
    closeRoomMemberMode();
    setSetupPanel("room");
  }

  function closeRoomPanel(): void {
    if (editingRoomID) void persistRoomName();
    setSetupPanel(null);
    setEditingRoomID("");
    setRoomName("");
    setRoomAgentIDs([]);
    setRoomAvatarImage("");
    closeRoomMemberMode();
  }

  async function persistRoomName(): Promise<void> {
    const name = roomName.trim();
    if (!window.wuu || !editingRoomID || !name || name === savedRoomNameRef.current) return;
    const previousName = savedRoomNameRef.current;
    savedRoomNameRef.current = name;
    try {
      const result = await window.wuu.updateChannelRoom({
        room_id: editingRoomID,
        name,
        agent_ids: roomAgentIDs,
      });
      setRooms((current) => current.map((candidate) => candidate.id === editingRoomID ? result.room : candidate));
      setRoomName(result.room.name);
      savedRoomNameRef.current = result.room.name;
    } catch (reason) {
      savedRoomNameRef.current = previousName;
      showErrorToast(reason);
    }
  }

  async function submitRoomMemberChange(): Promise<void> {
    if (!window.wuu || !editingRoomID || !roomMemberMode || roomMemberSelectionIDs.length === 0 || updatingRoomMembers) return;
    const room = rooms.find((candidate) => candidate.id === editingRoomID);
    if (!room) return;
    const nextAgentIDs = [...roomAgentIDs, ...roomMemberSelectionIDs.filter((agentID) => !roomAgentIDs.includes(agentID))];
    setUpdatingRoomMembers(true);
    try {
      const result = await window.wuu.updateChannelRoom({
        room_id: editingRoomID,
        name: roomName.trim() || room.name,
        agent_ids: nextAgentIDs,
      });
      setRooms((current) => current.map((candidate) => candidate.id === editingRoomID ? result.room : candidate));
      setRoomAgentIDs(nextAgentIDs);
      closeRoomMemberMode();
      await refreshRoomsAndAgents();
    } catch (reason) {
      showErrorToast(reason);
    } finally {
      setUpdatingRoomMembers(false);
    }
  }

  async function removeRoomMember(agentID: string): Promise<void> {
    if (!window.wuu || !editingRoomID || updatingRoomMembers) return;
    const room = rooms.find((candidate) => candidate.id === editingRoomID);
    const agent = agents.find((candidate) => candidate.id === agentID);
    if (!room) return;
    if (!window.confirm(t("channels.removeMemberConfirm", { name: agent?.name ?? agentID }))) return;
    const nextAgentIDs = roomAgentIDs.filter((candidate) => candidate !== agentID);
    setUpdatingRoomMembers(true);
    try {
      const result = await window.wuu.updateChannelRoom({
        room_id: editingRoomID,
        name: roomName.trim() || room.name,
        agent_ids: nextAgentIDs,
      });
      setRooms((current) => current.map((candidate) => candidate.id === editingRoomID ? result.room : candidate));
      setRoomAgentIDs(nextAgentIDs);
      await refreshRoomsAndAgents();
    } catch (reason) {
      showErrorToast(reason);
    } finally {
      setUpdatingRoomMembers(false);
    }
  }

  async function deleteRoom(): Promise<void> {
    if (!window.wuu || !editingRoomID) return;
    if (!window.confirm(t("channels.deleteRoomConfirm", { name: roomName.trim() }))) return;
    try {
      const roomID = editingRoomID;
      await window.wuu.deleteChannelRoom({ room_id: roomID });
      closeRoomPanel();
      messagesByRoomIDRef.current.delete(roomID);
      setMessagesByRoomID((current) => {
        const next = { ...current };
        delete next[roomID];
        return next;
      });
      setLoadedRoomIDs((current) => {
        const next = new Set(current);
        next.delete(roomID);
        return next;
      });
      if (selectedRoomID === roomID) {
        setActiveThreadRootID("");
      }
      await refreshRoomsAndAgents();
    } catch (reason) {
      showErrorToast(reason);
    }
  }

  function chooseRoomAvatar(roomID: string): void {
    roomAvatarTargetRef.current = roomID;
    roomAvatarInputRef.current?.click();
  }

  async function updateRoomAvatarFromFile(file: File): Promise<void> {
    if (!window.wuu) return;
    try {
      const avatarImage = await squareAvatarImageFromFile(file);
      const roomID = roomAvatarTargetRef.current;
      if (!roomID) {
        setRoomAvatarImage(avatarImage);
        return;
      }
      const result = await window.wuu.updateChannelRoom({ room_id: roomID, avatar_image: avatarImage });
      setRooms((current) => current.map((room) => room.id === result.room.id ? result.room : room));
      if (editingRoomID === roomID) setRoomAvatarImage(result.room.avatar_image ?? "");
    } catch (reason) {
      showErrorToast(reason, t("channels.invalidAvatarImage"));
    }
  }

  return (
    <section
      className={`channel-view channel-mode-${section}${listCollapsed && section === "agents" ? " channel-list-collapsed" : ""}${resizingSplit ? " resizing-channel-split" : ""}`}
      aria-label={t("channels.title")}
      data-wuu-component="channel-view"
      data-wuu-variant={section}
      style={section === "agents" ? { gridTemplateColumns: `${listCollapsed ? CHANNEL_SPLIT_COLLAPSED_WIDTH : splitWidth}px minmax(0, 1fr)` } : undefined}
    >
      {section === "rooms" ? <div
        ref={conversationRef}
        className={`channel-conversation${activeThreadRoot ? " thread-open" : ""}${resizingThread ? " resizing-thread-split" : ""}`}
        style={{ "--channel-thread-width": `${threadWidth}px` } as CSSProperties}
      >
        <div className="channel-room-main">
          {selectedRoom ? (
            <header className="channel-room-header">
              <div className="channel-room-header-title">
                <h2>{selectedRoomTitle}</h2>
              </div>
              {respondingAgents.length > 0 ? (
                <div
                  className="channel-response-status"
                  role="status"
                  aria-live="polite"
                  aria-label={respondingAgents.map(({ agent, status }) => `${agent.name}: ${activityText(status)}`).join(", ")}
                >
                  <span className="channel-response-status-avatars" aria-hidden="true">
                    {respondingAgents.slice(0, 3).map(({ agent, status }) => (
                      <span className="channel-response-status-avatar" key={agent.id}>
                        <AgentAvatarMark seed={agent.id} avatarKey={agent.avatar_key} avatarImage={agent.avatar_image} status={status} />
                        <i className={`channel-response-status-dot ${status}`} />
                      </span>
                    ))}
                  </span>
                  <span className="channel-response-status-copy">
                    <strong>{respondingAgentNames}</strong>
                    <span>
                      {respondingAgents.length === 1
                        ? activityText(respondingAgents[0].status)
                        : t("channels.agentsResponding", { count: respondingAgents.length })}
                    </span>
                  </span>
                </div>
              ) : null}
              {selectedRoom.kind === "channel" ? <div className="channel-room-header-actions">
                <button
                  className="icon-button"
                  type="button"
                  aria-label={t("channels.manageRoom", { name: selectedRoom.name })}
                  aria-haspopup="dialog"
                  onClick={() => editRoom(selectedRoom)}
                >
                  <Settings2 className="icon" />
                </button>
              </div> : null}
            </header>
          ) : null}
          {!loading && rooms.length === 0 ? (
            <div className="channel-room-main-empty">
              <button className="channel-empty-action" type="button" onClick={openNewRoom}>
                {t("channels.newRoom")}
              </button>
            </div>
          ) : null}
          <input
            ref={roomAvatarInputRef}
            className="channel-avatar-file-input"
            type="file"
            accept="image/png,image/jpeg,image/webp"
            onChange={(event) => {
              const input = event.currentTarget;
              const file = input.files?.[0];
              if (!file) return;
              void updateRoomAvatarFromFile(file).finally(() => { input.value = ""; });
            }}
          />
          {loadError ? <div className="channel-error" role="alert">{loadError}</div> : null}
        <div ref={messageScroll.scrollRef} className="channel-message-stream" role="log" aria-live="polite">
          {channelTimeline.map((item) => {
            if (item.kind === "orchestration" && selectedRoom) {
              return (
                <ChannelOrchestrationCluster
                  key={`orchestration-${item.tasks[0].id}`}
                  room={selectedRoom}
                  tasks={item.tasks}
                  agents={agents}
                  repliesByThread={repliesByThread}
                  onOpenThread={(messageID) => {
                    const task = messageByID.get(messageID);
                    if (task) openThread(task);
                  }}
                  onOpenSession={onOpenSession}
                  onMention={(name) => roomComposerRef.current?.insertMention(name)}
                />
              );
            }
            if (item.kind !== "message") return null;
            const message = item.message;
            if (message.kind === "system") {
              return <div className="channel-system-message" key={message.id}>{message.body}</div>;
            }
            const own = message.author_type === "human";
            const author = own ? t("channels.you") : (agentNames.get(message.author_id) ?? message.author_id);
            const threadReplies = repliesByThread.get(message.id) ?? [];
            const agent = own ? undefined : messageAgents.find((candidate) => candidate.id === message.author_id);
            const status = activityFor(agent);
            return (
              <MessageBubbleRow
                key={message.id}
                outgoing={own}
                className={`channel-message ${own ? "own" : "agent"}`}
                contentClassName={`channel-message-content${threadReplies.length ? " has-thread-digest" : ""}`}
                avatar={!own ? (
                  <AgentAvatar id={agent?.id ?? message.author_id} name={author} avatarKey={agent?.avatar_key ?? "abstract-1"} avatarImage={agent?.avatar_image} status={status} statusText={activityText(status)} model={agent?.model_override || initialized?.model} modelLabel={t("channels.model")} />
                ) : undefined}
                meta={(
                  <div className="channel-message-meta">
                    {!own ? (
                      <ChannelAuthorName
                        name={author}
                        mentionLabel={t("channels.mentionAgent", { name: author })}
                        onMention={() => roomComposerRef.current?.insertMention(author)}
                      />
                    ) : null}
                    <time dateTime={message.created_at}>
                      {formatDate(message.created_at, { hour: "2-digit", minute: "2-digit" })}
                    </time>
                  </div>
                )}
                footer={threadReplies.length ? (
                    <ChannelThreadDigest
                      replies={threadReplies}
                      agents={messageAgents}
                      onOpen={() => openThread(message)}
                      onMention={(name) => roomComposerRef.current?.insertMention(name)}
                    />
                ) : undefined}
              >
                <ChannelMessageWithReplyAction outgoing={own} onReply={() => openThread(message)}>
                  <ChannelMessageBubble
                    message={message}
                    outgoing={own}
                    allowCollapse={!own}
                    onExpand={messageScroll.pauseAutoFollow}
                    attachmentIDPrefix="main"
                  />
                </ChannelMessageWithReplyAction>
              </MessageBubbleRow>
            );
          })}
          {!loading && loadedRoomIDs.has(selectedRoomID) && selectedRoom && channelTimeline.length === 0 ? (
            selectedRoomAgents[0] ? (
              <div className="channel-onboarding">
                <div className="channel-onboarding-avatar" aria-hidden="true">
                  <AgentAvatarMark
                    seed={selectedRoomAgents[0].id}
                    avatarKey={selectedRoomAgents[0].avatar_key}
                    avatarImage={selectedRoomAgents[0].avatar_image}
                    status={activityFor(selectedRoomAgents[0]) === "thinking" ? "thinking" : "idle"}
                  />
                </div>
                <h2>{t("channels.onboardingReady", { name: selectedRoomAgents[0].name })}</h2>
                <p>{t("channels.onboardingIntro", { name: selectedRoomAgents[0].name })}</p>
                <button type="button" onClick={() => roomComposerRef.current?.focus()}>
                  {t("channels.onboardingStart")}
                </button>
              </div>
            ) : <div className="channel-stream-empty">{t("channels.empty")}</div>
          ) : null}
        </div>
        <JumpToLatestPill
          containerRef={messageScroll.scrollRef}
          bottomAnchor={composerFooterNode}
          threshold={AUTO_FOLLOW_BOTTOM_THRESHOLD_PX}
        />
        {selectedRoom ? (
          <div ref={setComposerFooter} className="channel-conversation-footer">
            <ChannelComposer
              ref={roomComposerRef}
              draft={body}
              placeholder={t("channels.messagePlaceholder")}
              disabled={false}
              sending={sending}
              files={composerFiles}
              images={composerImages}
              mentionAgents={selectedRoomAgents}
              queryHistorySessionID={selectedRoomID}
              onChangeDraft={setBody}
              onPasteAttachmentFiles={(files) => void attachMessageFiles(files)}
              onRemoveFile={(id) => setComposerFiles((current) => current.filter((file) => file.id !== id))}
              onRemoveImage={(id) => setComposerImages((current) => current.filter((image) => image.id !== id))}
              onSend={() => void sendMessage()}
            />
          </div>
        ) : null}
        </div>
        {activeThreadRoot ? (
          <aside className="channel-thread-panel" aria-label={t("channels.thread")}>
            <button
              className="channel-thread-resizer"
              type="button"
              role="separator"
              aria-label={t("channels.resizeThread")}
              aria-orientation="vertical"
              aria-valuemin={0}
              aria-valuemax={THREAD_PANEL_MAX_WIDTH}
              aria-valuenow={threadWidth}
              onPointerDown={startThreadResize}
              onKeyDown={handleThreadResizeKeyDown}
            />
            <div ref={threadScroll.scrollRef} className="channel-thread-messages">
              {threadConversationMessages.map((message) => {
                const own = message.author_type === "human";
                const author = own ? t("channels.you") : (agentNames.get(message.author_id) ?? message.author_id);
                const repliedMessage = message.reply_to ? messageByID.get(message.reply_to) : undefined;
                const agent = own ? undefined : messageAgents.find((candidate) => candidate.id === message.author_id);
                const status = activityFor(agent);
                return (
                  <MessageBubbleRow
                    key={message.id}
                    outgoing={own}
                    className={`channel-message channel-thread-message ${own ? "own" : "agent"}`}
                    contentClassName="channel-message-content"
                    avatar={!own ? (
                      <AgentAvatar id={agent?.id ?? message.author_id} name={author} avatarKey={agent?.avatar_key ?? "abstract-1"} avatarImage={agent?.avatar_image} status={status} statusText={activityText(status)} model={agent?.model_override || initialized?.model} modelLabel={t("channels.model")} />
                    ) : undefined}
                    meta={(
                      <div className="channel-message-meta">
                        {!own ? (
                          <ChannelAuthorName
                            name={author}
                            mentionLabel={t("channels.mentionAgent", { name: author })}
                            onMention={() => threadComposerRef.current?.insertMention(author)}
                          />
                        ) : null}
                        <time dateTime={message.created_at}>
                          {formatDate(message.created_at, { hour: "2-digit", minute: "2-digit" })}
                        </time>
                      </div>
                    )}
                  >
                    <ChannelMessageWithReplyAction outgoing={own} onReply={() => setThreadReplyTargetID(message.id)}>
                      <ChannelMessageBubble
                        message={message}
                        outgoing={own}
                        allowCollapse={!own}
                        onExpand={threadScroll.pauseAutoFollow}
                        attachmentIDPrefix="thread"
                        beforeBody={repliedMessage && repliedMessage.id !== activeThreadRoot.id ? (
                          <button className="channel-thread-reference" type="button" onClick={() => setThreadReplyTargetID(repliedMessage.id)}>
                            {t("channels.replyingTo", {
                              name: repliedMessage.author_type === "human"
                                ? t("channels.you")
                                : (agentNames.get(repliedMessage.author_id) ?? repliedMessage.author_id),
                            })}
                            <span>{repliedMessage.body}</span>
                          </button>
                        ) : undefined}
                      />
                    </ChannelMessageWithReplyAction>
                  </MessageBubbleRow>
                );
              })}
            </div>
            <div className="channel-thread-footer">
              {threadReplyTargetID && threadReplyTargetID !== activeThreadRoot.id ? (
                <div className="channel-thread-replying">
                  <span>{t("channels.replyingTo", {
                    name: messageByID.get(threadReplyTargetID)?.author_type === "human"
                      ? t("channels.you")
                      : (agentNames.get(messageByID.get(threadReplyTargetID)?.author_id ?? "") ?? messageByID.get(threadReplyTargetID)?.author_id ?? ""),
                  })}</span>
                  <button type="button" onClick={() => setThreadReplyTargetID(activeThreadRoot.id)} aria-label={t("channels.cancelReply")}><X aria-hidden="true" /></button>
                </div>
              ) : null}
              <ChannelComposer
                ref={threadComposerRef}
                draft={threadBody}
                placeholder={t("channels.threadPlaceholder")}
                hideExpandButton
                disabled={false}
                sending={threadSending}
                files={threadComposerFiles}
                images={threadComposerImages}
                mentionAgents={selectedRoomAgents}
                queryHistorySessionID={activeThreadRoot.id}
                onChangeDraft={setThreadBody}
                onPasteAttachmentFiles={(files) => void attachThreadMessageFiles(files)}
                onRemoveFile={(id) => setThreadComposerFiles((current) => current.filter((file) => file.id !== id))}
                onRemoveImage={(id) => setThreadComposerImages((current) => current.filter((image) => image.id !== id))}
                onSend={() => void sendThreadReply()}
              />
            </div>
          </aside>
        ) : null}
      </div> : section === "agents" ? (
        <div className="channel-agent-workspace">
          <aside className={`channel-list-pane channel-agent-directory${listCollapsed ? " collapsed" : ""}`}>
            <div className="channel-pane-heading">
              {!listCollapsed ? <span>{t("channels.agents")}<small className="channel-pane-count">{agents.length}</small></span> : null}
              <div className="channel-heading-actions">
                <button className="icon-button channel-list-collapse-toggle" type="button" aria-label={t(listCollapsed ? "channels.expandList" : "channels.collapseList")} aria-expanded={!listCollapsed} onClick={toggleListCollapsed}>
                  {listCollapsed ? <PanelLeftOpen className="icon" /> : <PanelLeftClose className="icon" />}
                </button>
                {!listCollapsed ? <button
                  className="icon-button"
                  type="button"
                  aria-label={t("channels.newAgent")}
                  onClick={() => {
                    if (selectedAgentID) void saveAgentDetails(selectedAgentID, agentDetailDraftRef.current);
                    setEditingAgentID("");
                    setAgentName("");
                    setAgentRole("");
                    setAgentAvatarKey(randomAgentAvatarKey());
                    setAgentAvatarImage("");
                    setAgentAvatarError("");
                    setAgentEngine("wuu");
                    setAgentModel("");
                    setAgentEffort("");
                    setSetupPanel("agent");
                  }}
                >
                  <Plus className="icon" />
                </button> : null}
              </div>
            </div>
            {!listCollapsed && loadError ? <div className="channel-error" role="alert">{loadError}</div> : null}
            {!listCollapsed ? <div className="channel-agent-directory-list channel-directory-list">
            <button
              className={`channel-agent-graph-entry${selectedAgentID ? "" : " selected"}`}
              type="button"
              aria-current={selectedAgentID ? undefined : "page"}
              onClick={() => {
                if (selectedAgentID) void saveAgentDetails(selectedAgentID, agentDetailDraftRef.current);
                setSelectedAgentID("");
              }}
            >
              <Network className="icon" />
              <span>{t("channels.relationshipGraph")}</span>
            </button>
            {agents.map((agent) => {
              const status = activityFor(agent);
              const roomCount = rooms.filter((room) => room.members.some((member) => member.member_type === "agent" && member.member_id === agent.id)).length;
              const model = agent.model_override || t("channels.inheritModel");
              return (
                <div className={`channel-directory-row channel-agent-directory-row${selectedAgentID === agent.id ? " selected" : ""}`} key={agent.id}>
                  <button className="channel-directory-avatar" type="button" aria-label={t("channels.viewAgent", { name: agent.name })} onClick={() => selectAgentDetails(agent)}>
                    <AgentAvatar id={agent.id} name={agent.name} avatarKey={agent.avatar_key} avatarImage={agent.avatar_image} status={status} statusText={activityText(status)} model={agent.model_override || initialized?.model} modelLabel={t("channels.model")} expressive />
                  </button>
                  <button className="channel-directory-identity channel-agent-directory-identity" type="button" aria-current={selectedAgentID === agent.id ? "page" : undefined} onClick={() => selectAgentDetails(agent)}>
                    <span><strong>{agent.name}</strong><small>{agent.role || model} · {t("channels.agentRoomCount", { count: roomCount })}</small></span>
                  </button>
                </div>
              );
            })}
            {!loading && agents.length === 0 ? <div className="channel-management-empty">{t("channels.newAgent")}</div> : null}
            </div> : null}
            {!listCollapsed ? <button
              className="channel-split-resizer"
              type="button"
              role="separator"
              aria-label={t("channels.resizeList")}
              aria-orientation="vertical"
              aria-valuemin={CHANNEL_SPLIT_MIN_WIDTH}
              aria-valuemax={CHANNEL_SPLIT_MAX_WIDTH}
              aria-valuenow={splitWidth}
              onPointerDown={startSplitResize}
              onKeyDown={handleSplitResizeKeyDown}
            /> : null}
          </aside>
          <div className="channel-agent-graph-pane">
            {selectedAgent ? (
              <article className="channel-agent-detail">
                <header className="channel-agent-detail-header">
                  <button className="channel-agent-detail-avatar" type="button" aria-label={t("channels.customAvatar")} onClick={() => agentDetailAvatarInputRef.current?.click()}>
                    <AgentAvatarMark seed={selectedAgent.id} avatarKey={agentAvatarKey} avatarImage={agentAvatarImage} status={activityFor(selectedAgent)} />
                    <span aria-hidden="true"><ImagePlus className="icon" /></span>
                  </button>
                  <input
                    ref={agentDetailAvatarInputRef}
                    className="channel-avatar-file-input"
                    type="file"
                    accept="image/png,image/jpeg,image/webp"
                    onChange={(event) => {
                      const input = event.currentTarget;
                      const file = input.files?.[0];
                      if (!file) return;
                      setAgentAvatarError("");
                      void squareAvatarImageFromFile(file)
                        .then(setAgentAvatarImage)
                        .catch(() => setAgentAvatarError(t("channels.invalidAvatarImage")))
                        .finally(() => { input.value = ""; });
                    }}
                  />
                  <div>
                    <span>{t("channels.agentDetails")}</span>
                    <h2>{selectedAgent.name}</h2>
                    <p>{activityText(activityFor(selectedAgent))}</p>
                  </div>
                  <div className="channel-agent-detail-actions">
                    <button type="button" disabled={Boolean(resettingAgentID || savingAgentID)} onClick={() => void resetAgent(selectedAgent.id)}>
                      {t(resettingAgentID === selectedAgent.id ? "channels.resettingAgent" : "channels.resetAgent")}
                    </button>
                  </div>
                </header>
                {agentAvatarError ? <div className="channel-agent-detail-notice channel-error" role="alert">{agentAvatarError}</div> : null}
                {agentResetStatus ? <div className="channel-agent-detail-notice channel-agent-reset-status" role="status">{agentResetStatus}</div> : null}
                <div className="channel-agent-detail-grid">
                  <div className="channel-agent-detail-main">
                    <section>
                      <h3>{t("channels.agentRuntime")}</h3>
                      <div className="channel-agent-detail-form">
                        <label className="channel-form-field">
                          <span>{t("channels.name")}</span>
                          <input value={agentName} onChange={(event) => setAgentName(event.currentTarget.value)} />
                        </label>
                        <label className="channel-form-field">
                          <span>{t("channels.agentRole")}</span>
                          <textarea value={agentRole} onChange={(event) => setAgentRole(event.currentTarget.value)} maxLength={280} placeholder={t("channels.agentRolePlaceholder")} />
                        </label>
                        <AgentAvatarCreator
                          seed={selectedAgent.id}
                          avatarKey={agentAvatarKey}
                          avatarImage={agentAvatarImage}
                          onChange={(nextAvatarKey) => {
                            setAgentAvatarKey(nextAvatarKey);
                            setAgentAvatarImage("");
                            setAgentAvatarError("");
                          }}
                        />
                        <label className="channel-form-field">
                          <span>{t("channels.engine")}</span>
                          <SelectMenu value={agentEngine} onChange={selectAgentEngine} groups={engineGroups} ariaLabel={t("channels.engine")} />
                        </label>
                        <label className="channel-form-field">
                          <span>{t("channels.model")}</span>
                          <SelectMenu value={agentModel} onChange={selectAgentModel} groups={modelGroups} ariaLabel={t("channels.model")} />
                        </label>
                        {agentModel && agentEffortOptions.length > 1 ? (
                          <div className="channel-form-field">
                            <span id="channel-agent-detail-effort-label">{t("channels.effort")}</span>
                            <div className="channel-effort-picker" role="radiogroup" aria-labelledby="channel-agent-detail-effort-label">
                              {agentEffortOptions.map((effort) => (
                                <button className="channel-effort-chip" type="button" role="radio" key={effort} aria-checked={agentEffort === effort} aria-pressed={agentEffort === effort} onClick={() => setAgentEffort(effort)}>
                                  {effortLabel(effort)}
                                </button>
                              ))}
                            </div>
                          </div>
                        ) : null}
                      </div>
                    </section>
                  </div>
                  <div className="channel-agent-detail-side">
                    <section>
                      <h3>{t("channels.agentChannels")}</h3>
                      {selectedAgentRooms.length ? (
                        <div className="channel-agent-detail-rooms">
                          {selectedAgentRooms.map((room) => <span key={room.id}># {room.name}</span>)}
                        </div>
                      ) : <p className="channel-agent-detail-empty">{t("channels.agentNoChannels")}</p>}
                    </section>
                    <section>
                      <h3>{t("channels.agentInfo")}</h3>
                      <dl>
                        <div><dt>{t("channels.agentAutostart")}</dt><dd>{selectedAgent.autostart ? t("channels.enabled") : t("channels.disabled")}</dd></div>
                        <div>
                          <dt>{t("channels.agentMemoryDirectory")}</dt>
                          <dd>
                            <button
                              className="channel-agent-memory-link"
                              type="button"
                              title={selectedAgent.memory_dir}
                              onClick={() => {
                                if (onOpenMemoryDirectory) {
                                  onOpenMemoryDirectory(selectedAgent.memory_dir);
                                  return;
                                }
                                void window.wuu?.revealWorkspaceItem(selectedAgent.memory_dir);
                              }}
                            >
                              <code>./memory</code>
                            </button>
                          </dd>
                        </div>
                        <div><dt>{t("channels.agentCreatedAt")}</dt><dd>{formatDate(selectedAgent.created_at)}</dd></div>
                      </dl>
                    </section>
                  </div>
                </div>
              </article>
            ) : (
              <AgentRelationshipGraph
                agents={agents}
                rooms={rooms}
                insights={agentInsights}
                inheritedProvider={initialized?.provider}
                inheritedModel={initialized?.model}
                onSelectAgent={selectAgentDetails}
                ariaLabel={t("channels.relationshipGraph")}
                zoomInLabel={t("channels.zoomIn")}
                zoomOutLabel={t("channels.zoomOut")}
                resetViewLabel={t("channels.resetGraphView")}
              />
            )}
          </div>
        </div>
      ) : (
        <div className="channel-management-view channel-tasks-view">
          <div className="channel-management-heading">
            <div>
              <strong>{t("channels.tasks")}</strong>
              <span>{trackedTasks.length}</span>
            </div>
            <button
              className="channel-management-primary"
              type="button"
              disabled={!selectedRoom || selectedRoomAgents.length === 0}
              onClick={() => {
                setTaskRoomID(selectedRoomID || rooms[0]?.id || "");
                setTaskOwnerID(selectedRoomAgents[0]?.id ?? "");
                setSetupPanel("task");
              }}
            >
              <Plus className="icon" />
              {t("channels.newTask")}
            </button>
          </div>
          {loadError ? <div className="channel-error" role="alert">{loadError}</div> : null}
          <div className="channel-task-board channel-task-table">
            {(["open", "doing", "done"] as const).map((state) => {
              const tasks = trackedTasks.filter((task) => (task.task_state ?? "open") === state);
              return (
                <section className={`channel-task-column channel-task-column-${state}`} key={state}>
                  <header className="channel-task-column-heading">
                    <strong>{t(taskStateKey(state))}</strong>
                    <span>{tasks.length}</span>
                  </header>
                  <div className="channel-task-column-items">
                    {tasks.map((task) => {
                      const room = rooms.find((candidate) => candidate.id === task.room_id);
                      const owner = agentNames.get(task.task_owner ?? "") ?? task.task_owner ?? "";
                      return (
                        <button
                          className={`channel-task-card${state === "done" ? " done" : ""}`}
                          type="button"
                          key={task.id}
                          data-tooltip={`${room ? `# ${room.name}` : task.room_id} · ${owner}`}
                          onClick={() => {
                            setSelectedRoomID(task.room_id);
                            onSectionChange?.("rooms");
                          }}
                        >
                          <strong>{task.body}</strong>
                          <span className="channel-task-card-meta">{room ? `# ${room.name}` : task.room_id} · {owner}</span>
                        </button>
                      );
                    })}
                    {tasks.length === 0 ? <span className="channel-task-column-empty">—</span> : null}
                  </div>
                </section>
              );
            })}
          </div>
        </div>
      )}

      <SidebarNameDialog
        open={setupPanel === "agent"}
        title={agentName}
        onTitleChange={setAgentName}
        onSubmit={() => void submitAgent()}
        onClose={closeAgentPanel}
        dialogTitle={editingAgentID ? t("channels.editAgent") : t("channels.newAgent")}
        dialogTitleId="channel-agent-dialog-title"
        dialogClassName="channel-agent-editor-dialog"
        fieldLabel={t("channels.name")}
        fieldAriaLabel={t("channels.name")}
        placeholder="Andy"
        icon={Bot}
        submitLabel={editingAgentID ? t("channels.save") : t("channels.create")}
        cancelLabel={t("channels.cancel")}
        submitDisabled={!agentName.trim() || Boolean(resettingAgentID) || (agentEngine !== "wuu" && !agentModel)}
        content={<div className="channel-setup-form">
          {agentResetStatus ? <div className="channel-agent-reset-status" role="status">{agentResetStatus}</div> : null}
          <div className="channel-identity-row">
            <button
              className="channel-identity-avatar-button"
              type="button"
              aria-label={t("channels.customAvatar")}
              aria-invalid={Boolean(agentAvatarError)}
              aria-describedby={agentAvatarError ? "channel-agent-avatar-error" : undefined}
              onClick={() => agentAvatarInputRef.current?.click()}
            >
              <AgentAvatarMark seed={editingAgentID || "new-agent"} avatarKey={agentAvatarKey} avatarImage={agentAvatarImage} />
              <span className="channel-identity-avatar-badge" aria-hidden="true"><ImagePlus className="icon" /></span>
            </button>
            <label className="channel-form-field">
              <span>{t("channels.name")}</span>
              <input value={agentName} onChange={(event) => setAgentName(event.currentTarget.value)} autoFocus placeholder="Andy" />
            </label>
            <label className="channel-form-field">
              <span>{t("channels.agentRole")}</span>
              <textarea value={agentRole} onChange={(event) => setAgentRole(event.currentTarget.value)} maxLength={280} placeholder={t("channels.agentRolePlaceholder")} />
            </label>
          </div>
          <FieldError id="channel-agent-avatar-error">{agentAvatarError}</FieldError>
          <input
            ref={agentAvatarInputRef}
            className="channel-avatar-file-input"
            type="file"
            accept="image/png,image/jpeg,image/webp"
            onChange={(event) => {
              const input = event.currentTarget;
              const file = input.files?.[0];
              if (!file) return;
              setAgentAvatarError("");
              void squareAvatarImageFromFile(file)
                .then(setAgentAvatarImage)
                .catch(() => setAgentAvatarError(t("channels.invalidAvatarImage")))
                .finally(() => { input.value = ""; });
            }}
          />
          <AgentAvatarCreator
            seed={editingAgentID || "new-agent"}
            avatarKey={agentAvatarKey}
            avatarImage={agentAvatarImage}
            onChange={(nextAvatarKey) => {
              setAgentAvatarKey(nextAvatarKey);
              setAgentAvatarImage("");
              setAgentAvatarError("");
            }}
          />
          <div className="channel-form-section">
            <label className="channel-form-field">
              <span>{t("channels.engine")}</span>
              <SelectMenu value={agentEngine} onChange={selectAgentEngine} groups={engineGroups} ariaLabel={t("channels.engine")} flip />
            </label>
            <label className="channel-form-field">
              <span>{t("channels.model")}</span>
              <SelectMenu value={agentModel} onChange={selectAgentModel} groups={modelGroups} ariaLabel={t("channels.model")} flip />
            </label>
            {agentModel && agentEffortOptions.length > 1 ? (
              <div className="channel-form-field">
                <span id="channel-agent-effort-label">{t("channels.effort")}</span>
                <div className="channel-effort-picker" role="radiogroup" aria-labelledby="channel-agent-effort-label">
                  {agentEffortOptions.map((effort) => (
                    <button
                      className="channel-effort-chip"
                      type="button"
                      role="radio"
                      key={effort}
                      aria-checked={agentEffort === effort}
                      aria-pressed={agentEffort === effort}
                      onClick={() => setAgentEffort(effort)}
                    >
                      {effortLabel(effort)}
                    </button>
                  ))}
                </div>
              </div>
            ) : null}
          </div>
          {editingAgentID ? (
            <div className="channel-form-danger-row">
              <button type="button" disabled={Boolean(resettingAgentID)} onClick={() => void resetAgent(editingAgentID)}>
                {t(resettingAgentID === editingAgentID ? "channels.resettingAgent" : "channels.resetAgent")}
              </button>
              <button className="danger" type="button" disabled={Boolean(resettingAgentID)} onClick={() => void deleteAgent(editingAgentID)}>
                {t("channels.deleteAgent")}
              </button>
            </div>
          ) : null}
        </div>}
      />
      <SidebarNameDialog
        open={setupPanel === "room"}
        title={roomName}
        onTitleChange={setRoomName}
        onSubmit={() => void submitRoom()}
        onClose={closeRoomPanel}
        dialogTitle={t(editingRoomID ? "channels.roomDetails" : "channels.newRoom")}
        dialogTitleId="channel-room-dialog-title"
        fieldLabel={t("channels.name")}
        fieldAriaLabel={t("channels.name")}
        placeholder={t("channels.newRoom")}
        icon={MessageCircle}
        submitLabel={t(editingRoomID ? "channels.save" : "channels.create")}
        cancelLabel={t("channels.cancel")}
        submitDisabled={!roomName.trim()}
        variant={editingRoomID ? "drawer" : "default"}
        hideActions={Boolean(editingRoomID)}
        closeOnEscape={!roomMemberMode}
        backgrounded={Boolean(roomMemberMode)}
        content={editingRoomID ? (
          <div className="channel-room-details-form">
            {selectedRoom ? (
              <section className="channel-room-identity" aria-label={t("channels.groupOverview")}>
                <span className="channel-room-identity-avatar">
                  <ChannelGroupAvatar room={selectedRoom} agents={agents} />
                </span>
                <span className="channel-room-identity-copy">
                  <strong>{roomName}</strong>
                  <span>{t("channels.agentCount", { count: roomAgentIDs.length })}</span>
                </span>
              </section>
            ) : null}
            <section className="channel-room-members-section" aria-labelledby="channel-room-members-title">
              <header className="channel-room-members-header">
                <div>
                  <h3 id="channel-room-members-title">{t("channels.groupMembers")}</h3>
                  <span>{t("channels.memberCount", { count: roomAgentIDs.length + (roomIncludesCurrentUser ? 1 : 0) })}</span>
                </div>
              </header>
              <div className="channel-room-member-list">
                {roomIncludesCurrentUser ? (
                  <div className="channel-room-member-row current" aria-label={t("channels.you")}>
                    <span className="channel-room-member-avatar">
                      <HumanAvatarMark />
                    </span>
                    <span className="channel-room-member-identity">
                      <strong>{t("channels.you")}</strong>
                    </span>
                  </div>
                ) : null}
                {agents.filter((agent) => roomAgentIDs.includes(agent.id)).map((agent) => (
                  <div className="channel-room-member-row" key={agent.id}>
                    <span className="channel-room-member-avatar">
                      <AgentAvatarMark seed={agent.id} avatarKey={agent.avatar_key} avatarImage={agent.avatar_image} status={activityFor(agent)} />
                    </span>
                    <span className="channel-room-member-identity">
                      <strong>{agent.name}</strong>
                    </span>
                    <button
                      className="channel-room-member-remove"
                      type="button"
                      aria-label={t("channels.removeMember", { name: agent.name })}
                      disabled={updatingRoomMembers}
                      onClick={() => void removeRoomMember(agent.id)}
                    >
                      <X className="icon" aria-hidden="true" />
                    </button>
                  </div>
                ))}
                <button
                  className="channel-room-member-add"
                  type="button"
                  onClick={() => openRoomMemberMode("add")}
                  disabled={roomAgentIDs.length >= MAX_ROOM_AGENTS || agents.every((agent) => roomAgentIDs.includes(agent.id))}
                >
                  <Plus className="icon" aria-hidden="true" />
                  <span>{t("channels.addMember")}</span>
                </button>
              </div>
            </section>
            <section className="channel-room-settings-section" aria-labelledby="channel-room-settings-title">
              <h3 id="channel-room-settings-title">{t("channels.groupSettings")}</h3>
              <label className="sidebar-name-dialog-field">
                <span className="sidebar-name-dialog-label">{t("channels.name")}</span>
                <input
                  className="sidebar-name-dialog-input"
                  value={roomName}
                  onChange={(event) => setRoomName(event.currentTarget.value)}
                  onBlur={() => {
                    if (!roomName.trim()) {
                      setRoomName(savedRoomNameRef.current);
                      return;
                    }
                    void persistRoomName();
                  }}
                />
              </label>
            </section>
            <section className="channel-room-danger-zone" aria-labelledby="channel-room-danger-title">
              <div>
                <h3 id="channel-room-danger-title">{t("channels.dangerZone")}</h3>
                <p>{t("channels.deleteRoomHint")}</p>
              </div>
              <button type="button" onClick={() => void deleteRoom()}>{t("channels.deleteRoom")}</button>
            </section>
          </div>
        ) : (
          <div className="channel-setup-form">
            <div className="channel-identity-row">
              <button
                className="channel-identity-avatar-button"
                type="button"
                aria-label={t("channels.customGroupAvatar")}
                onClick={() => chooseRoomAvatar("")}
              >
                <ChannelGroupAvatar
                  room={{
                    id: editingRoomID || "new-room",
                    kind: "channel",
                    name: roomName,
                    avatar_image: roomAvatarImage || undefined,
                    created_by: "local-user",
                    created_at: "",
                    members: [
                      { room_id: editingRoomID || "new-room", member_type: "human", member_id: "local-user", joined_at: "" },
                      ...roomAgentIDs.map((agentID) => ({ room_id: editingRoomID || "new-room", member_type: "agent" as const, member_id: agentID, joined_at: "" })),
                    ],
                  }}
                  agents={agents}
                />
                <span className="channel-identity-avatar-badge" aria-hidden="true"><ImagePlus className="icon" /></span>
              </button>
              <label className="channel-form-field">
                <span>{t("channels.name")}</span>
                <input value={roomName} onChange={(event) => setRoomName(event.currentTarget.value)} autoFocus placeholder={t("channels.newRoom")} />
              </label>
            </div>
            <ChannelMemberPicker agents={agents} selectedAgentIDs={roomAgentIDs} onToggle={toggleRoomAgent} maxSelected={MAX_ROOM_AGENTS} />
          </div>
        )}
      />
      <SidebarNameDialog
        open={Boolean(editingRoomID && roomMemberMode)}
        title=""
        onTitleChange={() => undefined}
        onSubmit={() => void submitRoomMemberChange()}
        onClose={closeRoomMemberMode}
        dialogTitle={t("channels.addAgents")}
        dialogTitleId="channel-room-member-dialog-title"
        fieldLabel={t("channels.groupMembers")}
        fieldAriaLabel={t("channels.groupMembers")}
        placeholder=""
        icon={Plus}
        submitLabel={t("channels.addSelectedMembers", { count: roomMemberSelectionIDs.length })}
        cancelLabel={t("channels.cancel")}
        submitDisabled={roomMemberSelectionIDs.length === 0 || updatingRoomMembers}
        dialogClassName="channel-room-member-dialog"
        content={roomMemberMode ? (
          <div className="channel-room-member-flow">
            <p>{t("channels.addAgentsHint")}</p>
            <ChannelMemberPicker
              agents={agents.filter((agent) => !roomAgentIDs.includes(agent.id))}
              selectedAgentIDs={roomMemberSelectionIDs}
              onToggle={toggleRoomMemberSelection}
              label={t("channels.availableAgents")}
              maxSelected={Math.max(0, MAX_ROOM_AGENTS - roomAgentIDs.length)}
            />
          </div>
        ) : null}
      />
      <SidebarNameDialog
        open={setupPanel === "task"}
        title={taskTitle}
        onTitleChange={setTaskTitle}
        onSubmit={() => void submitTask()}
        onClose={() => { setSetupPanel(null); setTaskRoomID(""); }}
        dialogTitle={t("channels.newTask")}
        dialogTitleId="channel-task-dialog-title"
        fieldLabel={t("channels.taskTitle")}
        fieldAriaLabel={t("channels.taskTitle")}
        placeholder={t("channels.taskTitle")}
        icon={ClipboardList}
        submitLabel={t("channels.create")}
        cancelLabel={t("channels.cancel")}
        submitDisabled={!taskTitle.trim() || !(taskRoomID || selectedRoomID) || !taskOwnerID}
        content={<div className="channel-setup-form">
          <label className="sidebar-name-dialog-field"><span className="sidebar-name-dialog-label">{t("channels.taskTitle")}</span><input className="sidebar-name-dialog-input" value={taskTitle} onChange={(event) => setTaskTitle(event.currentTarget.value)} autoFocus /></label>
          <label className="sidebar-name-dialog-field"><span className="sidebar-name-dialog-label">{t("channels.taskRoomLabel")}</span><SelectMenu value={taskRoomID || selectedRoomID} onChange={setTaskRoomID} options={rooms.map((room) => ({ value: room.id, label: `# ${room.name}` }))} ariaLabel={t("channels.taskRoomLabel")} flip /></label>
          <label className="sidebar-name-dialog-field"><span className="sidebar-name-dialog-label">{t("channels.taskOwnerLabel")}</span><SelectMenu value={taskOwnerID} onChange={setTaskOwnerID} options={taskOwnerAgents.map((agent) => ({ value: agent.id, label: agent.name }))} ariaLabel={t("channels.taskOwnerLabel")} flip /></label>
        </div>}
      />
    </section>
  );
}
