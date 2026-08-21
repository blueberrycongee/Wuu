import { Circle, CircleDot, Square, SquareCheck } from "lucide-react";
import { useMemo, useState } from "react";
import type {
  UserQuestionAnswer,
  UserQuestion,
  UserQuestionOption,
  UserQuestionRequest,
} from "../shared/protocol";
import { useI18n } from "./i18n";

const APPROVAL_ALLOW_ONCE = "Allow once";
const APPROVAL_ALLOW_SESSION = "Allow for this session";
const APPROVAL_DENY = "Deny";

function approvalKind(question: UserQuestion): "command" | "file" | "permissions" | undefined {
  if (question.id === "approval.command_execution") return "command";
  if (question.id === "approval.file_change") return "file";
  if (question.id === "approval.permissions") return "permissions";
  return undefined;
}

function approvalEngineName(pluginID: string): string | undefined {
  if (!pluginID.startsWith("agent-engine-")) return undefined;
  const engine = pluginID.slice("agent-engine-".length).trim();
  if (!engine) return undefined;
  return engine.charAt(0).toUpperCase() + engine.slice(1);
}

function approvalQuestionText(
  request: UserQuestionRequest,
  question: UserQuestion,
  t: ReturnType<typeof useI18n>["t"],
): { header: string; question: string } | undefined {
  const engine = approvalEngineName(request.plugin_id);
  const kind = approvalKind(question);
  if (!engine || !kind) return undefined;
  const values = { engine };
  const questionKeys = {
    command: "userQuestion.approvalCommandQuestion",
    file: "userQuestion.approvalFileQuestion",
    permissions: "userQuestion.approvalPermissionsQuestion",
  } as const;
  return {
    header: t("userQuestion.approvalHeader", values),
    question: t(questionKeys[kind], values),
  };
}

function approvalOptionText(
  request: UserQuestionRequest,
  question: UserQuestion,
  option: UserQuestionOption,
  t: ReturnType<typeof useI18n>["t"],
): UserQuestionOption {
  if (!approvalEngineName(request.plugin_id) || !approvalKind(question)) return option;
  const labels: Record<string, { label: Parameters<typeof t>[0]; description: Parameters<typeof t>[0] }> = {
    [APPROVAL_ALLOW_ONCE]: {
      label: "userQuestion.approvalAllowOnce",
      description: "userQuestion.approvalAllowOnceDescription",
    },
    [APPROVAL_ALLOW_SESSION]: {
      label: "userQuestion.approvalAllowSession",
      description: "userQuestion.approvalAllowSessionDescription",
    },
    [APPROVAL_DENY]: {
      label: "userQuestion.approvalDeny",
      description: "userQuestion.approvalDenyDescription",
    },
  };
  const keys = labels[option.label];
  if (!keys) return option;
  return { label: t(keys.label), description: t(keys.description) };
}

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
        const approvalText = approvalQuestionText(request, question, t);
        const header = approvalText?.header || question.header;
        const prompt = approvalText?.question || question.question;
        const lead = header || prompt;
        return (
          <div className="user-question-field" key={question.id}>
            <p className="user-question-prompt">{lead}</p>
            {header ? (
              <p className="user-question-body">{prompt}</p>
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
                  const displayOption = approvalOptionText(request, question, option, t);
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
                        <span className="user-question-option-label">{displayOption.label}</span>
                        {displayOption.description ? (
                          <span className="user-question-option-description">
                            {displayOption.description}
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
