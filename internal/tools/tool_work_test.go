package tools

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/blueberrycongee/wuu/internal/channels"
	"github.com/blueberrycongee/wuu/internal/providers"
)

func TestChatWorkRecordsArtifactAndEvidence(t *testing.T) {
	ctx := context.Background()
	service, err := channels.Open(filepath.Join(t.TempDir(), "channels"), nil)
	if err != nil {
		t.Fatalf("channels.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = service.Close() })
	owner, _ := service.CreateNamedAgent(ctx, channels.CreateNamedAgentParams{Name: "Owner"})
	room, _ := service.CreateRoom(ctx, channels.CreateRoomParams{
		Kind: channels.RoomChannel, Name: "Work", CreatedBy: "local-user",
		Members: []channels.RoomMember{{MemberType: channels.MemberAgent, MemberID: owner.Agent.ID}},
	})
	runtime, _ := service.BindRuntime(ctx, room.RuntimeID)
	task, err := runtime.CreateTask(ctx, channels.TaskCreateParams{RoomID: room.ID, Title: "Fix", OwnerID: owner.Agent.ID})
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	ownerClient, _ := service.BindAgent(ctx, owner.Agent.ID)
	kit, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	kit.SetChatAgent(ownerClient)
	artifactJSON, err := kit.Execute(ctx, providers.ToolCall{Name: "chat_work", Arguments: `{
		"action":"add_artifact","work_id":"` + task.ID + `","artifact_kind":"diff",
		"uri":"artifact://diff","summary":"two files","workspace_revision":"git:abc"}`})
	if err != nil {
		t.Fatalf("chat_work add_artifact error = %v", err)
	}
	var artifactResult struct {
		Artifact channels.WorkArtifact `json:"artifact"`
	}
	if err := json.Unmarshal([]byte(artifactJSON), &artifactResult); err != nil || artifactResult.Artifact.URI != "artifact://diff" {
		t.Fatalf("artifact result = %s, err = %v", artifactJSON, err)
	}
	evidenceJSON, err := kit.Execute(ctx, providers.ToolCall{Name: "chat_work", Arguments: `{
		"action":"evidence","work_id":"` + task.ID + `","checks_summary":"go test passed",
		"changed_files_count":2,"unresolved_items":"none"}`})
	if err != nil {
		t.Fatalf("chat_work evidence error = %v", err)
	}
	var evidenceResult struct {
		Work channels.Work `json:"work"`
	}
	if err := json.Unmarshal([]byte(evidenceJSON), &evidenceResult); err != nil {
		t.Fatalf("decode evidence = %v", err)
	}
	if evidenceResult.Work.ChecksSummary != "go test passed" || evidenceResult.Work.ChangedFilesCount != 2 || len(evidenceResult.Work.Artifacts) != 1 {
		t.Fatalf("evidence work = %#v", evidenceResult.Work)
	}
}
