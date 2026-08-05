import {
  Component,
  createElement,
  Fragment,
  useCallback,
  useSyncExternalStore,
  type ErrorInfo,
  type ReactNode,
} from "react";

import {
  type PluginHost,
  type PluginSlotId,
  type PluginSlotRenderContext,
  type RegisteredPluginSlotContribution,
} from "./PluginHost";

export interface PluginSlotProps {
  host: PluginHost;
  id: PluginSlotId;
  context?: PluginSlotRenderContext;
}

interface ContributionBoundaryProps {
  host: PluginHost;
  slotId: PluginSlotId;
  contribution: RegisteredPluginSlotContribution;
  children?: ReactNode;
}

interface ContributionBoundaryState {
  failed: boolean;
}

const EMPTY_CONTEXT: PluginSlotRenderContext = Object.freeze({});

class ContributionBoundary extends Component<ContributionBoundaryProps, ContributionBoundaryState> {
  state: ContributionBoundaryState = { failed: false };

  static getDerivedStateFromError(): ContributionBoundaryState {
    return { failed: true };
  }

  componentDidCatch(error: unknown, _errorInfo: ErrorInfo): void {
    const { contribution, host, slotId } = this.props;
    host.recordRenderFailure(contribution, slotId, error);
  }

  render(): ReactNode {
    return this.state.failed ? null : this.props.children;
  }
}

function ContributionContent({
  context,
  contribution,
}: {
  context: PluginSlotRenderContext;
  contribution: RegisteredPluginSlotContribution;
}): ReactNode {
  return contribution.render(context);
}

export function PluginSlot({ host, id, context = EMPTY_CONTEXT }: PluginSlotProps): ReactNode {
  const subscribe = useCallback((listener: () => void) => host.subscribeSlot(id, listener), [host, id]);
  const getSnapshot = useCallback(() => host.getSlotSnapshot(id), [host, id]);
  const contributions = useSyncExternalStore(subscribe, getSnapshot, getSnapshot);

  return createElement(
    Fragment,
    null,
    ...contributions.map((contribution) => createElement(
      ContributionBoundary,
      {
        key: `${contribution.pluginId}:${contribution.generation}:${contribution.id}`,
        host,
        slotId: id,
        contribution,
      },
      createElement(ContributionContent, { contribution, context }),
    )),
  );
}
