import { useCallback, useEffect, useMemo, useState } from "react";
import type {
  EngineInfo,
  EngineListResult,
  EngineUpdateParams,
} from "../shared/protocol";
import { useI18n } from "./i18n";
import { SelectMenu } from "./SelectMenu";

const ENGINE_LABELS: Record<string, string> = {
  wuu: "Wuu",
  codex: "Codex",
  claude: "Claude Code",
};

const EXTERNAL_ENGINES = ["codex", "claude"] as const;

/**
 * Agent options inside the model services page. External agents (Codex,
 * Claude Code) are auto-detected and presented like model choices: they show
 * up when their CLI is installed and disappear otherwise, with no setup
 * required. Only the default selection is a first-class setting; binary path
 * overrides and explicit enable/disable live behind the advanced disclosure.
 */
export function EngineSettingsSection(): JSX.Element {
  const { t } = useI18n();
  const [result, setResult] = useState<EngineListResult | undefined>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [advancedOpen, setAdvancedOpen] = useState(false);

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

  const save = useCallback(async (params: EngineUpdateParams) => {
    setBusy(true);
    setError("");
    try {
      setResult(await window.wuu.updateEngines(params));
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }, []);

  const engines = useMemo(() => result?.engines ?? [], [result]);
  const settings = result?.settings;
  const defaultEngine = settings?.default_engine ?? "wuu";
  const engineById = useCallback(
    (id: string): EngineInfo | undefined => engines.find((e) => e.id === id),
    [engines],
  );
  const selectableIds = useMemo(
    () => engines.filter((e) => e.enabled && e.binary_ok).map((e) => e.id),
    [engines],
  );

  // One-line state shown under each auto-detected agent.
  const statusLine = (engine: EngineInfo | undefined): string => {
    if (!engine || !engine.enabled) return t("settings.engineDisabled");
    if (!engine.binary_ok) return engine.error || t("settings.engineNotInstalled");
    return engine.binary_path || t("settings.engineAutoBinary");
  };

  const statusFlag = (
    engine: EngineInfo | undefined,
  ): { text: string; className: string } => {
    if (engine?.enabled && engine?.binary_ok) {
      return { text: t("settings.engineReady"), className: "settings-inline-flag success" };
    }
    if (!engine?.enabled) {
      return { text: t("settings.engineDisabled"), className: "settings-inline-flag" };
    }
    return { text: t("settings.engineNotInstalled"), className: "settings-inline-flag" };
  };

  return (
    <section
      className="settings-section"
      data-wuu-component="settings-section"
      data-testid="settings-agent-engines"
    >
      <header className="settings-section-header">
        <h2 className="settings-section-title">{t("settings.agentEngines")}</h2>
        <p className="settings-section-description">
          {t("settings.agentEnginesDescription")}
        </p>
      </header>
      <div className="settings-group" data-wuu-component="settings-group">
        {result === undefined ? (
          <div className="settings-row">
            <small className="settings-muted-line">{t("settings.engineDetecting")}</small>
          </div>
        ) : (
          <>
            <div className="settings-row settings-row-block">
              <div className="settings-row-label">
                <span className="settings-row-label-title">
                  {t("settings.defaultEngine")}
                </span>
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
                  options={selectableIds.map((id) => ({
                    value: id,
                    label: ENGINE_LABELS[id] ?? id,
                  }))}
                />
              </div>
            </div>
            {EXTERNAL_ENGINES.map((id) => {
              const engine = engineById(id);
              const flag = statusFlag(engine);
              const notInstalled = engine?.enabled && !engine?.binary_ok;
              return (
                <div
                  key={id}
                  className="settings-row settings-row-block"
                  data-testid={`settings-engine-${id}-status`}
                >
                  <div className="settings-row-label">
                    <span className="settings-row-label-title">{ENGINE_LABELS[id]}</span>
                    <span className="settings-row-label-description">
                      {statusLine(engine)}
                      {notInstalled ? ` · ${t("settings.engineNotInstalledHint")}` : ""}
                    </span>
                  </div>
                  <div className="settings-row-control-block">
                    <span className={flag.className}>{flag.text}</span>
                  </div>
                </div>
              );
            })}
          </>
        )}
      </div>
      <button
        className="settings-button settings-engine-advanced-toggle"
        type="button"
        aria-expanded={advancedOpen}
        data-testid="settings-engine-advanced-toggle"
        onClick={() => setAdvancedOpen((open) => !open)}
      >
        {advancedOpen ? t("settings.engineAdvancedHide") : t("settings.engineAdvanced")}
      </button>
      {advancedOpen ? (
        <div className="settings-group" data-wuu-component="settings-group">
          {EXTERNAL_ENGINES.map((id) => {
            const engineSettings = settings?.[id];
            const enabled = engineById(id)?.enabled ?? false;
            return (
              <div key={id} className="settings-row settings-row-block">
                <div className="settings-row-label">
                  <span className="settings-row-label-title">{ENGINE_LABELS[id]}</span>
                  <span className="settings-row-label-description">
                    {t("settings.engineBinaryPath")}
                  </span>
                </div>
                <div className="settings-row-control-block">
                  <input
                    key={`${id}-${engineSettings?.binary_path ?? ""}`}
                    className="settings-input settings-engine-path-input"
                    type="text"
                    aria-label={`${ENGINE_LABELS[id]} ${t("settings.engineBinaryPath")}`}
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
            );
          })}
        </div>
      ) : null}
      {error ? <small className="settings-muted-line">{error}</small> : null}
    </section>
  );
}
