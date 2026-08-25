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

func TestOpenMigratesLegacyRoomAgentOutOfParticipantTables(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	service, err := Open(dir, nil)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	room, err := service.CreateRoom(ctx, CreateRoomParams{Kind: RoomChannel, Name: "Legacy", CreatedBy: "local-user"})
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}
	runtime, err := service.GetRoomRuntime(ctx, room.RuntimeID)
	if err != nil {
		t.Fatalf("GetRoomRuntime() error = %v", err)
	}
	var tokenHashValue string
	if err := service.db.QueryRowContext(ctx, `SELECT token_hash FROM room_runtimes WHERE id = ?`, runtime.ID).Scan(&tokenHashValue); err != nil {
		t.Fatalf("read runtime token hash: %v", err)
	}
	if _, err := service.db.ExecContext(ctx, `DELETE FROM room_runtimes WHERE id = ?`, runtime.ID); err != nil {
		t.Fatalf("remove separated runtime: %v", err)
	}
	if _, err := service.db.ExecContext(ctx, `
		INSERT INTO named_agents(id, name, kind, room_id, memory_dir, avatar_key, avatar_image,
			engine_override, token_hash, autostart, created_at)
		VALUES (?, ?, 'room', ?, ?, '', '', 'wuu', ?, 1, ?)`, runtime.ID, room.Name,
		room.ID, runtime.MemoryDir, tokenHashValue, toMillis(runtime.CreatedAt)); err != nil {
		t.Fatalf("insert legacy room agent: %v", err)
	}
	if _, err := service.db.ExecContext(ctx, `INSERT INTO room_members(room_id, member_type, member_id, joined_at) VALUES (?, 'agent', ?, ?)`, room.ID, runtime.ID, toMillis(runtime.CreatedAt)); err != nil {
		t.Fatalf("insert legacy runtime membership: %v", err)
	}
	if _, err := service.db.ExecContext(ctx, `INSERT INTO room_cursors(room_id, member_type, member_id, last_read_seq) VALUES (?, 'agent', ?, 0)`, room.ID, runtime.ID); err != nil {
		t.Fatalf("insert legacy runtime cursor: %v", err)
	}
	if err := service.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	service, err = Open(dir, nil)
	if err != nil {
		t.Fatalf("Open(migrated) error = %v", err)
	}
	defer service.Close()
	migrated, err := service.GetRoomRuntime(ctx, runtime.ID)
	if err != nil || migrated.RoomID != room.ID {
		t.Fatalf("migrated runtime = %#v, err = %v", migrated, err)
	}
	for table, query := range map[string]string{
		"named_agents": `SELECT COUNT(*) FROM named_agents WHERE id = ?`,
		"room_members": `SELECT COUNT(*) FROM room_members WHERE member_id = ?`,
		"room_cursors": `SELECT COUNT(*) FROM room_cursors WHERE member_id = ?`,
	} {
		var count int
		if err := service.db.QueryRowContext(ctx, query, runtime.ID).Scan(&count); err != nil || count != 0 {
			t.Fatalf("legacy runtime remained in %s: count=%d err=%v", table, count, err)
		}
	}
}
