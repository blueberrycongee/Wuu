import type { ChannelRoom, ChannelRoomMember } from "../shared/protocol";

export function sameChannelRooms(
  current: readonly ChannelRoom[],
  incoming: readonly ChannelRoom[],
): boolean {
  return (
    current.length === incoming.length &&
    current.every((room, index) => sameChannelRoom(room, incoming[index]))
  );
}

function sameChannelRoom(
  current: ChannelRoom,
  incoming: ChannelRoom | undefined,
): boolean {
  return (
    incoming !== undefined &&
    current.id === incoming.id &&
    current.kind === incoming.kind &&
    current.name === incoming.name &&
    current.avatar_image === incoming.avatar_image &&
    current.created_by === incoming.created_by &&
    current.created_at === incoming.created_at &&
    (current.unread_count ?? 0) === (incoming.unread_count ?? 0) &&
    current.members.length === incoming.members.length &&
    current.members.every((member, index) =>
      sameChannelRoomMember(member, incoming.members[index]),
    )
  );
}

function sameChannelRoomMember(
  current: ChannelRoomMember,
  incoming: ChannelRoomMember | undefined,
): boolean {
  return (
    incoming !== undefined &&
    current.room_id === incoming.room_id &&
    current.member_type === incoming.member_type &&
    current.member_id === incoming.member_id &&
    current.joined_at === incoming.joined_at
  );
}
