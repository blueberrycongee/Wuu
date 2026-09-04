import {
  Bug,
  Check,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  CircleHelp,
  Cpu,
  Eye,
  FileText,
  FlaskConical,
  FoldVertical,
  Folder,
  FolderOpen,
  FolderPlus,
  FolderX,
  Gauge,
  GitBranch,
  GitCommitHorizontal,
  GitCompare,
  GitPullRequest,
  Hammer,
  Lock,
  MessageSquarePlus,
  Paperclip,
  PieChart,
  Plus,
  Puzzle,
  RotateCcw,
  ScrollText,
  Search,
  Settings,
  Shield,
  Terminal,
  TriangleAlert,
  Zap,
  type LucideIcon
} from "lucide-react";
import { type CSSProperties, type RefObject, useEffect, useRef, useState } from "react";
import type {
  CodexModelSummary,
  DesktopProject,
  EngineInfo,
  EngineModelInfo,
  GitStatusResult,
  InitializeResult,
  PermissionSummary,
  ProviderModelSummary,
  ProviderSummary,
  RuntimeContext
} from "../shared/protocol";
import { FloatingMenuPortal, isInsideFloatingMenu } from "./ComposerFloatingMenu";
import type { ComposerSlashCommand } from "./ComposerSlashCommands";
import type {
  CodexModelLoadState,
  CodexRuntimeMenu,
  ComposerVariant,
  FloatingMenuPlacement,
  PermissionMode
} from "./ComposerTypes";
import {
  codexEffortOptions,
  displayCodexModelName,
  isCodexProvider,
  providerIsCodex,
  providerModelDisplayName,
  providerModelVariantOptions,
  shortCodexModelLabel,
  variantLabel
} from "./RuntimeHelpers";
import { translateCurrent as translate, useI18n } from "./i18n";
import { Tooltip } from "./Tooltip";

type ChipTone = "neutral" | "danger";

// Runtime engine display names. Models and effort remain subordinate to the
// selected engine because each engine owns a different runtime contract.
const ENGINE_LABELS: Record<string, string> = {
  wuu: "Wuu",
  codex: "Codex",
  claude: "Claude Code"
};

function engineLabel(id: string): string {
  return ENGINE_LABELS[id] ?? id;
}

type EngineOption = {
  id: string;
  label: string;
};

// Available engine choices shown in the runtime picker. The built-in wuu
// engine is always present; external engines appear only when auto-detected
// (enabled and binary found). An active external engine is kept in the list
// even when it just became unavailable, so the selection stays visible.
function availableEngineOptions(
  engines: EngineInfo[] | undefined,
  activeEngine: string
): EngineOption[] {
  const options: EngineOption[] = [{ id: "wuu", label: engineLabel("wuu") }];
  for (const engine of engines ?? []) {
    if (engine.id !== "codex" && engine.id !== "claude") continue;
    if (!engine.enabled || (!engine.binary_ok && engine.id !== activeEngine)) continue;
    options.push({ id: engine.id, label: engineLabel(engine.id) });
  }
  return options;
}

function EngineOptionsMenu({
  options,
  selected,
  locked,
  running,
  lockedDescription,
  onSelect
}: {
  options: EngineOption[];
  selected: string;
  locked: boolean;
  running: boolean;
  lockedDescription?: string;
  onSelect: (id: string) => void;
}): JSX.Element {
  const { t } = useI18n();
  return (
    <div
      className={`runtime-engine-options${locked ? " is-locked" : ""}`}
      role="group"
      aria-label={locked && lockedDescription ? lockedDescription : t("runtime.engine")}
    >
      {options.map((option) => {
        const isSelected = option.id === selected;
        return (
          <button
            className="runtime-engine-option"
            key={option.id}
            role="menuitemradio"
            type="button"
            disabled={locked || running}
            aria-checked={isSelected}
            onClick={() => {
              if (!isSelected) onSelect(option.id);
            }}
          >
            <span className="runtime-engine-option-name">{option.label}</span>
            {locked && isSelected ? <Lock aria-hidden="true" /> : isSelected ? <Check aria-hidden="true" /> : null}
          </button>
        );
      })}
    </div>
  );
}

type RuntimePanelView = "summary" | "engines" | "providers" | "models";
type RuntimePanelDirection = "forward" | "back";

function RuntimePanelHeader({ title, onBack }: { title: string; onBack: () => void }): JSX.Element {
  const { t } = useI18n();
  return (
    <div className="runtime-panel-header">
      <button type="button" className="runtime-panel-back" aria-label={t("common.back")} onClick={onBack}>
        <ChevronLeft aria-hidden="true" />
      </button>
      <span>{title}</span>
    </div>
  );
}

