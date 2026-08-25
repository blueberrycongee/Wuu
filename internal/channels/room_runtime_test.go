package channels

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestRoomRuntimeNeverEntersParticipantNamespaces(t *testing.T) {
	ctx := context.Background()
	service, err := Open(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer service.Close()
	agent, err := service.CreateNamedAgent(ctx, CreateNamedAgentParams{Name: "Owner"})
	if err != nil {
		t.Fatalf("CreateNamedAgent() error = %v", err)
	}
	room, err := service.CreateRoom(ctx, CreateRoomParams{
		Kind: RoomChannel, Name: "Private runtime", CreatedBy: "local-user",
		Members: []RoomMember{{MemberType: MemberAgent, MemberID: agent.Agent.ID}},
	})
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}
	if room.RuntimeID == "" {
		t.Fatal("room has no runtime")
	}
	if _, err := service.GetNamedAgent(ctx, room.RuntimeID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetNamedAgent(runtime) error = %v, want not found", err)
	}
	if _, err := service.BindAgent(ctx, room.RuntimeID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("BindAgent(runtime) error = %v, want not found", err)
	}
	if _, err := service.BindRuntime(ctx, room.RuntimeID); err != nil {
		t.Fatalf("BindRuntime() error = %v", err)
	}
	for table, query := range map[string]string{
		"named_agents": `SELECT COUNT(*) FROM named_agents WHERE id = ?`,
		"room_members": `SELECT COUNT(*) FROM room_members WHERE member_id = ?`,
		"room_cursors": `SELECT COUNT(*) FROM room_cursors WHERE member_id = ?`,
	} {
		var count int
		if err := service.db.QueryRowContext(ctx, query, room.RuntimeID).Scan(&count); err != nil {
			t.Fatalf("query %s: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("runtime leaked into %s", table)
		}
	}
	raw, err := json.Marshal(room)
	if err != nil {
		t.Fatalf("Marshal(room) error = %v", err)
	}
	if strings.Contains(string(raw), room.RuntimeID) || strings.Contains(string(raw), "runtime_id") || strings.Contains(string(raw), "agent_id") {
		t.Fatalf("serialized room exposed runtime: %s", raw)
	}
}

func TestRuntimeTaskProjectionUsesVisibleOwnerAsAuthor(t *testing.T) {
	ctx := context.Background()
	service, err := Open(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer service.Close()
	owner, _ := service.CreateNamedAgent(ctx, CreateNamedAgentParams{Name: "Owner"})
	room, _ := service.CreateRoom(ctx, CreateRoomParams{
		Kind: RoomChannel, Name: "Work", CreatedBy: "local-user",
		Members: []RoomMember{{MemberType: MemberAgent, MemberID: owner.Agent.ID}},
	})
	runtime, err := service.BindRuntime(ctx, room.RuntimeID)
	if err != nil {
		t.Fatalf("BindRuntime() error = %v", err)
	}
	task, err := runtime.CreateTask(ctx, TaskCreateParams{RoomID: room.ID, Title: "Check", OwnerID: owner.Agent.ID})
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if task.AuthorType != MemberAgent || task.AuthorID != owner.Agent.ID {
		t.Fatalf("task author = %s/%s, want visible owner", task.AuthorType, task.AuthorID)
	}
	if task.AuthorID == room.RuntimeID {
		t.Fatal("task projection exposed the room runtime as an author")
	}
}
