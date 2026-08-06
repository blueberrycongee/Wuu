import type { Agent } from "../shared/protocol";
import { agentRunning } from "./AppState";
import { DefaultAvatarMark } from "./DefaultAvatar";
import { agentNameForSubagentID } from "./agentNames";
import { useI18n } from "./i18n";
import { Tooltip } from "./Tooltip";

const VISIBLE_SUBAGENT_LIMIT = 6;

export function runningSubagents(agents: readonly Agent[] | undefined): Agent[] {
  return (agents ?? []).filter(agentRunning);
}

function subagentName(agent: Agent): string {
  return (
    agent.participant?.name.trim() ||
    agentNameForSubagentID(agent.id).displayName
  );
}

function SubagentAvatar({ agent }: { agent: Agent }): JSX.Element {
  const avatarImage = agent.participant?.avatar_image?.trim();
  return (
    <span className="composer-subagent-avatar" aria-hidden="true">
      {avatarImage ? (
        <img src={avatarImage} alt="" />
      ) : (
        <DefaultAvatarMark
          seed={agent.participant?.id || agent.id}
          kind={agent.participant?.kind}
        />
      )}
    </span>
  );
}

export function ComposerSubagentStatus({
  agents,
  onSelect,
  onOpenAll,
}: {
  agents: readonly Agent[] | undefined;
  onSelect: (agent: Agent) => void;
  onOpenAll: () => void;
}): JSX.Element | null {
  const { t } = useI18n();
  const running = runningSubagents(agents);
  if (running.length === 0) {
    return null;
  }

  const label = t("composer.subagentsRunning", { count: running.length });
  if (running.length === 1) {
    const agent = running[0];
    const name = subagentName(agent);
    return (
      <Tooltip content={name}>
        <button
          type="button"
          className="composer-subagent-capsule composer-subagent-capsule-single"
          onClick={() => onSelect(agent)}
          aria-label={t("composer.subagentOpen", { name })}
        >
          <span className="composer-subagent-avatar-frame">
            <SubagentAvatar agent={agent} />
          </span>
          <span>{label}</span>
        </button>
      </Tooltip>
    );
  }

  const visible = running.slice(0, VISIBLE_SUBAGENT_LIMIT);
  const overflowCount = running.length - visible.length;
  return (
    <div
      className="composer-subagent-capsule"
      role="group"
      aria-label={label}
    >
      <span className="composer-subagent-avatars">
        {visible.map((agent) => {
          const name = subagentName(agent);
          return (
            <Tooltip key={agent.id} content={name}>
              <button
                type="button"
                className="composer-subagent-avatar-button"
                onClick={() => onSelect(agent)}
                aria-label={t("composer.subagentOpen", { name })}
              >
                <SubagentAvatar agent={agent} />
              </button>
            </Tooltip>
          );
        })}
        {overflowCount > 0 ? (
          <Tooltip content={t("composer.subagentsMore", { count: overflowCount })}>
            <button
              type="button"
              className="composer-subagent-avatar-button composer-subagent-overflow"
              onClick={onOpenAll}
              aria-label={t("composer.subagentsOpenAll")}
            >
              +{overflowCount}
            </button>
          </Tooltip>
        ) : null}
      </span>
      <span>{label}</span>
    </div>
  );
}
