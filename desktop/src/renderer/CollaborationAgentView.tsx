import { MessageSquareText, Settings2 } from "lucide-react";
import type { NamedAgent } from "../shared/protocol";
import { AgentAvatarMark } from "./AgentAvatarMark";
import { useI18n } from "./i18n";

export function CollaborationAgentView({ agent, onManage }: { agent: NamedAgent; onManage: () => void }): JSX.Element {
  const { t } = useI18n();

  return (
    <section className="collaboration-agent-view" aria-label={agent.name}>
      <header className="collaboration-agent-header">
        <span className="collaboration-agent-header-avatar" aria-hidden="true">
          <AgentAvatarMark seed={agent.id} avatarKey={agent.avatar_key} avatarImage={agent.avatar_image} />
        </span>
        <div>
          <h2>{agent.name}</h2>
          <span>{t("channels.directMessage")}</span>
        </div>
        <button className="icon-button collaboration-agent-manage" type="button" aria-label={t("channels.editAgent")} onClick={onManage}>
          <Settings2 aria-hidden="true" />
        </button>
      </header>
      <div className="collaboration-agent-empty">
        <MessageSquareText aria-hidden="true" />
        <h3>{t("channels.directMessageWith", { name: agent.name })}</h3>
        <p>{t("channels.directMessagesComingSoon")}</p>
      </div>
    </section>
  );
}