function RuntimePanelSummary({
  engine,
  provider,
  engineLocked,
  model,
  effortOptions,
  selectedEffort,
  effortDisabled,
  onOpenEngines,
  onOpenProviders,
  onOpenModels,
  onSelectEffort
}: {
  engine: string;
  provider?: string;
  engineLocked: boolean;
  model: string;
  effortOptions: string[];
  selectedEffort: string;
  effortDisabled: boolean;
  onOpenEngines: () => void;
  onOpenProviders?: () => void;
  onOpenModels: () => void;
  onSelectEffort: (effort: string) => void;
}): JSX.Element {
  const [previewEffort, setPreviewEffort] = useState(selectedEffort);

  useEffect(() => {
    setPreviewEffort(selectedEffort);
  }, [selectedEffort]);

  return (
    <div className="runtime-panel-summary">
      <div className="runtime-panel-context">
        <button type="button" onClick={onOpenEngines}>
          <span>{engineLabel(engine)}</span>
          {engineLocked ? <Lock aria-hidden="true" /> : <ChevronRight aria-hidden="true" />}
        </button>
        {provider && onOpenProviders ? (
          <>
            <span className="runtime-panel-context-separator" aria-hidden="true">/</span>
            <button type="button" onClick={onOpenProviders}>
              <span>{provider}</span>
              <ChevronRight aria-hidden="true" />
            </button>
          </>
        ) : null}
      </div>
      <button type="button" className="runtime-panel-model" onClick={onOpenModels}>
        <span className="runtime-panel-model-name">{model}</span>
        <span className="runtime-panel-effort-value">{variantLabel(previewEffort)}</span>
        <ChevronRight aria-hidden="true" />
      </button>
      {effortOptions.length > 1 ? (
        <EffortSelector
          options={effortOptions}
          selectedVariant={selectedEffort}
          disabled={effortDisabled}
          onPreviewEffort={setPreviewEffort}
          onSelectEffort={onSelectEffort}
        />
      ) : null}
    </div>
  );
}

type PermissionModeState = PermissionMode;

type PermissionModeOption = {
  mode: PermissionMode;
  label: string;
  chipLabel: string;
  icon: LucideIcon;
  tone: ChipTone;
};

function permissionModeLabels(engine?: string): Record<PermissionMode, { label: string; chipLabel: string }> {
  switch (engine?.trim().toLowerCase()) {
    case "codex":
      return {
        standard: { label: "Workspace Write", chipLabel: "Workspace Write" },
        read_only: { label: "Read Only", chipLabel: "Read Only" },
        unconfined: { label: "Danger Full Access", chipLabel: "Danger Full Access" }
      };
    case "claude":
      return {
        standard: { label: "Don't Ask", chipLabel: "Don't Ask" },
        read_only: { label: "Plan", chipLabel: "Plan" },
        unconfined: { label: "Bypass Permissions", chipLabel: "Bypass Permissions" }
      };
    default:
      return {
        standard: {
          label: translate("runtime.permission.standardLabel"),
          chipLabel: translate("runtime.permission.standard")
        },
        read_only: {
          label: translate("runtime.permission.readOnly"),
          chipLabel: translate("runtime.permission.readOnly")
        },
        unconfined: {
          label: translate("runtime.permission.unconfined"),
          chipLabel: translate("runtime.permission.unconfined")
        }
      };
  }
}

function permissionModeOptions(engine?: string): PermissionModeOption[] {
  const labels = permissionModeLabels(engine);
  return [
    {
      mode: "standard",
      ...labels.standard,
      icon: Shield,
      tone: "neutral"
    },
    {
      mode: "read_only",
      ...labels.read_only,
      icon: Eye,
      tone: "neutral"
    },
    {
      mode: "unconfined",
      ...labels.unconfined,
      icon: TriangleAlert,
      tone: "danger"
    }
  ];
}

export function permissionModeFromSummary(permissions?: PermissionSummary): PermissionModeState {
  const mode = permissions?.mode?.trim();
  switch (mode) {
    case "read_only":
      return "read_only";
    case "unconfined":
      return "unconfined";
    case "standard":
      return "standard";
    default:
      return "standard";
  }
}

export function permissionModeHasAdvancedOverrides(_permissions?: PermissionSummary): boolean {
  return false;
}

export function permissionModeOption(mode: PermissionModeState, engine?: string): Omit<PermissionModeOption, "mode"> & { mode: PermissionModeState } {
  const options = permissionModeOptions(engine);
  return options.find((option) => option.mode === mode) ?? options[0];
}

