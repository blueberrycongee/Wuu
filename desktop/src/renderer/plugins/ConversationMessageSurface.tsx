import type { ReactNode } from "react";

import { desktopPluginHost } from "./DesktopPluginRuntime";
import type { PluginHost } from "./PluginHost";
import { PluginSurface } from "./PluginSurface";

export type ConversationMessageSurfaceContext = Readonly<Record<string, unknown>> & {
  readonly version: 1;
  readonly messageId: string;
  readonly turnId: string;
  readonly threadId?: string;
  readonly kind: "user-message" | "assistant-message" | "reasoning" | "notice";
  readonly status?: string;
  readonly phase?: string;
  readonly streaming: boolean;
  readonly attachmentCount: number;
  readonly actions: Readonly<{
    edit?: () => void;
    fork?: () => void;
  }>;
};

interface ConversationMessageSurfaceProps {
  context: ConversationMessageSurfaceContext;
  fallback: ReactNode;
  host?: PluginHost;
}

/** Semantic message boundary. Its context deliberately contains no private ThreadItem record. */
export function ConversationMessageSurface({
  context,
  fallback,
  host = desktopPluginHost,
}: ConversationMessageSurfaceProps): ReactNode {
  return (
    <PluginSurface
      host={host}
      id="conversation.message"
      context={context}
      fallback={fallback}
    />
  );
}
