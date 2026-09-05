import { Circle, CircleDot } from "lucide-react";
import { useState } from "react";
import { useI18n } from "./i18n";
import type { WuuMascotAccessory, WuuMascotActivity } from "./WuuMascot";

export const ONBOARDING_PLUGIN_MASCOT: Readonly<
  Record<string, { activity: WuuMascotActivity; accessory: WuuMascotAccessory }>
> = {
  "ask-user": { activity: "thinking", accessory: "headphones" },
  todo: { activity: "edit", accessory: "cap" },
  automation: { activity: "command", accessory: "wizard-hat" },
  subagent: { activity: "tool", accessory: "bow-tie" },
  memory: { activity: "read", accessory: "beanie" },
  dream: { activity: "compose", accessory: "halo" },
  "note-compaction": { activity: "compact", accessory: "graduation-cap" },
};

export function OnboardingPluginPreview({
  pluginID,
}: {
  pluginID: string;
}): JSX.Element | null {
  if (!(pluginID in ONBOARDING_PLUGIN_MASCOT)) return null;

  return (
    <aside
      className="onboarding-plugin-preview"
      data-plugin={pluginID}
      data-testid="onboarding-plugin-preview"
      aria-live="polite"
    >
      {pluginID === "ask-user" ? <AskUserPreview /> : null}
      {pluginID === "todo" ? <TodoPreview /> : null}
      {pluginID === "automation" ? <AutomationPreview /> : null}
      {pluginID === "subagent" ? <SubagentPreview /> : null}
      {pluginID === "memory" ? <MemoryPreview /> : null}
      {pluginID === "dream" ? <DreamPreview /> : null}
      {pluginID === "note-compaction" ? <NotePreview /> : null}
    </aside>
  );
}

function AskUserPreview(): JSX.Element {
  const { t } = useI18n();
  const [active, setActive] = useState("a");
  const options = [
    { id: "a", label: t("onboarding.preview.askUser.optionA"), description: t("onboarding.preview.askUser.optionAHelp") },
    { id: "b", label: t("onboarding.preview.askUser.optionB"), description: t("onboarding.preview.askUser.optionBHelp") },
  ] as const;

  return (
    <section className="user-question-card">
      <p className="user-question-kicker">{t("userQuestion.kicker")}</p>
      <div className="user-question-field">
        <p className="user-question-prompt">{t("onboarding.preview.askUser.question")}</p>
        <div className="user-question-options" role="radiogroup">
          {options.map((option) => (
            <button
              key={option.id}
              type="button"
              className="user-question-option"
              data-active={active === option.id ? "" : undefined}
              aria-pressed={active === option.id}
              onClick={() => setActive(option.id)}
            >
              <span className="user-question-option-indicator" aria-hidden="true">
                {active === option.id ? <CircleDot /> : <Circle />}
              </span>
              <span className="user-question-option-content">
                <span className="user-question-option-label">{option.label}</span>
                <span className="user-question-option-description">{option.description}</span>
              </span>
            </button>
          ))}
        </div>
      </div>
      <div className="user-question-actions">
        <button className="user-question-cancel" type="button" disabled>{t("userQuestion.cancel")}</button>
        <button className="user-question-submit" type="button" disabled>{t("userQuestion.continue")}</button>
      </div>
    </section>
  );
}

function TodoPreview(): JSX.Element {
  const { t } = useI18n();
  return (
    <ol className="onboarding-preview-todo">
      <li data-status="completed">
        <span aria-hidden="true">✓</span>
        {t("onboarding.preview.todo.one")}
      </li>
      <li data-status="in_progress">
        <span aria-hidden="true">●</span>
        {t("onboarding.preview.todo.two")}
      </li>
      <li>
        <span aria-hidden="true">○</span>
        {t("onboarding.preview.todo.three")}
      </li>
    </ol>
  );
}

function AutomationPreview(): JSX.Element {
  const { t } = useI18n();
  return (
    <div className="onboarding-preview-task">
      <span className="onboarding-preview-dot" data-run="completed" aria-hidden="true" />
      <span className="onboarding-preview-task-copy">
        <strong>{t("onboarding.preview.automation.name")}</strong>
        <span>{t("onboarding.preview.automation.schedule")}</span>
      </span>
      <span className="onboarding-preview-task-meta">{t("onboarding.preview.automation.status")}</span>
    </div>
  );
}

function SubagentPreview(): JSX.Element {
  const { t } = useI18n();
  return (
    <div className="onboarding-preview-task">
      <span className="onboarding-preview-dot" data-run="running" aria-hidden="true" />
      <span className="onboarding-preview-task-copy">
        <strong>{t("onboarding.preview.subagent.child")}</strong>
        <span>{t("onboarding.preview.subagent.state")}</span>
      </span>
    </div>
  );
}

function MemoryPreview(): JSX.Element {
  const { t } = useI18n();
  return (
    <div className="onboarding-preview-prose">
      <h3>{t("onboarding.preview.memory.heading")}</h3>
      <ul>
        <li>{t("onboarding.preview.memory.item1")}</li>
        <li>{t("onboarding.preview.memory.item2")}</li>
      </ul>
    </div>
  );
}

function DreamPreview(): JSX.Element {
  const { t } = useI18n();
  return (
    <div className="onboarding-preview-settings">
      <div className="onboarding-preview-setting">
        <span>{t("onboarding.preview.dream.enabled")}</span>
        <strong>{t("onboarding.preview.dream.on")}</strong>
      </div>
      <div className="onboarding-preview-setting">
        <span>{t("onboarding.preview.dream.status")}</span>
        <strong>{t("onboarding.preview.dream.value")}</strong>
      </div>
    </div>
  );
}

function NotePreview(): JSX.Element {
  const { t } = useI18n();
  return (
    <article className="onboarding-preview-note">
      <strong>{t("onboarding.preview.note.title")}</strong>
      <p>{t("onboarding.preview.note.excerpt")}</p>
    </article>
  );
}
