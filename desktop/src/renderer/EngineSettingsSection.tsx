import { useCallback, useEffect, useMemo, useState } from "react";
import type {
  EngineInfo,
  EngineListResult,
  EngineUpdateParams,
} from "../shared/protocol";
import { useI18n } from "./i18n";
import { SelectMenu } from "./SelectMenu";

/**
 * Agent Engines settings: enable/disable the external engines (codex,
 * claude), override their binary paths, and pick the default engine used
 * for new threads. The section is self-contained: it reads engine/list on
 * mount and pushes changes through engine/update.
 */
export function EngineSettingsSection(): JSX.Element {
  const { t } = useI18n();
  const [result, setResult] = useState<EngineListResult | undefined>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const refresh = useCallback(async () => {
    try {
      setResult(await window.wuu.listEngines());
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const save = useCallback(
    async (params: EngineUpdateParams) => {
      setBusy(true);
      setError("");
      try {
        setResult(await window.wuu.updateEngines(params));
      } catch (e) {
        setError(e instanceof Error ? e.message : String(e));
      } finally {
        setBusy(false);
      }
    },
    [],
  );

  const engines = useMemo(() => result?.engines ?? [], [result]);
  const settings = result?.settings;
  const defaultEngine = settings?.default_engine ?? "wuu";

  const engineById = useCallback(
    (id: string): EngineInfo | undefined => engines.find((e) => e.id === id),
    [engines],
  );
  const enabledIds = useMemo(
    () => engines.filter((e) => e.enabled && e.binary_ok).map((e) => e.id),
    [engines],
  );

  const renderEngineRow = (id: "codex" | "claude", label: string) => {
    const engine = engineById(id);
    const engineSettings = settings?.[id];
    const enabled = engine?.enabled ?? false;
    const binaryPath = engine?.binary_path ?? "";
    return (
      <div key={id} className="settings-row settings-row-block">
        <div className="settings-row-label">
          <span className="settings-row-label-title">{label}</span>
          <span className="settings-row-label-description">
            {engine?.error
              ? engine.error
              : enabled
                ? binaryPath || t("settings.engineAutoBinary")
                : t("settings.engineDisabled")}
          </span>
        </div>
        <div className="settings-row-control-block">
          <div className="settings-engine-path-row">
            <input
              key={`${id}-${engineSettings?.binary_path ?? ""}`}
              className="settings-text-input settings-engine-path-input"
              type="text"
              aria-label={`${label} ${t("settings.engineBinaryPath")}`}
              data-testid={`settings-engine-${id}-path`}
              placeholder={t("settings.engineBinaryPathPlaceholder")}
              defaultValue={engineSettings?.binary_path ?? ""}
              disabled={busy}
              onBlur={(event) => {
                const next = event.currentTarget.value.trim();
                if (next !== (engineSettings?.binary_path ?? "")) {
                  void save({ [id]: { binary_path: next } });
                }
              }}
            />
            <button
              className="settings-switch"
              type="button"
              role="switch"
              aria-checked={enabled}
              data-testid={`settings-engine-${id}-enabled`}
              disabled={busy}
              onClick={() => void save({ [id]: { enabled: !enabled } })}
            >
              <span className="settings-switch-thumb" aria-hidden="true" />
              <span className="sr-only">
                {enabled ? t("settings.engineDisable") : t("settings.engineEnable")}
              </span>
            </button>
          </div>
        </div>
      </div>
    );
  };

  return (
    <section
      className="settings-section"
      data-wuu-component="settings-section"
      data-testid="settings-agent-engines"
    >
      <header className="settings-section-header">
        <h2 className="settings-section-title">{t("settings.agentEngines")}</h2>
        <p className="settings-section-description">{t("settings.agentEnginesDescription")}</p>
      </header>
      <div className="settings-group" data-wuu-component="settings-group">
        <div className="settings-row settings-row-block">
          <div className="settings-row-label">
            <span className="settings-row-label-title">{t("settings.defaultEngine")}</span>
            <span className="settings-row-label-description">
              {t("settings.defaultEngineDescription")}
            </span>
          </div>
          <div className="settings-row-control-block">
            <SelectMenu
              className="settings-select"
              triggerClassName="settings-select-trigger"
              ariaLabel={t("settings.defaultEngine")}
              dataTestid="settings-default-engine"
              value={defaultEngine}
              disabled={busy}
              onChange={(next) => void save({ default_engine: next })}
              options={enabledIds.map((id) => ({
                value: id,
                label: id === "wuu" ? "Wuu" : id,
              }))}
            />
          </div>
        </div>
        {renderEngineRow("codex", "Codex")}
        {renderEngineRow("claude", "Claude Code")}
        {error ? <small className="settings-muted-line">{error}</small> : null}
      </div>
    </section>
  );
}
