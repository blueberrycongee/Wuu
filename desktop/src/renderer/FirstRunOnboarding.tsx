import { Check, ChevronLeft, LoaderCircle, Plug, Sparkles } from "lucide-react";
import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import type {
  ExtensionInventoryRecord,
  ExtensionPackageUpdateParams,
  ProviderSummary,
  RuntimeConnectionUpdate,
} from "../shared/protocol";
import { useI18n } from "./i18n";
import type { TranslationKey } from "./i18n/resources/zh-CN";
import { PluginIcon } from "./PublicIcon";
import { applyThemePreference } from "./Theme";
import { WuuMascot } from "./WuuMascot";

type OnboardingStep = "welcome" | "plugins" | "provider" | "ready";
type PluginPreset = "minimal" | "recommended" | "all" | "custom";

const STEP_ORDER: readonly OnboardingStep[] = ["welcome", "plugins", "provider", "ready"];
const BUNDLED_PLUGIN_ORDER = [
  "ask-user",
  "todo",
  "automation",
  "subagent",
  "memory",
  "dream",
  "note-compaction",
] as const;
const RECOMMENDED_PLUGIN_IDS = new Set<string>([
  "todo",
  "automation",
  "subagent",
]);

const PLUGIN_DESCRIPTION_KEYS: Readonly<Record<string, TranslationKey>> = {
  "ask-user": "onboarding.plugin.askUser",
  todo: "onboarding.plugin.todo",
  automation: "onboarding.plugin.automation",
  subagent: "onboarding.plugin.subagent",
  memory: "onboarding.plugin.memory",
  dream: "onboarding.plugin.dream",
  "note-compaction": "onboarding.plugin.noteCompaction",
};

export function bundledOnboardingPlugins(
  inventory: readonly ExtensionInventoryRecord[] | undefined,
): ExtensionInventoryRecord[] {
  if (!inventory) return [];
  const order = new Map<string, number>(
    BUNDLED_PLUGIN_ORDER.map((id, index) => [id, index]),
  );
  return inventory
    .filter(
      (record) =>
        record.kind === "plugin" &&
        record.package_source === "bundled" &&
        record.provenance.official === true &&
        order.has(record.provenance.plugin_id ?? ""),
    )
    .sort(
      (left, right) =>
        (order.get(left.provenance.plugin_id ?? "") ?? 99) -
        (order.get(right.provenance.plugin_id ?? "") ?? 99),
    );
}

export function hasOnboardingProvider(
  providers: readonly ProviderSummary[] | undefined,
): boolean {
  return providers?.some(
    (provider) => provider.api_key_configured === true || provider.connection_locked === true,
  ) ?? false;
}