export function RuntimePicker({
  variant,
  initialized,
  state,
  openMenu,
  anchorRef,
  running,
  engines,
  activeEngine,
  engineLocked,
  engineModel,
  engineEffort,
  onSelectEngine,
  onSelectEngineModel,
  onSelectEngineEffort,
  onToggleMenu,
  onSelectModel,
  onSelectEffort
}: {
  variant: ComposerVariant;
  initialized: InitializeResult;
  state: CodexModelLoadState;
  openMenu: CodexRuntimeMenu;
  anchorRef: RefObject<HTMLDivElement | null>;
  running: boolean;
  engines?: EngineInfo[];
  activeEngine?: string;
  engineLocked?: boolean;
  engineModel?: string;
  engineEffort?: string;
  onSelectEngine?: (id: string) => void;
  onSelectEngineModel?: (model: string, effort: string) => void;
  onSelectEngineEffort?: (effort: string) => void;
  onToggleMenu: (menu: Exclude<CodexRuntimeMenu, null>) => void;
  onSelectModel: (provider: string, model: string, variant?: string) => void | Promise<boolean>;
  onSelectEffort: (variant: string) => void | Promise<boolean>;
}): JSX.Element {
  const { t } = useI18n();
  const currentProvider = initialized.providers?.find((provider) => provider.name === initialized.provider);
  const codexProvider = isCodexProvider(initialized);
  const currentCodexModel = codexProvider ? state.models.find((model) => model.slug === initialized.model) : undefined;
  const currentProviderModel = currentProvider?.models?.find((model) => model.id === initialized.model);
  const currentVariant = initialized.variant ?? initialized.effort ?? "";
  const placement: FloatingMenuPlacement = variant === "hero" ? "below" : "above";
  const externalEngine = activeEngine && activeEngine !== "wuu" ? activeEngine : "";
  const engineOptions = availableEngineOptions(engines, externalEngine);
  const selectedEngine = externalEngine || "wuu";
  const externalEngineInfo = engines?.find((engine) => engine.id === externalEngine);
  const externalModelInfo = externalEngineInfo?.models?.find((model) => model.id === engineModel);
  return (
    <div className="codex-runtime-anchor" ref={anchorRef}>
      <Tooltip content={running ? t("runtime.modelSwitchWhileRunning") : undefined}>
        <button
          className="codex-runtime-trigger"
          type="button"
          disabled={running}
          aria-haspopup="menu"
          aria-expanded={openMenu === "model"}
          onClick={() => onToggleMenu("model")}
        >
          <span>
            {externalEngine
              ? `${engineLabel(externalEngine)} · ${externalModelInfo?.display_name || engineModel || t("runtime.selectModel")}`
              : `${engineLabel("wuu")} · ${runtimeTriggerLabel(initialized, currentProviderModel, currentCodexModel)}`}
          </span>
          <span className="codex-runtime-effort">
            {variantLabel(externalEngine ? engineEffort ?? "" : currentVariant)}
          </span>
          <ChevronDown className="icon" />
        </button>
      </Tooltip>
      {openMenu === "model" ? (
        <FloatingMenuPortal
          anchorRef={anchorRef}
          owner="codex-runtime"
          placement={placement}
          align="right"
          width={320}
          flip
        >
          {externalEngine ? (
            <EngineRuntimeMenu
              engine={externalEngineInfo}
              selectedModel={engineModel ?? ""}
              selectedEffort={engineEffort ?? ""}
              disabled={running || Boolean(engineLocked)}
              onSelectModel={(model, effort) => onSelectEngineModel?.(model, effort)}
              onSelectEffort={(effort) => onSelectEngineEffort?.(effort)}
              engineOptions={engineOptions}
              selectedEngine={selectedEngine}
              engineLocked={Boolean(engineLocked)}
              running={running}
              lockedDescription={engineLocked ? t("runtime.engineLockedDescription") : undefined}
              onSelectEngine={(id) => onSelectEngine?.(id)}
            />
          ) : (
            <RuntimeModelMenu
              initialized={initialized}
              state={state}
              selectedProvider={initialized.provider}
              selectedModel={initialized.model}
              selectedVariant={currentVariant}
              onSelectModel={onSelectModel}
              onSelectEffort={onSelectEffort}
              engineOptions={engineOptions}
              selectedEngine={selectedEngine}
              engineLocked={Boolean(engineLocked)}
              running={running}
              lockedDescription={engineLocked ? t("runtime.engineLockedDescription") : undefined}
              onSelectEngine={(id) => onSelectEngine?.(id)}
            />
          )}
        </FloatingMenuPortal>
      ) : null}
    </div>
  );
}

function engineModelDefaultEffort(model: EngineModelInfo): string {
  const supported = model.supported_efforts ?? [];
  if (model.default_effort && supported.includes(model.default_effort)) {
    return model.default_effort;
  }
  if (supported.includes("medium")) return "medium";
  return supported[0] ?? "";
}

