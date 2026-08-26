package channels

import (
	"context"
	"testing"
)

func TestRoomRuntimeCanInviteAndCreatePersistentNamedAgents(t *testing.T) {
	ctx := context.Background()
	wake := &recordingWakeSink{}
	service := openTestService(t, wake)
	andy := createTestAgent(t, service, "Andy")
	reviewer := createTestAgent(t, service, "Reviewer")
	room := createTestRoom(t, service, andy)
	client, err := service.BindRuntime(ctx, room.RuntimeID)
	if err != nil {
		t.Fatalf("BindRuntime() error = %v", err)
	}

	roster, err := client.RoomRoster(ctx, room.ID)
	if err != nil {
		t.Fatalf("RoomRoster() error = %v", err)
	}
	if roster.MembershipRevision != 1 || len(roster.Members) != 1 || len(roster.AvailableAgents) != 1 || roster.AvailableAgents[0].Name != "Reviewer" {
		t.Fatalf("initial roster = %#v", roster)
	}

	invited, err := client.InviteRoomAgent(ctx, room.ID, reviewer.Agent.ID)
	if err != nil {
		t.Fatalf("InviteRoomAgent() error = %v", err)
	}
	if invited.MembershipRevision != 2 || len(invited.Members) != 3 {
		t.Fatalf("invited room = %#v", invited)
	}

	created, err := client.CreateAndInviteRoomAgent(ctx, room.ID, "Designer", "界面设计与交互评审")
	if err != nil {
		t.Fatalf("CreateAndInviteRoomAgent() error = %v", err)
	}
	if created.Agent.Name != "Designer" || created.Agent.Role != "界面设计与交互评审" || created.Room.MembershipRevision != 3 || len(created.Room.Members) != 4 {
		t.Fatalf("created room agent = %#v", created)
	}
	persisted, err := service.GetNamedAgent(ctx, created.Agent.ID)
	if err != nil || !persisted.Autostart {
		t.Fatalf("created named agent was not persisted: %v", err)
	}
}

func TestNamedAgentCannotManageRoomRoster(t *testing.T) {
	ctx := context.Background()
	service := openTestService(t, nil)
	andy := createTestAgent(t, service, "Andy")
	room := createTestRoom(t, service, andy)
	client, err := service.BindAgent(ctx, andy.Agent.ID)
	if err != nil {
		t.Fatalf("BindAgent() error = %v", err)
	}
	if _, err := client.RoomRoster(ctx, room.ID); err != ErrUnauthorized {
		t.Fatalf("RoomRoster() error = %v, want ErrUnauthorized", err)
	}
}
