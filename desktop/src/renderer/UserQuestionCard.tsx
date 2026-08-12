import { Circle, CircleDot, Square, SquareCheck } from "lucide-react";
import { useMemo, useState } from "react";
import type {
  UserQuestionAnswer,
  UserQuestionRequest,
} from "../shared/protocol";
import { useI18n } from "./i18n";

type Props = {
  request: UserQuestionRequest;
  onAnswer: (answer: UserQuestionAnswer) => Promise<void>;
  onCancel: () => Promise<void>;
};

export function UserQuestionCard({ request, onAnswer, onCancel }: Props): JSX.Element {
  const { t } = useI18n();
  const [selected, setSelected] = useState<Record<string, string[]>>({});
  const [custom, setCustom] = useState<Record<string, string>>({});
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const complete = useMemo(
    () => request.questions.every((question) =>
      (selected[question.id]?.length ?? 0) > 0 ||
      (question.allow_custom && (custom[question.id]?.trim().length ?? 0) > 0)),
    [custom, request.questions, selected],
  );

  function toggle(questionID: string, label: string, multiSelect: boolean): void {
    setSelected((current) => {
      const values = current[questionID] ?? [];
      const next = multiSelect
        ? values.includes(label)
          ? values.filter((value) => value !== label)
          : [...values, label]
        : [label];
      return { ...current, [questionID]: next };
    });
  }

  async function submit(): Promise<void> {
    if (!complete || submitting) return;
    setSubmitting(true);
    setError("");
    try {
      await onAnswer({
        answers: request.questions.map((question) => ({
          id: question.id,
          selected: selected[question.id] ?? [],
          ...(custom[question.id]?.trim()
            ? { custom: custom[question.id].trim() }
            : {}),
        })),
      });
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : t("userQuestion.sendFailed"),
      );
      setSubmitting(false);
    }
  }

  return (
    <section className="user-question-card" aria-label={t("userQuestion.kicker")}>
      <p className="user-question-kicker">{t("userQuestion.kicker")}</p>
      {request.questions.map((question) => {
        const lead = question.header || question.question;
        return (
          <div className="user-question-field" key={question.id}>
            <p className="user-question-prompt">{lead}</p>
            {question.header ? (
              <p className="user-question-body">{question.question}</p>
            ) : null}
            {question.detail ? (
              <p className="user-question-detail">{question.detail}</p>
            ) : null}
            {question.options?.length ? (
              <div
                className="user-question-options"
                role={question.multi_select ? "group" : "radiogroup"}
                aria-label={lead}
              >
                {question.options.map((option) => {
                  const active = selected[question.id]?.includes(option.label) ?? false;
                  return (
                    <button
                      aria-checked={active}
                      className="user-question-option"
                      data-active={active || undefined}
                      data-multi={question.multi_select ? "true" : "false"}
                      key={option.label}
                      onClick={() => toggle(question.id, option.label, Boolean(question.multi_select))}
                      role={question.multi_select ? "checkbox" : "radio"}
                      type="button"
                    >
                      <span className="user-question-option-indicator" aria-hidden="true">
                        {question.multi_select ? (
                          active ? <SquareCheck /> : <Square />
                        ) : active ? (
                          <CircleDot />
                        ) : (
                          <Circle />
                        )}
                      </span>
                      <span className="user-question-option-content">
                        <span className="user-question-option-label">{option.label}</span>
                        {option.description ? (
                          <span className="user-question-option-description">
                            {option.description}
                          </span>
                        ) : null}
                      </span>
                    </button>
                  );
                })}
              </div>
            ) : null}
            {question.allow_custom ? (
              <input
                aria-label={t("userQuestion.customAriaLabel", { question: lead })}
                className="user-question-custom"
                onChange={(event) => {
                  const value = event.currentTarget.value;
                  setCustom((current) => ({ ...current, [question.id]: value }));
                }}
                placeholder={t("userQuestion.customPlaceholder")}
                value={custom[question.id] ?? ""}
              />
            ) : null}
          </div>
        );
      })}
      <div className="user-question-actions">
        {error ? <span className="user-question-error" role="alert">{error}</span> : null}
        <button
          className="user-question-cancel"
          disabled={submitting}
          onClick={() => {
            setSubmitting(true);
            setError("");
            void onCancel().catch((cause) => {
              setError(
                cause instanceof Error ? cause.message : t("userQuestion.cancelFailed"),
              );
              setSubmitting(false);
            });
          }}
          type="button"
        >
          {t("userQuestion.cancel")}
        </button>
        <button
          className="user-question-submit"
          disabled={!complete || submitting}
          onClick={() => void submit()}
          type="button"
        >
          {submitting ? t("userQuestion.sending") : t("userQuestion.continue")}
        </button>
      </div>
    </section>
  );
}
