import { Plus, Search, Settings } from "lucide-react";
import { useMemo, useState, type PointerEvent as ReactPointerEvent } from "react";
import type { ChannelRoom, NamedAgent } from "../shared/protocol";
import { AgentAvatarMark } from "./AgentAvatarMark";
import { AppModeSwitch } from "./AppModeSwitch";
import { ChannelGroupAvatar } from "./ChannelGroupAvatar";
import { formatChannelUnreadCount } from "./ChannelView";
import { useI18n } from "./i18n";

function searchable(value: string): string {
  return value.normalize("NFKD").replace(/\p{M}/gu, "").toLocaleLowerCase();
}

export function CollaborationSidebar({
  initialized,
  agents,
  rooms,
  selectedAgentID,
  selectedRoomID,
  onSelectAgent,
  onSelectRoom,
  onCreateAgent,
  onCreateRoom,
  onSwitchToHarness,
  onOpenSettings,
  onPointerEnter,
  onPointerLeave,
}: {
  initialized: boolean;
  agents: NamedAgent[];
  rooms: ChannelRoom[];
  selectedAgentID?: string;
  selectedRoomID?: string;
  onSelectAgent: (agentID: string) => void;
  onSelectRoom: (roomID: string) => void;
  onCreateAgent: () => void;
  onCreateRoom: () => void;
  onSwitchToHarness: () => void;
  onOpenSettings: () => void;
  onPointerEnter?: () => void;
  onPointerLeave?: (event: ReactPointerEvent<HTMLElement>) => void;
}): JSX.Element {
  const { t } = useI18n();
  const [query, setQuery] = useState("");
  const normalizedQuery = searchable(query.trim());
  const visibleAgents = useMemo(
    () => normalizedQuery
      ? agents.filter((agent) => searchable(agent.name).includes(normalizedQuery))
      : agents,
    [agents, normalizedQuery],
  );
  const visibleRooms = useMemo(
    () => normalizedQuery
      ? rooms.filter((room) => room.kind === "channel" && searchable(room.name).includes(normalizedQuery))
      : rooms.filter((room) => room.kind === "channel"),
    [normalizedQuery, rooms],
  );
  const directMessagesByAgentID = useMemo(() => {
    const result = new Map<string, ChannelRoom>();
    for (const room of rooms) {
      if (room.kind !== "dm") continue;
      const agentID = room.members.find((member) => member.member_type === "agent")?.member_id;
      if (agentID) result.set(agentID, room);
    }
    return result;
  }, [rooms]);

  return (
    <aside
      className="sidebar collaboration-sidebar"
      data-wuu-component="collaboration-sidebar"
      onPointerEnter={onPointerEnter}
      onPointerLeave={onPointerLeave}
    >
      <div className="sidebar-content">
        <div className="traffic-spacer" />
        <AppModeSwitch
          mode="collaboration"
          collaborationEnabled
          onChange={(mode) => { if (mode === "harness") onSwitchToHarness(); }}
        />

        <div className="collaboration-sidebar-tools">
          <label className="collaboration-sidebar-search">
            <Search aria-hidden="true" />
            <input
              type="search"
              value={query}
              placeholder={t("channels.searchConversations")}
              aria-label={t("channels.searchConversations")}
              onChange={(event) => setQuery(event.currentTarget.value)}
            />
          </label>
        </div>

        <div className="collaboration-sidebar-main">
          <section className="collaboration-contact-section" aria-labelledby="collaboration-agents-label">
            <div className="collaboration-contact-heading">
              <span id="collaboration-agents-label">{t("channels.agents")}</span>
              <button type="button" aria-label={t("channels.newAgent")} title={t("channels.newAgent")} onClick={onCreateAgent}>
                <Plus aria-hidden="true" />
              </button>
            </div>
            <div className="collaboration-contact-list">
              {visibleAgents.map((agent) => {
                const directMessage = directMessagesByAgentID.get(agent.id);
                const selected = selectedAgentID === agent.id || directMessage?.id === selectedRoomID;
                const unread = selected ? 0 : (directMessage?.unread_count ?? 0);
                return (
                  <button
                    className={`collaboration-contact-row${selected ? " active" : ""}${unread > 0 ? " has-unread" : ""}`}
                    type="button"
                    aria-current={selected ? "page" : undefined}
                    key={agent.id}
                    onClick={() => onSelectAgent(agent.id)}
                  >
                    <span className="collaboration-contact-avatar" aria-hidden="true">
                      <AgentAvatarMark seed={agent.id} avatarKey={agent.avatar_key} avatarImage={agent.avatar_image} />
                    </span>
                    <span className="collaboration-contact-copy">
                      <strong>{agent.name}</strong>
                      <small>{t("channels.directMessage")}</small>
                    </span>
                    {unread > 0 ? <span className="collaboration-contact-unread">{formatChannelUnreadCount(unread)}</span> : null}
                  </button>
                );
              })}
              {visibleAgents.length === 0 ? (
                <div className="collaboration-contact-empty">{t(query ? "channels.noMatchingAgents" : "channels.noAgents")}</div>
              ) : null}
            </div>
          </section>

          <section className="collaboration-contact-section" aria-labelledby="collaboration-groups-label">
            <div className="collaboration-contact-heading">
              <span id="collaboration-groups-label">{t("channels.groups")}</span>
              <button type="button" aria-label={t("channels.newRoom")} title={t("channels.newRoom")} onClick={onCreateRoom}>
                <Plus aria-hidden="true" />
              </button>
            </div>
            <div className="collaboration-contact-list">
              {visibleRooms.map((room) => {
                const unread = selectedRoomID === room.id ? 0 : (room.unread_count ?? 0);
                return (
                  <button
                    className={`collaboration-contact-row${selectedRoomID === room.id && !selectedAgentID ? " active" : ""}${unread > 0 ? " has-unread" : ""}`}
                    type="button"
                    aria-current={selectedRoomID === room.id && !selectedAgentID ? "page" : undefined}
                    key={room.id}
                    onClick={() => onSelectRoom(room.id)}
                  >
                    <span className="collaboration-contact-avatar collaboration-room-avatar" aria-hidden="true">
                      <ChannelGroupAvatar room={room} agents={agents} />
                    </span>
                    <span className="collaboration-contact-copy">
                      <strong>{room.name}</strong>
                      <small>{t("channels.groupConversation")}</small>
                    </span>
                    {unread > 0 ? <span className="collaboration-contact-unread">{formatChannelUnreadCount(unread)}</span> : null}
                  </button>
                );
              })}
              {visibleRooms.length === 0 ? (
                <div className="collaboration-contact-empty">{t(query ? "channels.noMatchingConversations" : "sidebar.collaborationEmpty")}</div>
              ) : null}
            </div>
          </section>
        </div>

        <div className="sidebar-settings">
          <button className="sidebar-settings-button" type="button" disabled={!initialized} onClick={onOpenSettings}>
            <Settings className="icon-lg" aria-hidden="true" />
            <span>{t("sidebar.settings")}</span>
          </button>
        </div>
      </div>
    </aside>
  );
}