function EngineRuntimeMenu({
  engine,
  selectedModel,
  selectedEffort,
  disabled,
  onSelectModel,
  onSelectEffort,
  engineOptions,
  selectedEngine,
  engineLocked,
  running,
  lockedDescription,
  onSelectEngine
}: {
  engine?: EngineInfo;
  selectedModel: string;
  selectedEffort: string;
  disabled: boolean;
  onSelectModel: (model: string, effort: string) => void;
  onSelectEffort: (effort: string) => void;
  engineOptions: EngineOption[];
  selectedEngine: string;
  engineLocked: boolean;
  running: boolean;
  lockedDescription?: string;
  onSelectEngine: (id: string) => void;
}): JSX.Element {
  const { t } = useI18n();
  const [view, setView] = useState<RuntimePanelView>("summary");
  const [direction, setDirection] = useState<RuntimePanelDirection>("forward");
  const [query, setQuery] = useState("");
  const [optimistic, setOptimistic] = useState<{ model: string; effort: string } | null>(null);
  useEffect(() => {
    setOptimistic(null);
  }, [selectedModel, selectedEffort]);
  useEffect(() => {
    setQuery("");
    setDirection("back");
    setView("summary");
  }, [engine?.id]);

  const openView = (nextView: RuntimePanelView): void => {
    setDirection("forward");
    setView(nextView);
  };
  const showSummary = (): void => {
    setDirection("back");
    setView("summary");
  };

  const models = engine?.models ?? [];
  const normalizedQuery = query.trim().toLocaleLowerCase();
  const filteredModels = normalizedQuery
    ? models.filter((model) =>
        (model.display_name || model.id).toLocaleLowerCase().includes(normalizedQuery)
        || model.id.toLocaleLowerCase().includes(normalizedQuery))
    : models;
  const effectiveModelID = optimistic?.model ?? selectedModel;
  const effectiveModel = models.find((model) => model.id === effectiveModelID);
  const effortOptions = effectiveModel?.supported_efforts ?? [];
  const effectiveEffort = optimistic?.effort
    ?? (effortOptions.includes(selectedEffort)
      ? selectedEffort
      : effectiveModel
        ? engineModelDefaultEffort(effectiveModel)
        : selectedEffort);
  const chooserHeight = Math.min(320, 50 + engineOptions.length * 36);

  return (
    <div
      className={`codex-runtime-menu codex-model-menu runtime-panel is-${view}`}
      role="menu"
      style={{ "--runtime-chooser-height": `${chooserHeight}px` } as CSSProperties}
    >
      <div key={`${engine?.id ?? "engine"}:${view}`} className={`runtime-panel-page is-${direction}`}>
        {view === "summary" ? (
          <RuntimePanelSummary
            engine={selectedEngine}
            engineLocked={engineLocked}
            model={effectiveModel?.display_name || effectiveModelID || t("runtime.selectModel")}
            effortOptions={effortOptions}
            selectedEffort={effectiveEffort}
            effortDisabled={disabled}
            onOpenEngines={() => openView("engines")}
            onOpenModels={() => openView("models")}
            onSelectEffort={(effort) => {
              setOptimistic((current) => ({ model: current?.model ?? selectedModel, effort }));
              onSelectEffort(effort);
            }}
          />
        ) : null}
        {view === "engines" ? (
          <>
            <RuntimePanelHeader title={t("runtime.engine")} onBack={showSummary} />
            <div className="runtime-panel-list">
              <EngineOptionsMenu
                options={engineOptions}
                selected={selectedEngine}
                locked={engineLocked}
                running={running}
                lockedDescription={lockedDescription}
                onSelect={(id) => {
                  onSelectEngine(id);
                  showSummary();
                }}
              />
            </div>
          </>
        ) : null}
        {view === "models" ? (
          <>
            <RuntimePanelHeader title={t("runtime.model")} onBack={showSummary} />
            <label className="select-menu-search">
              <Search className="select-menu-search-icon icon-lg" />
              <input
                type="search"
                value={query}
                placeholder={t("runtime.searchModels")}
                aria-label={t("runtime.searchModels")}
                onChange={(event) => setQuery(event.currentTarget.value)}
              />
            </label>
            <div className="codex-model-groups">
              {engine?.models_error ? (
                <div className="composer-menu-note warning">
                  <strong>{t("runtime.modelsLoadFailed")}</strong>
                  <span>{engine.models_error}</span>
                </div>
              ) : null}
              {models.length === 0 ? <div className="composer-menu-empty">{t("runtime.noModels")}</div> : null}
              {models.length > 0 && filteredModels.length === 0 ? (
                <div className="composer-menu-empty">{t("runtime.noMatchingModels")}</div>
              ) : null}
              {filteredModels.length > 0 ? (
                <div className="codex-model-group">
                  {filteredModels.map((model) => {
                    const selected = model.id === effectiveModelID;
                    return (
                      <button
                        className="codex-model-item"
                        role="menuitemradio"
                        type="button"
                        key={model.id}
                        disabled={disabled}
                        aria-checked={selected}
                        onClick={() => {
                          const effort = engineModelDefaultEffort(model);
                          setOptimistic({ model: model.id, effort });
                          onSelectModel(model.id, effort);
                          showSummary();
                        }}
                      >
                        <span className="codex-model-item-name">{model.display_name || model.id}</span>
                        {selected ? <Check className="icon-lg" /> : null}
                      </button>
                    );
                  })}
                </div>
              ) : null}
            </div>
          </>
        ) : null}
      </div>
    </div>
  );
}

