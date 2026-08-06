import { describe, expect, it } from "vitest";
import type { ChannelRoom } from "../shared/protocol";
import { sameChannelRooms } from "./ChannelRoomState";

function room(overrides: Partial<ChannelRoom> = {}): ChannelRoom {
  return {
    id: "room-1",
    kind: "channel",
    name: "Engineering",
    created_by: "human-1",
    created_at: "2026-08-06T00:00:00.000Z",
    unread_count: 2,
    members: [
      {
        room_id: "room-1",
        member_type: "human",
        member_id: "human-1",
        joined_at: "2026-08-06T00:00:00.000Z",
      },
    ],
    ...overrides,
  };
}

describe("sameChannelRooms", () => {
  it("treats equivalent poll snapshots as unchanged", () => {
    const current = [room()];
    const incoming = current.map((item) => ({
      ...item,
      members: item.members.map((member) => ({ ...member })),
    }));

    expect(sameChannelRooms(current, incoming)).toBe(true);
  });

  it("treats an omitted zero unread count as the same visible state", () => {
    expect(
      sameChannelRooms(
        [room({ unread_count: undefined })],
        [room({ unread_count: 0 })],
      ),
    ).toBe(true);
  });

  it("detects user-visible room and membership changes", () => {
    const current = [room()];

    expect(sameChannelRooms(current, [room({ unread_count: 3 })])).toBe(false);
    expect(sameChannelRooms(current, [room({ name: "Product" })])).toBe(false);
    expect(
      sameChannelRooms(current, [
        room({
          members: [
            {
              ...current[0].members[0],
              member_id: "human-2",
            },
          ],
        }),
      ]),
    ).toBe(false);
  });

  it("detects room ordering changes", () => {
    const first = room();
    const second = room({ id: "room-2", name: "Design" });

    expect(sameChannelRooms([first, second], [second, first])).toBe(false);
  });
});
