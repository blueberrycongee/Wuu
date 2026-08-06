import { performance } from "node:perf_hooks";
import { appendTurnTokenSample, initialState } from "../src/renderer/AppState";

for (const telemetryCount of [500, 1_000, 2_000, 4_000]) {
  let state = initialState;
  const startedAt = performance.now();
  for (let index = 0; index < telemetryCount; index += 1) {
    state = appendTurnTokenSample(
      state,
      `telemetry-turn-${index}`,
      "thread-benchmark",
      index,
      index,
      0,
      0,
      index,
    );
  }
  const elapsed = performance.now() - startedAt;
  console.log(JSON.stringify({
    telemetryCount,
    retainedTelemetry: Object.keys(state.turnTokenUsage).length,
    elapsedMs: elapsed,
  }));
}