function RuntimeModelMenu({
  initialized,
  state,
  selectedProvider,
  selectedModel,
  selectedVariant,
  onSelectModel,
  onSelectEffort,
  engineOptions,
  selectedEngine,
  engineLocked,
  running,
  lockedDescription,
  onSelectEngine
}: {
  initialized: InitializeResult;
  state: CodexModelLoadState;
  selectedProvider: string;
  selectedModel: string;
  selectedVariant: string;
  onSelectModel: (provider: string, model: string, variant?: string) => void | Promise<boolean>;
  onSelectEffort: (variant: string) => void | Promise<boolean>;
  engineOptions: EngineOption[];
  selectedEngine: string;
  engineLocked: boolean;
  running: boolean;
  lockedDescription?: string;
  onSelectEngine: (id: string) => void;
}): JSX.Element {
  const { t } = useI18n();
  const [view, setView] = useState<RuntimePanelView>("summary");
  const [direction, setDirection] = useState<RuntimePanelDirection>("forward");
  const [query, setQuery] = useState("");
  // The panel stays open while a selection commits through the app-server
  // stream, so the highlighted row and the effort pills follow the click
  // immediately instead of waiting for the round-trip. Once the stream
  // confirms, the external props become the source of truth again and the
  // optimistic state is dropped.
  const [optimistic, setOptimistic] = useState<{ provider: string; model: string; variant: string } | null>(null);
  useEffect(() => {
    setOptimistic(null);
  }, [selectedProvider, selectedModel, selectedVariant]);

  const openView = (nextView: RuntimePanelView): void => {
    setDirection("forward");
    setView(nextView);
  };
  const showSummary = (): void => {
    setDirection("back");
    setView("summary");
  };

  const providers = initialized.providers ?? [];
  const configuredModels = providers
    .map((provider) => {
      const model = configuredRuntimeModelForProvider(provider, state);
      return model ? { provider, model } : undefined;
    })
    .filter((item): item is RuntimeModelOption => Boolean(item));
  const additionalModels = providers.flatMap((provider) =>
    runtimeModelsForProvider(provider, state)
      .filter((model) => model.id !== provider.model)
      .map((model) => ({ provider, model }))
  );

  // Group by provider with the configured model first, preserving provider
  // order. The group header is the provider name — the classification users
  // actually think in ("Claude is Anthropic's"), unlike a flat configured/
  // additional split.
  const groups: RuntimeModelGroup[] = [];
  const groupByProvider = new Map<string, RuntimeModelGroup>();
  for (const item of configuredModels) {
    const group = { provider: item.provider, models: [item.model] };
    groups.push(group);
    groupByProvider.set(item.provider.name, group);
  }
  for (const item of additionalModels) {
    const group = groupByProvider.get(item.provider.name);
    if (group) {
      group.models.push(item.model);
    } else {
      const created = { provider: item.provider, models: [item.model] };
      groups.push(created);
      groupByProvider.set(item.provider.name, created);
    }
  }

  const effectiveProviderName = optimistic?.provider ?? selectedProvider;
  const effectiveModelID = optimistic?.model ?? selectedModel;
  const effectiveVariant = optimistic?.variant ?? selectedVariant;
  const effectiveProvider = providers.find((provider) => provider.name === effectiveProviderName);
  const scopedGroups = groups.filter((group) => group.provider.name === effectiveProviderName);

  const normalizedQuery = query.trim().toLocaleLowerCase();
  const filteredGroups = normalizedQuery
    ? scopedGroups
        .map((group) => ({
          ...group,
          models: group.models.filter(
            (model) =>
              providerModelDisplayName(model).toLocaleLowerCase().includes(normalizedQuery) ||
              model.id.toLocaleLowerCase().includes(normalizedQuery) ||
              group.provider.name.toLocaleLowerCase().includes(normalizedQuery)
          )
        }))
        .filter((group) => group.models.length > 0)
    : scopedGroups;

  const effectiveCodex = providerIsCodex(initialized, effectiveProviderName);
  const effectiveCodexModel = effectiveCodex
    ? state.models.find((model) => model.slug === effectiveModelID)
    : undefined;
  const effortOptions = effectiveCodex
    ? codexEffortOptions(effectiveCodexModel, effectiveVariant)
    : providerModelVariantOptions(effectiveProvider, effectiveModelID, effectiveVariant);
  const effectiveModel = scopedGroups
    .flatMap((group) => group.models)
    .find((model) => model.id === effectiveModelID);
  const chooserRows = view === "providers" ? groups.length : engineOptions.length;
  const chooserHeight = Math.min(320, 50 + chooserRows * 36);

  const selectModel = (provider: string, model: string, variant?: string): void => {
    setOptimistic({ provider, model, variant: variant ?? "" });
    void Promise.resolve(onSelectModel(provider, model, variant)).then((committed) => {
      if (committed === false) setOptimistic(null);
    });
  };

  return (
    <div
      className={`codex-runtime-menu codex-model-menu runtime-panel is-${view}`}
      role="menu"
      style={{ "--runtime-chooser-height": `${chooserHeight}px` } as CSSProperties}
    >
      <div key={view} className={`runtime-panel-page is-${direction}`}>
        {view === "summary" ? (
          <RuntimePanelSummary
            engine={selectedEngine}
            provider={effectiveProviderName}
            engineLocked={engineLocked}
            model={effectiveModel ? providerModelDisplayName(effectiveModel) : effectiveModelID || t("runtime.selectModel")}
            effortOptions={effortOptions}
            selectedEffort={effectiveVariant}
            effortDisabled={false}
            onOpenEngines={() => openView("engines")}
            onOpenProviders={() => openView("providers")}
            onOpenModels={() => openView("models")}
            onSelectEffort={(variant) => {
              setOptimistic((current) =>
                current ? { ...current, variant } : { provider: selectedProvider, model: selectedModel, variant }
              );
              void Promise.resolve(onSelectEffort(variant)).then((committed) => {
                if (committed === false) setOptimistic(null);
              });
            }}
          />
        ) : null}
        {view === "engines" ? (
          <>
            <RuntimePanelHeader title={t("runtime.engine")} onBack={showSummary} />
            <div className="runtime-panel-list">
              <EngineOptionsMenu
                options={engineOptions}
                selected={selectedEngine}
                locked={engineLocked}
                running={running}
                lockedDescription={lockedDescription}
                onSelect={(id) => {
                  onSelectEngine(id);
                  showSummary();
                }}
              />
            </div>
          </>
        ) : null}
        {view === "providers" ? (
          <>
            <RuntimePanelHeader title={t("runtime.provider")} onBack={showSummary} />
            <div className="runtime-panel-list runtime-provider-options" role="group" aria-label={t("runtime.provider")}>
              {groups.map((group) => {
                const selected = group.provider.name === effectiveProviderName;
                return (
                  <button
                    type="button"
                    className="runtime-provider-option"
                    role="menuitemradio"
                    aria-checked={selected}
                    key={group.provider.name}
                    onClick={() => {
                      const model = group.models[0];
                      if (model && !selected) {
                        selectModel(group.provider.name, model.id, defaultVariantForRuntimeModel(group.provider, model));
                      }
                      showSummary();
                    }}
                  >
                    <span>{group.provider.name}</span>
                    {selected ? <Check aria-hidden="true" /> : null}
                  </button>
                );
              })}
            </div>
          </>
        ) : null}
        {view === "models" ? (
          <>
            <RuntimePanelHeader title={t("runtime.model")} onBack={showSummary} />
            <label className="select-menu-search">
              <Search className="select-menu-search-icon icon-lg" />
              <input
                type="search"
                value={query}
                placeholder={t("runtime.searchModels")}
                aria-label={t("runtime.searchModels")}
                onChange={(event) => setQuery(event.currentTarget.value)}
              />
            </label>
            <div className="codex-model-groups">
              {effectiveCodex && state.loading ? <div className="composer-menu-empty">{t("runtime.loadingCodexModels")}</div> : null}
              {effectiveCodex && state.error ? (
                <div className="composer-menu-note warning">
                  <strong>{t("runtime.codexLoginUnavailable")}</strong>
                  <span>{state.error}</span>
                </div>
              ) : null}
              {!state.loading && providers.length === 0 ? <div className="composer-menu-empty">{t("runtime.noModels")}</div> : null}
              {filteredGroups.length === 0 ? <div className="composer-menu-empty">{t("runtime.noMatchingModels")}</div> : null}
              {filteredGroups.map((group) => (
                <div className="codex-model-group" key={group.provider.name}>
                  {group.models.map((model) => (
                    <RuntimeModelMenuItem
                      key={`${group.provider.name}/${model.id}`}
                      provider={group.provider}
                      model={model}
                      selected={group.provider.name === effectiveProviderName && model.id === effectiveModelID}
                      selectedVariant={effectiveVariant}
                      onSelectModel={(provider, model, variant) => {
                        selectModel(provider, model, variant);
                        showSummary();
                      }}
                    />
                  ))}
                </div>
              ))}
            </div>
          </>
        ) : null}
      </div>
    </div>
  );
}

