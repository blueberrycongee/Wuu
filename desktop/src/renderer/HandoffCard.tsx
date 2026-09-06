import type { InitializeResult } from "../shared/protocol";
import { RuntimeModelMenu } from "./ComposerRuntimeMenus";
import type { CodexModelLoadState } from "./ComposerTypes";
import { useI18n } from "./i18n";
import { canSubmitHandoffDraft, type HandoffDraft } from "./HandoffDraft";

export function HandoffCard({
  draft,
  initialized,
  modelState,
  sourceLabel,
  workspaceLabel,
  submitting,
  onSelectProvider,
  onSelectModel,
  onSelectEffort,
  onSubmit,
}: {
  draft: HandoffDraft;
  initialized: InitializeResult;
  modelState: CodexModelLoadState;
  sourceLabel: string;
  workspaceLabel: string;
  submitting?: boolean;
  onSelectProvider: (providerId: string) => void;
  onSelectModel: (providerId: string, modelId: string, variant?: string) => void;
  onSelectEffort: (variant: string) => void;
  onSubmit: () => void;
}): JSX.Element {
  const { t } = useI18n();
  const statusLabel = {
    pending: t("handoff.card.pending"),
    resolved: t("handoff.card.resolved"),
    unverified: t("handoff.card.unverified"),
    unavailable: t("handoff.card.unavailable"),
  }[draft.status];
  const selectedProvider = draft.providerId || initialized.provider;
  const selectedModel = draft.modelId || (draft.providerId ? "" : initialized.model);
  const selectedVariant = draft.variant || (draft.providerId ? "" : initialized.variant ?? initialized.effort ?? "");
  return (
    <section className="handoff-card" aria-label={t("handoff.card.title")}>
      <header className="handoff-card-title">{t("handoff.card.title")}</header>
      <dl className="handoff-card-grid">
        <div>
          <dt>{t("handoff.card.source")}</dt>
          <dd>{sourceLabel} · {t("handoff.card.cutoffPending")}</dd>
        </div>
        <div>
          <dt>{t("handoff.card.workspace")}</dt>
          <dd>{workspaceLabel || t("handoff.card.sharedWorkspace")}</dd>
        </div>
        <div>
          <dt>{t("handoff.card.context")}</dt>
          <dd>{t("handoff.card.contextHint")}</dd>
        </div>
        {draft.intent ? (
          <div>
            <dt>{t("handoff.card.intent")}</dt>
            <dd>{draft.intent}</dd>
          </div>
        ) : null}
      </dl>
      <div className="handoff-card-picker">
        <RuntimeModelMenu
          initialized={initialized}
          state={modelState}
          selectedProvider={selectedProvider}
          selectedModel={selectedModel}
          selectedVariant={selectedVariant}
          onSelectModel={onSelectModel}
          onSelectEffort={onSelectEffort}
          engineOptions={[{ id: "wuu", label: "Wuu" }]}
          selectedEngine="wuu"
          engineLocked
          running={false}
          onSelectEngine={() => undefined}
          filterQuery={draft.filterQuery}
          forcedView={draft.pickerView}
          hideHandoff
          embedded
          onSelectProvider={onSelectProvider}
        />
      </div>
      <div className="handoff-card-footer">
        <span className={`handoff-card-status is-${draft.status}`}>{draft.targetLabel || statusLabel}</span>
        <button
          type="button"
          className="handoff-card-submit"
          disabled={!canSubmitHandoffDraft(draft) || submitting}
          onClick={onSubmit}
        >
          {t("handoff.card.submit")}
        </button>
      </div>
    </section>
  );
}
