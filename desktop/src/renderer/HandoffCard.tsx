import { useI18n } from "./i18n";
import { canSubmitHandoffDraft, type HandoffDraft } from "./HandoffDraft";

export function HandoffCard({
  draft,
  sourceLabel,
  workspaceLabel,
  submitting,
  onSelectCandidate,
  onSubmit,
}: {
  draft: HandoffDraft;
  sourceLabel: string;
  workspaceLabel: string;
  submitting?: boolean;
  onSelectCandidate: (providerId: string, modelId: string) => void;
  onSubmit: () => void;
}): JSX.Element {
  const { t } = useI18n();
  const statusLabel = {
    pending: t("handoff.card.pending"),
    resolved: t("handoff.card.resolved"),
    unverified: t("handoff.card.unverified"),
    unavailable: t("handoff.card.unavailable"),
  }[draft.status];
  return (
    <section className="handoff-card" aria-label={t("handoff.card.title")}>
      <header className="handoff-card-title">{t("handoff.card.title")}</header>
      <dl className="handoff-card-grid">
        <div>
          <dt>{t("handoff.card.target")}</dt>
          <dd>
            {draft.targetLabel || statusLabel}
            <span className={`handoff-card-status is-${draft.status}`}>{statusLabel}</span>
          </dd>
        </div>
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
      {draft.candidates.length > 0 && draft.status === "pending" ? (
        <div className="handoff-card-candidates" role="listbox" aria-label={t("handoff.card.target")}>
          {draft.candidates.slice(0, 8).map((candidate) => (
            <button
              key={`${candidate.providerId}/${candidate.modelId}`}
              type="button"
              role="option"
              className="handoff-card-candidate"
              onClick={() => onSelectCandidate(candidate.providerId, candidate.modelId)}
            >
              {candidate.providerLabel} / {candidate.modelLabel}
            </button>
          ))}
        </div>
      ) : null}
      <button
        type="button"
        className="handoff-card-submit"
        disabled={!canSubmitHandoffDraft(draft) || submitting}
        onClick={onSubmit}
      >
        {t("handoff.card.submit")}
      </button>
    </section>
  );
}