function EffortSelector({
  options,
  selectedVariant,
  disabled = false,
  onPreviewEffort,
  onSelectEffort
}: {
  options: string[];
  selectedVariant: string;
  disabled?: boolean;
  onPreviewEffort?: (variant: string) => void;
  onSelectEffort: (variant: string) => void;
}): JSX.Element {
  const selectedIndex = Math.max(0, options.indexOf(selectedVariant));
  const [previewIndex, setPreviewIndex] = useState(selectedIndex);

  useEffect(() => {
    setPreviewIndex(selectedIndex);
  }, [selectedIndex]);

  const commit = (index: number): void => {
    if (disabled) return;
    const next = options[index];
    if (next !== undefined && index !== selectedIndex) onSelectEffort(next);
  };
  const preview = (index: number): void => {
    setPreviewIndex(index);
    const next = options[index];
    if (next !== undefined) onPreviewEffort?.(next);
  };
  const resetPreview = (): void => {
    setPreviewIndex(selectedIndex);
    onPreviewEffort?.(options[selectedIndex] ?? selectedVariant);
  };
  const progress = options.length > 1 ? previewIndex / (options.length - 1) : 0;

  return (
    <div
      className={`codex-effort-slider${disabled ? " is-disabled" : ""}`}
      style={{ "--effort-progress": `${progress * 100}%` } as CSSProperties}
    >
      <span className="codex-effort-track" aria-hidden="true" />
      <span className="codex-effort-fill" aria-hidden="true" />
      <span className="codex-effort-stops" aria-hidden="true">
        {options.map((variant, index) => (
          <span className={index <= previewIndex ? "is-filled" : ""} key={variant || `default-${index}`} />
        ))}
      </span>
      <input
        type="range"
        min={0}
        max={options.length - 1}
        step={1}
        value={previewIndex}
        disabled={disabled}
        aria-label={translate("runtime.reasoningEffort")}
        aria-valuetext={variantLabel(options[previewIndex] ?? selectedVariant)}
        onChange={(event) => preview(Number(event.currentTarget.value))}
        onPointerUp={(event) => commit(Number(event.currentTarget.value))}
        onPointerCancel={resetPreview}
        onKeyUp={(event) => commit(Number(event.currentTarget.value))}
        onBlur={resetPreview}
      />
    </div>
  );
}

type RuntimeModelOption = {
  provider: ProviderSummary;
  model: ProviderModelSummary;
};

type RuntimeModelGroup = {
  provider: ProviderSummary;
  models: ProviderModelSummary[];
};

function RuntimeModelMenuItem({
  provider,
  model,
  selected,
  selectedVariant,
  onSelectModel
}: {
  provider: ProviderSummary;
  model: ProviderModelSummary;
  selected: boolean;
  selectedVariant: string;
  onSelectModel: (provider: string, model: string, variant?: string) => void;
}): JSX.Element {
  const nextVariant = selected
    ? selectedVariant
    : defaultVariantForRuntimeModel(provider, model);
  return (
    <button
      className="codex-model-item"
      role="menuitemradio"
      type="button"
      aria-checked={selected}
      onClick={() => onSelectModel(provider.name, model.id, nextVariant)}
    >
      <span className="codex-model-item-name">{providerModelDisplayName(model)}</span>
      {selected ? <Check className="icon-lg" /> : null}
    </button>
  );
}

function runtimeTriggerLabel(
  initialized: InitializeResult,
  providerModel?: ProviderModelSummary,
  codexModel?: CodexModelSummary
): string {
  if (codexModel) {
    return shortCodexModelLabel(codexModel.slug);
  }
  return shortCodexModelLabel(providerModel?.display_name || initialized.model);
}

function configuredRuntimeModelForProvider(
  provider: ProviderSummary,
  state: CodexModelLoadState
): ProviderModelSummary | undefined {
  if (!provider.model) {
    return undefined;
  }
  return (
    runtimeModelsForProvider(provider, state).find((model) => model.id === provider.model) ??
    provider.models?.find((model) => model.id === provider.model) ?? { id: provider.model, source: "selected" }
  );
}

