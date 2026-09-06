import type { InitializeResult } from "../shared/protocol";
import { RuntimeModelMenu } from "./ComposerRuntimeMenus";
import type { CodexModelLoadState } from "./ComposerTypes";
import { useI18n } from "./i18n";
import type { HandoffDraft } from "./HandoffDraft";

export function HandoffCard({
  draft,
  initialized,
  modelState,
  onSelectProvider,
  onSelectModel,
  onSelectEffort,
}: {
  draft: HandoffDraft;
  initialized: InitializeResult;
  modelState: CodexModelLoadState;
  onSelectProvider: (providerId: string) => void;
  onSelectModel: (providerId: string, modelId: string, variant?: string) => void;
  onSelectEffort: (variant: string) => void;
}): JSX.Element {
  const { t } = useI18n();
  const selectedProvider = draft.providerId || initialized.provider;
  const selectedModel = draft.modelId || (draft.providerId ? "" : initialized.model);
  const selectedVariant = draft.variant || (draft.providerId ? "" : initialized.variant ?? initialized.effort ?? "");
  return (
    <section className="handoff-card" aria-label={t("handoff.card.title")}>
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
    </section>
  );
}
