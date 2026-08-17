import { useCallback, useMemo, useSyncExternalStore } from "react";

import type { TodoUpdate } from "../shared/protocol";
import { useI18n } from "./i18n";
import { desktopPluginHost } from "./plugins/DesktopPluginRuntime";
import type { PluginHost } from "./plugins/PluginHost";
import { PluginSlot } from "./plugins/PluginSlot";

export interface ConversationStatusClusterProps {
  /**
   * Master visibility switch. App sets this to the dock composer being
   * visible while the user is not scrolled away; the "jump to latest" pill
   * takes over this floating slot once the user scrolls up.
   */
  visible: boolean;
  /**
   * Main conversation session id, passed to the `composer.cluster` slot so
   * plugins can scope status lookups to the active thread.
   */
  threadId: string | undefined;
  /**
   * In-flight TODO list for the active turn. Progress is shown only while
   * items remain; the pill disappears once the list is fully completed.
   */
  todoUpdate: TodoUpdate | undefined;
  host?: PluginHost;
}

/**
 * Floating capsule row above the dock composer. It hosts the in-flight TODO
 * progress pill plus plugin contributions registered to the `composer.cluster`
 * slot (e.g. the bundled subagent status chips), so composer-adjacent status
 * shares one visual home instead of stacking rows above the input.
 */
export function ConversationStatusCluster({
  visible,
  threadId,
  todoUpdate,
  host = desktopPluginHost,
}: ConversationStatusClusterProps): React.ReactElement | null {
  const { t, formatNumber } = useI18n();

  const subscribeCluster = useCallback(
    (listener: () => void) => host.subscribeSlot("composer.cluster", listener),
    [host],
  );
  const snapshotCluster = useCallback(
    () => host.getSlotSnapshot("composer.cluster"),
    [host],
  );
  const clusterContributions = useSyncExternalStore(
    subscribeCluster,
    snapshotCluster,
    snapshotCluster,
  );

  const pluginSlotContext = useMemo(
    () =>
      Object.freeze({
        threadId,
        mainConversation: true,
      }),
    [threadId],
  );

  const total = todoUpdate?.todos.length ?? 0;
  const completed =
    todoUpdate?.todos.filter((item) => item.status === "completed").length ?? 0;
  const todoVisible = Boolean(todoUpdate && total > 0 && completed < total);

  const currentItem = todoUpdate?.todos.find(
    (item) => item.status === "in_progress",
  );
  const nextItem = todoUpdate?.todos.find((item) => item.status === "pending");
  const detailItems = [currentItem, nextItem].flatMap((item, index, items) =>
    item && items.findIndex((other) => other === item) === index ? [item] : [],
  );

  if (!visible) {
    return null;
  }
  if (!todoVisible && clusterContributions.length === 0) {
    return null;
  }

  return (
    <div
      className="jump-to-latest-cluster"
      aria-label={todoVisible ? t("app.currentPositionAndProgress") : undefined}
    >
      {todoVisible ? (
        <div
          className="jump-to-latest-progress"
          aria-label={t("app.todoProgressLabel", {
            completed: formatNumber(completed),
            total: formatNumber(total),
          })}
        >
          {t("app.progressFraction", {
            completed: formatNumber(completed),
            total: formatNumber(total),
          })}
          {detailItems.length > 0 ? (
            <span className="jump-to-latest-progress-detail" aria-hidden="true">
              {detailItems.map((item) => (
                <span
                  className={`jump-to-latest-progress-step ${item.status}`}
                  key={item.content}
                >
                  {t(
                    item.status === "in_progress"
                      ? "app.todoInProgress"
                      : "app.todoNext",
                    { content: item.content },
                  )}
                </span>
              ))}
            </span>
          ) : null}
        </div>
      ) : null}
      <PluginSlot host={host} id="composer.cluster" context={pluginSlotContext} />
    </div>
  );
}