function runtimeModelsForProvider(provider: ProviderSummary, state: CodexModelLoadState): ProviderModelSummary[] {
  const type = provider.type.trim().toLowerCase().replaceAll("_", "-");
  const isCodex = type === "openai-codex" || type === "codex-subscription" || type === "chatgpt-codex";
  if (isCodex && state.provider === provider.name && state.models.length > 0) {
    return state.models.map((model) => ({
      id: model.slug,
      display_name: displayCodexModelName(model),
      default_effort: model.default_reasoning_level,
      supported_efforts: model.supported_reasoning,
      source: "live"
    }));
  }
  if (provider.models?.length) {
    return provider.models;
  }
  return [{ id: provider.model, source: "selected" }];
}

function defaultVariantForRuntimeModel(
  provider: ProviderSummary,
  model: ProviderModelSummary
): string {
  const modelVariants = (model.variants ?? []).map((item) => item.id).filter(Boolean);
  const supported = modelVariants.length > 0 ? modelVariants : model.supported_efforts ?? [];
  if (supported.length === 0) {
    return "";
  }
  if (model.default_variant && supported.includes(model.default_variant)) {
    return model.default_variant;
  }
  if (model.default_effort && supported.includes(model.default_effort)) {
    return model.default_effort;
  }
  const providerModel = provider.models?.find((item) => item.id === model.id);
  if (providerModel?.default_variant && supported.includes(providerModel.default_variant)) {
    return providerModel.default_variant;
  }
  if (providerModel?.default_effort && supported.includes(providerModel.default_effort)) {
    return providerModel.default_effort;
  }
  return "";
}

// Composer-bar "+" menu: the single entry point for everything the composer
// can attach or invoke — attachments plus the full slash-command list
// (built-in actions, prompts, and skills; future plugin actions join here).
// Clicking a command behaves exactly like picking it in the "/" panel.
// Open state is local, so the host's floating-menu registry needs no wiring.
export function ComposerPlusButton({
  variant,
  disabled,
  commands,
  menuAnchorRef,
  onAddAttachment,
  onSelectCommand
}: {
  variant: ComposerVariant;
  disabled: boolean;
  commands: ComposerSlashCommand[];
  menuAnchorRef: RefObject<HTMLElement | null>;
  onAddAttachment: () => void;
  onSelectCommand: (command: ComposerSlashCommand) => void;
}): JSX.Element {
  const { t } = useI18n();
  const triggerRef = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);

  useEffect(() => {
    if (!open) {
      return undefined;
    }
    function handlePointerDown(event: PointerEvent): void {
      const target = event.target;
      if (!(target instanceof Node)) {
        return;
      }
      if (triggerRef.current?.contains(target)) {
        return;
      }
      if (isInsideFloatingMenu(target, "composer-plus")) {
        return;
      }
      setOpen(false);
    }
    function handleKeyDown(event: KeyboardEvent): void {
      if (event.key === "Escape") {
        setOpen(false);
      }
    }
    document.addEventListener("pointerdown", handlePointerDown);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("pointerdown", handlePointerDown);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [open]);

  return (
    <div className="composer-plus-menu-anchor" ref={triggerRef}>
      <button
        className="composer-tool-button composer-plus-button"
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label={t("composer.plusMenu")}
        title={t("composer.plusMenu")}
        disabled={disabled}
        onClick={() => setOpen((current) => !current)}
      >
        <Plus aria-hidden="true" />
      </button>
      {open ? (
        <FloatingMenuPortal
          anchorRef={menuAnchorRef}
          owner="composer-plus"
          placement="above"
          align="left"
          offset={variant === "hero" ? 10 : 8}
          width={320}
          matchAnchorWidth
        >
          <div className="composer-context-menu composer-plus-menu" role="menu" aria-label={t("composer.plusMenu")}>
            <div className="composer-plus-menu-section" role="presentation">{t("composer.plusSectionAdd")}</div>
            <button
              role="menuitem"
              type="button"
              onClick={() => {
                setOpen(false);
                onAddAttachment();
              }}
            >
              <Paperclip className="icon-lg" />
              <span className="composer-plus-menu-item-title">{t("composer.addAttachment")}</span>
              <span className="composer-plus-menu-item-desc">{t("composer.addAttachmentHint")}</span>
            </button>
            <div className="composer-plus-menu-section" role="presentation">{t("composer.plusSectionCommands")}</div>
            {commands.map((command) => (
              <Tooltip content={command.disabledReason} key={command.id}>
                <button
                  role="menuitem"
                  type="button"
                  disabled={Boolean(command.disabledReason)}
                  onClick={() => {
                    setOpen(false);
                    onSelectCommand(command);
                  }}
                >
                  <SlashCommandIcon command={command} />
                  <span className="composer-plus-menu-item-title">
                    {command.kind === "skill" ? command.description : command.title}
                  </span>
                  <span className="composer-plus-menu-item-desc">
                    {command.kind === "skill" ? command.title : command.description}
                  </span>
                </button>
              </Tooltip>
            ))}
          </div>
        </FloatingMenuPortal>
      ) : null}
    </div>
  );
}

