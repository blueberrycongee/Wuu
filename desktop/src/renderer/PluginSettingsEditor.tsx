import { useEffect, useId, useState } from "react";
import type {
  ExtensionInventoryRecord,
  ExtensionSettingDescriptor,
} from "../shared/protocol";
import { useI18n } from "./i18n";

type SettingValue = boolean | string | number;

export function PluginSettingsEditor({
  plugin,
}: {
  plugin: ExtensionInventoryRecord;
}): JSX.Element | null {
  const settings = plugin.contributions?.settings ?? [];
  const approved = plugin.approval_state === "official" || plugin.approval_state === "granted";
  const fingerprint = plugin.fingerprint;

  if (!approved || plugin.enabled === false || !fingerprint || settings.length === 0) {
    return null;
  }

  return (
    <section className="plugin-settings-editor" aria-label={`${plugin.name} settings`}>
      {settings.map((setting) => (
        <PluginSettingControl
          key={setting.id}
          pluginId={plugin.id}
          fingerprint={fingerprint}
          setting={setting}
        />
      ))}
    </section>
  );
}

function PluginSettingControl({
  pluginId,
  fingerprint,
  setting,
}: {
  pluginId: string;
  fingerprint: string;
  setting: ExtensionSettingDescriptor;
}): JSX.Element {
  const { locale, t } = useI18n();
  const controlId = useId();
  const [draft, setDraft] = useState<SettingValue>(setting.default);
  const [committed, setCommitted] = useState<SettingValue>(setting.default);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    let cancelled = false;
    void load();
    return () => {
      cancelled = true;
    };

    async function load(): Promise<void> {
      setLoading(true);
      setError("");
      try {
        const result = await window.wuu.getPluginSetting({
          id: pluginId,
          fingerprint,
          key: setting.id,
        });
        if (!cancelled) {
          setDraft(result.value);
          setCommitted(result.value);
        }
      } catch (loadError) {
        if (!cancelled) {
          setError(errorMessage(loadError, t("skills.pluginSettingLoadFailed")));
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
  }, [fingerprint, locale, pluginId, setting.id]);

  async function save(value: SettingValue): Promise<void> {
    if (saving || Object.is(value, committed)) return;
    setSaving(true);
    setError("");
    setSaved(false);
    try {
      const result = await window.wuu.setPluginSetting({
        id: pluginId,
        fingerprint,
        key: setting.id,
        value,
      });
      setDraft(result.value);
      setCommitted(result.value);
      setSaved(true);
    } catch (saveError) {
      setError(errorMessage(saveError, t("skills.pluginSettingSaveFailed")));
    } finally {
      setSaving(false);
    }
  }

  function retry(): void {
    if (!loading && !Object.is(draft, committed)) {
      void save(draft);
      return;
    }
    setLoading(true);
    setError("");
    void window.wuu
      .getPluginSetting({ id: pluginId, fingerprint, key: setting.id })
      .then((result) => {
        setDraft(result.value);
        setCommitted(result.value);
      })
      .catch((retryError: unknown) => {
        setError(errorMessage(retryError, t("skills.pluginSettingLoadFailed")));
      })
      .finally(() => setLoading(false));
  }

  const descriptionId = `${controlId}-description`;
  const statusId = `${controlId}-status`;
  const describedBy = `${descriptionId} ${statusId}`;

  return (
    <div className="plugin-setting" data-setting-key={setting.id}>
      <div className="plugin-setting-heading">
        <label htmlFor={controlId}>{setting.title}</label>
        <span>{t(setting.scope === "workspace" ? "skills.pluginSettingWorkspace" : "skills.pluginSettingUser")}</span>
      </div>
      {setting.description ? <p id={descriptionId}>{setting.description}</p> : <span id={descriptionId} />}
      <div className="plugin-setting-control">
        {setting.type === "boolean" ? (
          <input
            id={controlId}
            type="checkbox"
            checked={Boolean(draft)}
            disabled={loading || saving}
            aria-describedby={describedBy}
            onChange={(event) => {
              const value = event.currentTarget.checked;
              setDraft(value);
              void save(value);
            }}
          />
        ) : setting.type === "enum" ? (
          <select
            id={controlId}
            value={String(draft)}
            disabled={loading || saving}
            aria-describedby={describedBy}
            onChange={(event) => {
              const value = event.currentTarget.value;
              setDraft(value);
              void save(value);
            }}
          >
            {(setting.enum ?? []).map((option) => <option key={option} value={option}>{option}</option>)}
          </select>
        ) : (
          <input
            id={controlId}
            type={setting.type === "number" ? "number" : "text"}
            value={String(draft)}
            disabled={loading || saving}
            aria-describedby={describedBy}
            onChange={(event) => {
              const raw = event.currentTarget.value;
              setDraft(setting.type === "number" ? Number(raw) : raw);
              setSaved(false);
            }}
            onBlur={() => void save(draft)}
          />
        )}
        <span className="plugin-setting-default">
          {t("skills.pluginSettingDefault", { value: String(setting.default) })}
        </span>
      </div>
      <div id={statusId} className={`plugin-setting-status${error ? " is-error" : ""}`} aria-live="polite">
        {error ? (
          <>
            <span>{error}</span>
            <button type="button" className="text-button" onClick={retry} disabled={loading || saving}>
              {t("skills.pluginSettingRetry")}
            </button>
          </>
        ) : saving ? t("skills.pluginSettingSaving") : setting.apply === "restart" ? (
          saved ? t("skills.pluginSettingRestartSaved") : t("skills.pluginSettingRestart")
        ) : saved ? t("skills.pluginSettingLiveSaved") : t("skills.pluginSettingLive")}
      </div>
    </div>
  );
}

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message.trim() ? error.message : fallback;
}
