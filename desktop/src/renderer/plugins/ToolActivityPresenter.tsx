import { useSyncExternalStore, type ReactNode } from "react";

import type { ThreadItem } from "../../shared/protocol";
import type { ToolActivitySnapshot, ToolActivityStructuredResult } from "../../shared/workbench";
import { desktopPluginHost, desktopWorkbenchController } from "./DesktopPluginRuntime";
import type { PluginHost } from "./PluginHost";
import { PluginErrorBoundary, type WorkbenchController } from "./Workbench";

export interface ToolActivityPresenterProps {
  item?: ThreadItem;
  fallback: ReactNode;
  host?: PluginHost;
  controller?: WorkbenchController;
}

export function ToolActivityPresenter({
  item,
  fallback,
  host = desktopPluginHost,
  controller = desktopWorkbenchController,
}: ToolActivityPresenterProps): JSX.Element {
  const presenters = useSyncExternalStore(
    (listener) => host.subscribe(listener),
    () => host.getToolActivityPresenters(),
    () => host.getToolActivityPresenters(),
  );
  const dispatchKey = item?.display?.capability ?? item?.name;
  const presenter = dispatchKey === undefined
    ? undefined
    : presenters.find((candidate) => candidate.key === dispatchKey);
  if (!presenter || !item) return <>{fallback}</>;

  const Presenter = presenter.render;
  return (
    <PluginErrorBoundary
      pluginId={presenter.pluginId}
      generation={presenter.generation}
      services={controller.services}
      onUseDefault={() => undefined}
      fallback={fallback}
    >
      <Presenter
        activity={toToolActivitySnapshot(item)}
        host={controller.createRendererHostAPI(presenter.pluginId, presenter.generation)}
        fallback={fallback}
      />
    </PluginErrorBoundary>
  );
}

/** Convert the private thread record into the complete public presenter DTO. */
export function toToolActivitySnapshot(item: ThreadItem): ToolActivitySnapshot {
  const detail = item.result_detail;
  const structuredResult: ToolActivityStructuredResult | undefined = detail === undefined
    ? undefined
    : Object.freeze({
      content: detail.content === undefined
        ? undefined
        : Object.freeze(detail.content.map((part) => Object.freeze({
          type: part.type,
          text: part.text,
          data: part.data,
          mimeType: part.mime_type,
          uri: part.uri,
          name: part.name,
          resource: immutableValue(part.resource),
        }))),
      structuredContent: immutableValue(detail.structured_content),
      metadata: immutableValue(detail.meta),
      isError: detail.is_error,
      activity: detail.activity === undefined ? undefined : Object.freeze({
        id: detail.activity.id,
        kind: detail.activity.kind,
        state: detail.activity.state,
        threadId: detail.activity.thread_id,
        previewUri: detail.activity.preview_uri,
      }),
    });
  return Object.freeze({
    id: item.id,
    toolName: item.name ?? "",
    capability: item.display?.capability,
    kind: item.display?.kind,
    status: item.status === "in_progress"
      ? "running"
      : item.status === "failed" ? "failed" : "completed",
    argumentsText: item.arguments,
    resultText: item.result,
    structuredResult,
    error: item.error,
  });
}

function immutableValue(value: unknown): unknown {
  if (Array.isArray(value)) return Object.freeze(value.map(immutableValue));
  if (typeof value === "object" && value !== null) {
    return Object.freeze(Object.fromEntries(
      Object.entries(value).map(([key, entry]) => [key, immutableValue(entry)]),
    ));
  }
  return value;
}
