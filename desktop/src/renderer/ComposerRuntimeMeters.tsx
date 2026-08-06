import { useSyncExternalStore } from "react";
import type { TurnContextUsage } from "./AppState";
import { ComposerContextMeter } from "./ComposerContextMeter";
import { ComposerTokenGauge } from "./ComposerTokenGauge";
import { turnTelemetryStore } from "./TurnTelemetryStore";

export function ComposerRuntimeMeters({
  running,
  turnID,
  fallbackTokensPerSecond = 0,
  fallbackSampledAt,
  fallbackSource = "none",
  fallbackContextUsage,
}: {
  running: boolean;
  turnID?: string;
  fallbackTokensPerSecond?: number;
  fallbackSampledAt?: number;
  fallbackSource?: "real" | "estimated" | "none";
  fallbackContextUsage?: TurnContextUsage | null;
}): JSX.Element {
  const telemetry = useSyncExternalStore(
    turnTelemetryStore.subscribe,
    () => turnTelemetryStore.getSnapshot(turnID),
    () => turnTelemetryStore.getSnapshot(turnID),
  );
  const useLiveTelemetry = Boolean(turnID && telemetry.source !== "none");
  const contextUsage = telemetry.contextUsage
    ? {
        ...telemetry.contextUsage,
        requestContext: fallbackContextUsage?.requestContext,
      }
    : fallbackContextUsage ?? undefined;

  return (
    <>
      <ComposerTokenGauge
        running={running}
        tokensPerSecond={
          useLiveTelemetry ? telemetry.tokensPerSecond : fallbackTokensPerSecond
        }
        sampledAt={useLiveTelemetry ? telemetry.sampledAt : fallbackSampledAt}
        source={useLiveTelemetry ? telemetry.source : fallbackSource}
      />
      <ComposerContextMeter usage={contextUsage} />
    </>
  );
}
