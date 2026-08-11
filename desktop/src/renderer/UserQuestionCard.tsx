import { useMemo, useState } from "react";
import type {
  UserQuestionAnswer,
  UserQuestionRequest,
} from "../shared/protocol";

type Props = {
  request: UserQuestionRequest;
  onAnswer: (answer: UserQuestionAnswer) => Promise<void>;
  onCancel: () => Promise<void>;
};

export function UserQuestionCard({ request, onAnswer, onCancel }: Props): JSX.Element {
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
      setError(cause instanceof Error ? cause.message : "Could not send your answer.");
      setSubmitting(false);
    }
  }

  return (
    <section className="user-question-card" aria-label="Question from Wuu">
      {request.questions.map((question) => (
        <fieldset className="user-question-field" key={question.id}>
          <legend>{question.header || question.question}</legend>
          {question.header ? <p className="user-question-prompt">{question.question}</p> : null}
          {question.detail ? <p className="user-question-detail">{question.detail}</p> : null}
          {question.options?.length ? (
            <div className="user-question-options">
              {question.options.map((option) => {
                const active = selected[question.id]?.includes(option.label) ?? false;
                return (
                  <button
                    aria-pressed={active}
                    className="user-question-option"
                    data-active={active || undefined}
                    key={option.label}
                    onClick={() => toggle(question.id, option.label, Boolean(question.multi_select))}
                    type="button"
                  >
                    <span>{option.label}</span>
                    {option.description ? <small>{option.description}</small> : null}
                  </button>
                );
              })}
            </div>
          ) : null}
          {question.allow_custom ? (
            <input
              aria-label={`Custom answer for ${question.question}`}
              className="user-question-custom"
              onChange={(event) => {
                const value = event.currentTarget.value;
                setCustom((current) => ({ ...current, [question.id]: value }));
              }}
              placeholder="Type another answer"
              value={custom[question.id] ?? ""}
            />
          ) : null}
        </fieldset>
      ))}
      <div className="user-question-actions">
        {error ? <span className="user-question-error" role="alert">{error}</span> : null}
        <button
          className="user-question-cancel"
          disabled={submitting}
          onClick={() => {
            setSubmitting(true);
            setError("");
            void onCancel().catch((cause) => {
              setError(cause instanceof Error ? cause.message : "Could not cancel the question.");
              setSubmitting(false);
            });
          }}
          type="button"
        >
          Cancel
        </button>
        <button disabled={!complete || submitting} onClick={() => void submit()} type="button">
          {submitting ? "Sending…" : "Continue"}
        </button>
      </div>
    </section>
  );
}