export function FirstRunOnboarding({
  inventory,
  providers,
  onUpdateExtensionPackage,
  onSaveProvider,
  onComplete,
}: {
  inventory?: readonly ExtensionInventoryRecord[];
  providers?: readonly ProviderSummary[];
  onUpdateExtensionPackage: (update: ExtensionPackageUpdateParams) => Promise<void>;
  onSaveProvider: (
    provider: string,
    model: string,
    connection: RuntimeConnectionUpdate,
  ) => Promise<void>;
  onComplete: () => Promise<void>;
}): JSX.Element {
  const { t } = useI18n();
  const bundledPlugins = useMemo(() => bundledOnboardingPlugins(inventory), [inventory]);
  const [step, setStep] = useState<OnboardingStep>("welcome");
  const [selectedPluginIDs, setSelectedPluginIDs] = useState<Set<string>>(
    () => recommendedPluginSubjectIDs(bundledPlugins),
  );
  const initializedPluginSelection = useRef(bundledPlugins.length > 0);
  const [applyingPlugins, setApplyingPlugins] = useState(false);
  const [providerName, setProviderName] = useState("");
  const [providerType, setProviderType] = useState("openai-compatible");
  const [model, setModel] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [savingProvider, setSavingProvider] = useState(false);
  const [finishing, setFinishing] = useState(false);
  const [error, setError] = useState("");
  const providerReady = hasOnboardingProvider(providers);
  const preset = selectedPreset(selectedPluginIDs, bundledPlugins);
  const currentStepIndex = STEP_ORDER.indexOf(step);

  useLayoutEffect(() => {
    const root = document.documentElement;
    root.dataset.theme = "light";
    return () => {
      applyThemePreference(window.wuu?.initialThemePreference ?? "system");
    };
  }, []);

  useEffect(() => {
    if (initializedPluginSelection.current || bundledPlugins.length === 0) return;
    initializedPluginSelection.current = true;
    setSelectedPluginIDs(recommendedPluginSubjectIDs(bundledPlugins));
  }, [bundledPlugins]);

  function choosePreset(next: Exclude<PluginPreset, "custom">): void {
    if (next === "minimal") {
      setSelectedPluginIDs(new Set());
      return;
    }
    if (next === "all") {
      setSelectedPluginIDs(new Set(bundledPlugins.map((plugin) => plugin.id)));
      return;
    }
    setSelectedPluginIDs(
      recommendedPluginSubjectIDs(bundledPlugins),
    );
  }

  function togglePlugin(id: string): void {
    setSelectedPluginIDs((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  async function applyPluginChoices(): Promise<void> {
    if (applyingPlugins || bundledPlugins.length === 0) return;
    setApplyingPlugins(true);
    setError("");
    try {
      for (const plugin of bundledPlugins) {
        const shouldEnable = selectedPluginIDs.has(plugin.id);
        const enabled = plugin.enabled !== false;
        if (shouldEnable !== enabled) {
          await onUpdateExtensionPackage({
            id: plugin.id,
            action: shouldEnable ? "enable" : "disable",
          });
        }
      }
      setStep("provider");
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : t("onboarding.pluginsFailed"));
    } finally {
      setApplyingPlugins(false);
    }
  }

  async function saveProvider(): Promise<void> {
    const name = providerName.trim();
    const providerModel = model.trim();
    const key = apiKey.trim();
    if (!name || !providerModel || !key || savingProvider) return;
    setSavingProvider(true);
    setError("");
    try {
      await onSaveProvider(name, providerModel, {
        type: providerType,
        create_provider: true,
        api_key: key,
      });
      setStep("ready");
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : t("provider.saveFailed"));
    } finally {
      setSavingProvider(false);
    }
  }

  async function finish(): Promise<void> {
    if (finishing) return;
    setFinishing(true);
    setError("");
    try {
      await onComplete();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : t("onboarding.finishFailed"));
      setFinishing(false);
    }
  }

  return (
    <main className="first-run-onboarding" data-testid="first-run-onboarding">
      <header className="onboarding-chrome">
        <span className="onboarding-wordmark">wuu</span>
        <div className="onboarding-progress" aria-label={t("onboarding.progress")}> 
          {STEP_ORDER.map((item, index) => (
            <span
              key={item}
              className={`onboarding-progress-dot${index <= currentStepIndex ? " is-active" : ""}`}
            />
          ))}
        </div>
      </header>

      <section className={`onboarding-stage onboarding-stage-${step}`}>
        {step === "welcome" ? (
          <div className="onboarding-welcome">
            <WuuMascot
              className="onboarding-mascot onboarding-mascot-hero"
              size={152}
              activity="compose"
              accessory="none"
              followPointer
              aria-hidden="true"
            />
            <h1>{t("onboarding.welcomeTitle")}</h1>
            <button className="onboarding-primary" type="button" onClick={() => setStep("plugins")}>
              {t("onboarding.begin")}
            </button>
          </div>
        ) : null}

        {step === "plugins" ? (
          <div className="onboarding-panel onboarding-plugins-panel">
            <div className="onboarding-panel-heading">
              <div>
                <p className="onboarding-eyebrow">{t("onboarding.pluginsEyebrow")}</p>
                <h1>{t("onboarding.pluginsTitle")}</h1>
                <p>{t("onboarding.pluginsDescription")}</p>
              </div>
              <WuuMascot className="onboarding-mascot" size={88} activity="tool" accessory="none" aria-hidden="true" />
            </div>

            <div className="onboarding-presets" aria-label={t("onboarding.presets")}> 
              {(["minimal", "recommended", "all"] as const).map((item) => (
                <button
                  key={item}
                  type="button"
                  className={preset === item ? "is-selected" : ""}
                  aria-pressed={preset === item}
                  onClick={() => choosePreset(item)}
                >
                  {t(`onboarding.preset.${item}`)}
                </button>
              ))}
            </div>

            {inventory === undefined ? (
              <div className="onboarding-loading" role="status">
                <LoaderCircle className="is-spinning" />
                {t("onboarding.loadingPlugins")}
              </div>
            ) : bundledPlugins.length === 0 ? (
              <div className="onboarding-loading" role="alert">
                {t("onboarding.pluginsUnavailable")}
              </div>
            ) : (
              <div className="onboarding-plugin-grid">
                {bundledPlugins.map((plugin) => {
                  const selected = selectedPluginIDs.has(plugin.id);
                  const pluginID = plugin.provenance.plugin_id ?? "";
                  const descriptionKey = PLUGIN_DESCRIPTION_KEYS[pluginID];
                  return (
                    <button
                      key={plugin.id}
                      type="button"
                      className={`onboarding-plugin${selected ? " is-selected" : ""}`}
                      aria-pressed={selected}
                      onClick={() => togglePlugin(plugin.id)}
                    >
                      <span className="onboarding-plugin-icon">
                        <PluginIcon
                          icon={plugin.icon}
                          pluginId={plugin.id}
                          fingerprint={plugin.fingerprint ?? ""}
                        />
                      </span>
                      <span className="onboarding-plugin-copy">
                        <strong>{plugin.name}</strong>
                        <span>{descriptionKey ? t(descriptionKey) : plugin.description}</span>
                      </span>
                      <span className="onboarding-plugin-check" aria-hidden="true">
                        {selected ? <Check /> : null}
                      </span>
                    </button>
                  );
                })}
              </div>
            )}

            <OnboardingError message={error} />
            <div className="onboarding-actions">
              <button className="onboarding-back" type="button" onClick={() => setStep("welcome")}>
                <ChevronLeft />{t("onboarding.back")}
              </button>
              <button
                className="onboarding-primary"
                type="button"
                disabled={applyingPlugins || bundledPlugins.length === 0}
                onClick={() => void applyPluginChoices()}
              >
                {applyingPlugins ? t("onboarding.applying") : t("onboarding.continue")}
              </button>
            </div>
          </div>
        ) : null}

        {step === "provider" ? (
          <div className={`onboarding-panel onboarding-provider-panel${providerReady ? " is-ready" : ""}`}>
            <div className="onboarding-provider-mark">
              <WuuMascot className="onboarding-mascot" size={104} activity="thinking" accessory="none" aria-hidden="true" />
              <span className="onboarding-provider-plug"><Plug /></span>
            </div>
            <p className="onboarding-eyebrow">{t("onboarding.providerEyebrow")}</p>
            <h1>{providerReady ? t("onboarding.providerReadyTitle") : t("onboarding.providerTitle")}</h1>
            <p className="onboarding-provider-description">
              {providerReady ? t("onboarding.providerReadyDescription") : t("onboarding.providerDescription")}
            </p>

            {!providerReady ? (
              <div className="onboarding-provider-form">
                <label>
                  <span>{t("provider.identifier")}</span>
                  <input value={providerName} onChange={(event) => setProviderName(event.currentTarget.value)} placeholder="openai" autoFocus />
                </label>
                <label>
                  <span>{t("provider.type")}</span>
                  <select value={providerType} onChange={(event) => setProviderType(event.currentTarget.value)}>
                    <option value="openai-compatible">{t("provider.openaiCompatible")}</option>
                    <option value="anthropic">{t("provider.anthropicCompatible")}</option>
                  </select>
                </label>
                <label>
                  <span>{t("provider.modelName")}</span>
                  <input value={model} onChange={(event) => setModel(event.currentTarget.value)} placeholder="gpt-4o" />
                </label>
                <label>
                  <span>{t("provider.apiKey")}</span>
                  <input type="password" value={apiKey} onChange={(event) => setApiKey(event.currentTarget.value)} />
                </label>
              </div>
            ) : null}

            <OnboardingError message={error} />
            <div className="onboarding-actions">
              <button className="onboarding-back" type="button" onClick={() => setStep("plugins")}>
                <ChevronLeft />{t("onboarding.back")}
              </button>
              <div className="onboarding-action-group">
                {!providerReady ? (
                  <button className="onboarding-secondary" type="button" onClick={() => setStep("ready")}>
                    {t("onboarding.configureLater")}
                  </button>
                ) : null}
                <button
                  className="onboarding-primary"
                  type="button"
                  disabled={savingProvider || (!providerReady && (!providerName.trim() || !model.trim() || !apiKey.trim()))}
                  onClick={() => providerReady ? setStep("ready") : void saveProvider()}
                >
                  {savingProvider ? t("onboarding.savingProvider") : t("onboarding.continue")}
                </button>
              </div>
            </div>
          </div>
        ) : null}

        {step === "ready" ? (
          <div className="onboarding-welcome onboarding-ready">
            <div className="onboarding-ready-mascot">
              <WuuMascot className="onboarding-mascot onboarding-mascot-hero" size={144} activity="compose" accessory="sprout" aria-hidden="true" />
              <Sparkles aria-hidden="true" />
            </div>
            <p className="onboarding-eyebrow">{t("onboarding.readyEyebrow")}</p>
            <h1>{t("onboarding.readyTitle")}</h1>
            <p className="onboarding-lede">{t("onboarding.readyDescription", { count: selectedPluginIDs.size })}</p>
            <OnboardingError message={error} />
            <button className="onboarding-primary" type="button" disabled={finishing} onClick={() => void finish()}>
              {finishing ? t("onboarding.finishing") : t("onboarding.enterWuu")}
            </button>
          </div>
        ) : null}
      </section>
    </main>
  );
}

function selectedPreset(
  selected: ReadonlySet<string>,
  plugins: readonly ExtensionInventoryRecord[],
): PluginPreset {
  if (selected.size === 0) return "minimal";
  const available = new Set(plugins.map((plugin) => plugin.id));
  if (available.size > 0 && selected.size === available.size && [...available].every((id) => selected.has(id))) {
    return "all";
  }
  const recommended = [...recommendedPluginSubjectIDs(plugins)];
  if (selected.size === recommended.length && recommended.every((id) => selected.has(id))) {
    return "recommended";
  }
  return "custom";
}

function recommendedPluginSubjectIDs(
  plugins: readonly ExtensionInventoryRecord[],
): Set<string> {
  return new Set(
    plugins
      .filter((plugin) => RECOMMENDED_PLUGIN_IDS.has(plugin.provenance.plugin_id ?? ""))
      .map((plugin) => plugin.id),
  );
}

function OnboardingError({ message }: { message: string }): JSX.Element | null {
  if (!message) return null;
  return <p className="onboarding-error" role="alert">{message}</p>;
}