// Icon resolver shared by the "/" command panel and the "+" menu so both
// surfaces show the same glyph for the same command.
export function SlashCommandIcon({ command }: { command: ComposerSlashCommand }): JSX.Element {
  switch (command.action ?? command.id) {
    case "review":
      return <Search className="icon" />;
    case "open-review":
      return <GitCompare className="icon" />;
    case "debug":
      return <Bug className="icon" />;
    case "fix":
      return <Hammer className="icon" />;
    case "test":
      return <FlaskConical className="icon" />;
    case "explain":
      return <CircleHelp className="icon" />;
    case "commit":
      return <GitCommitHorizontal className="icon" />;
    case "pr":
      return <GitPullRequest className="icon" />;
    case "open-skills":
      return <Puzzle className="icon" />;
    case "new-thread":
      return <MessageSquarePlus className="icon" />;
    case "open-terminal":
      return <Terminal className="icon" />;
    case "open-files":
      return <FileText className="icon" />;
    case "open-project":
      return <FolderOpen className="icon" />;
    case "no-project":
      return <FolderX className="icon" />;
    case "reset-side-thread":
      return <RotateCcw className="icon" />;
    case "context":
      return <PieChart className="icon" />;
    case "compact":
      return <FoldVertical className="icon" />;
    case "instructions":
      return <ScrollText className="icon" />;
    case "fast":
      return <Zap className="icon" />;
    case "model":
      return <Cpu className="icon" />;
    case "effort":
      return <Gauge className="icon" />;
    case "settings":
      return <Settings className="icon" />;
    default:
      return <Puzzle className="icon" />;
  }
}

export function AccessMenu({
  permissions,
  engine,
  disabled,
  onSelect
}: {
  permissions?: PermissionSummary;
  engine?: string;
  disabled: boolean;
  onSelect: (mode: PermissionMode) => void;
}): JSX.Element {
  useI18n();
  const mode = permissionModeFromSummary(permissions);
  const options = permissionModeOptions(engine);
  return (
    <div className="composer-context-menu access-menu" role="menu">
      {options.map((option) => (
        <button
          key={option.mode}
          className={`permission-mode-option ${option.tone}`}
          role="menuitemradio"
          aria-checked={mode === option.mode}
          aria-label={option.label}
          type="button"
          disabled={disabled}
          onClick={() => onSelect(option.mode)}
        >
          <strong>{option.label}</strong>
        </button>
      ))}
    </div>
  );
}



export function BranchMenu({
  gitStatus,
  onSelectBranch
}: {
  gitStatus: GitStatusResult;
  onSelectBranch: (branch: string) => void;
}): JSX.Element {
  const { t } = useI18n();
  const branches = gitStatus.branches ?? [];
  return (
    <div className="composer-context-menu branch-menu" role="menu">
      {gitStatus.dirty_count > 0 ? (
        <div className="composer-menu-note warning">
          <strong>{t("runtime.uncommittedChanges")}</strong>
          <span>{t(gitStatus.dirty_count === 1 ? "runtime.dirtyFileWarningOne" : "runtime.dirtyFileWarning", { count: gitStatus.dirty_count })}</span>
        </div>
      ) : null}
      {branches.length === 0 ? <div className="composer-menu-empty">{t("runtime.noLocalBranches")}</div> : null}
      {branches.map((branch) => {
        const selected = branch === gitStatus.branch;
        return (
          <button
            key={branch}
            role="menuitem"
            type="button"
            disabled={selected}
            onClick={() => onSelectBranch(branch)}
          >
            <GitBranch className="icon-lg" />
            <span>{branch}</span>
            {selected ? <Check className="icon" /> : null}
          </button>
        );
      })}
    </div>
  );
}

export function ProjectPickerMenu({
  projects,
  activeContext,
  query,
  setQuery,
  onSelectProject,
  onSelectNoProject,
  onCreateProject,
  onOpenProject
}: {
  projects: DesktopProject[];
  activeContext?: RuntimeContext;
  query: string;
  setQuery: (value: string) => void;
  onSelectProject: (id: string) => void;
  onSelectNoProject: () => void;
  onCreateProject: () => void;
  onOpenProject: () => void;
}): JSX.Element {
  const { t } = useI18n();
  const normalizedQuery = query.trim().toLocaleLowerCase();
  const filteredProjects = normalizedQuery
    ? projects.filter((project) => project.name.toLocaleLowerCase().includes(normalizedQuery) || project.path.toLocaleLowerCase().includes(normalizedQuery))
    : projects;

  return (
    <div className="composer-project-menu" role="menu">
      <label className="project-search">
        <Search className="icon-lg" />
        <input value={query} placeholder={t("runtime.searchProjects")} onChange={(event) => setQuery(event.target.value)} />
      </label>
      <div className="project-picker-list">
        {filteredProjects.length === 0 ? <div className="project-picker-empty">{t("runtime.noMatchingProjects")}</div> : null}
        {filteredProjects.map((project) => {
          const selected = activeContext?.kind === "project" && activeContext.project_id === project.id;
          return (
            <button key={project.id} type="button" role="menuitem" onClick={() => onSelectProject(project.id)}>
              <Folder className="icon-lg" />
              <span>{project.name}</span>
              {selected ? <Check className="icon-lg" /> : null}
            </button>
          );
        })}
      </div>
      <div className="project-picker-divider" />
      <button type="button" role="menuitem" onClick={onOpenProject}>
        <FolderOpen className="icon-lg" />
        <span>{t("runtime.useExistingFolder")}</span>
      </button>
      <button type="button" role="menuitem" onClick={onCreateProject}>
        <FolderPlus className="icon-lg" />
        <span>{t("runtime.createBlankProject")}</span>
      </button>
      <button type="button" role="menuitem" onClick={onSelectNoProject}>
        <FolderX className="icon-lg" />
        <span>{t("runtime.noProject")}</span>
        {activeContext?.kind === "no_project" ? <Check className="icon-lg" /> : null}
      </button>
    </div>
  );
}
