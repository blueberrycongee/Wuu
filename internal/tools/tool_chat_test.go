package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blueberrycongee/wuu/internal/channels"
	"github.com/blueberrycongee/wuu/internal/modelprofile"
	"github.com/blueberrycongee/wuu/internal/providers"
	"github.com/blueberrycongee/wuu/internal/toolresult"
)

func TestNamedAgentChatToolsAreIsolatedAndRoundTrip(t *testing.T) {
	ctx := context.Background()
	service, err := channels.Open(filepath.Join(t.TempDir(), "channels"), nil)
	if err != nil {
		t.Fatalf("channels.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = service.Close() })
	credential, err := service.CreateNamedAgent(ctx, channels.CreateNamedAgentParams{Name: "Alpha"})
	if err != nil {
		t.Fatalf("CreateNamedAgent() error = %v", err)
	}
	room, err := service.OpenDirectMessage(ctx, "human-1", credential.Agent.ID)
	if err != nil {
		t.Fatalf("OpenDirectMessage() error = %v", err)
	}
	kit, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("tools.New() error = %v", err)
	}
	profile := modelprofile.Profile{ProviderName: "openai", Model: "test", Family: modelprofile.FamilyCodex}
	kit.SetActiveProfile(profile, true)
	assertDefinitionMissing(t, kit.Definitions(), "chat_check")
	assertDefinitionMissing(t, kit.Definitions(), "chat_draft")
	assertDefinitionMissing(t, kit.Definitions(), "chat_task")
	assertDefinitionMissing(t, kit.Definitions(), "chat_verify")
	assertDefinitionMissing(t, kit.Definitions(), "chat_remind")
	if _, err := kit.Execute(ctx, providers.ToolCall{Name: "chat_check", Arguments: `{}`}); err == nil {
		t.Fatal("ordinary session executed chat_check")
	}

	client, err := service.BindAgent(ctx, credential.Agent.ID)
	if err != nil {
		t.Fatalf("BindAgent() error = %v", err)
	}
	kit.SetChatAgent(client)
	for _, name := range []string{"chat_check", "chat_read", "chat_send", "collaboration_send", "chat_draft", "chat_task", "chat_remind"} {
		assertDefinitionPresent(t, kit.Definitions(), name)
	}
	assertDefinitionMissing(t, kit.Definitions(), "chat_verify")

	sentJSON, err := kit.Execute(ctx, providers.ToolCall{Name: "chat_send", Arguments: `{"room_id":"` + room.ID + `","kind":"text","body":"reviewed","basis_seq":0}`})
	if err != nil {
		t.Fatalf("chat_send error = %v", err)
	}
	var sent struct {
		Status  string           `json:"status"`
		Message channels.Message `json:"message"`
	}
	if err := json.Unmarshal([]byte(sentJSON), &sent); err != nil {
		t.Fatalf("decode chat_send = %v: %s", err, sentJSON)
	}
	if sent.Status != "committed" || sent.Message.AuthorID != credential.Agent.ID || sent.Message.Seq != 1 {
		t.Fatalf("chat_send result = %#v", sent)
	}

	readJSON, err := kit.Execute(ctx, providers.ToolCall{Name: "chat_read", Arguments: `{"room_id":"` + room.ID + `","after_seq":0,"limit":10}`})
	if err != nil {
		t.Fatalf("chat_read error = %v", err)
	}
	var read struct {
		Messages []channels.Message `json:"messages"`
	}
	if err := json.Unmarshal([]byte(readJSON), &read); err != nil {
		t.Fatalf("decode chat_read = %v: %s", err, readJSON)
	}
	if len(read.Messages) != 1 || read.Messages[0].Body != "reviewed" {
		t.Fatalf("chat_read messages = %#v", read.Messages)
	}

	if _, err := service.SendHuman(ctx, channels.HumanSendParams{
		RoomID: room.ID, HumanID: "human-1", Body: "@Alpha follow up",
		Images: []channels.MessageImage{{MediaType: "image/png", Data: "aW1hZ2U="}},
	}); err != nil {
		t.Fatalf("SendHuman() error = %v", err)
	}
	checkJSON, err := kit.Execute(ctx, providers.ToolCall{Name: "chat_check", Arguments: `{}`})
	if err != nil {
		t.Fatalf("chat_check error = %v", err)
	}
	var check channels.CheckResult
	if err := json.Unmarshal([]byte(checkJSON), &check); err != nil {
		t.Fatalf("decode chat_check = %v: %s", err, checkJSON)
	}
	if len(check.Items) != 1 || check.Items[0].MessageID == "" {
		t.Fatalf("chat_check result = %#v", check)
	}
	richRead, err := kit.ExecuteResult(ctx, providers.ToolCall{Name: "chat_read", Arguments: `{"item_ids":["` + check.Items[0].ID + `"]}`})
	if err != nil {
		t.Fatalf("rich chat_read error = %v", err)
	}
	if len(richRead.Content) != 2 || richRead.Content[0].Type != toolresult.ContentTypeText || richRead.Content[1].Type != toolresult.ContentTypeImage {
		t.Fatalf("rich chat_read content = %#v", richRead.Content)
	}
	if richRead.Content[1].Data != "aW1hZ2U=" || strings.Contains(richRead.Content[0].Text, "aW1hZ2U=") {
		t.Fatalf("rich chat_read did not separate binary payload: %#v", richRead.Content)
	}
	if err := json.Unmarshal([]byte(richRead.Content[0].Text), &read); err != nil {
		t.Fatalf("decode rich chat_read text = %v: %s", err, richRead.Content[0].Text)
	}
	if len(read.Messages) != 1 || read.Messages[0].Body != "@Alpha follow up" || len(read.Messages[0].Images) != 1 {
		t.Fatalf("rich chat_read messages = %#v", read.Messages)
	}

	kit.SetImageInputSupported(false)
	omittedRead, err := kit.ExecuteResult(ctx, providers.ToolCall{Name: "chat_read", Arguments: `{"room_id":"` + room.ID + `","after_seq":0,"limit":10}`})
	if err != nil {
		t.Fatalf("non-vision chat_read error = %v", err)
	}
	if len(omittedRead.Content) != 2 || omittedRead.Content[0].Type != toolresult.ContentTypeText || omittedRead.Content[1].Text != "[1 image omitted: unsupported]" {
		t.Fatalf("non-vision chat_read content = %#v", omittedRead.Content)
	}
	if strings.Contains(omittedRead.Content[0].Text, "aW1hZ2U=") || strings.Contains(omittedRead.Content[0].Text, `"images"`) || omittedRead.Content[1].Type == toolresult.ContentTypeImage {
		t.Fatalf("non-vision chat_read exposed image data: %#v", omittedRead.Content)
	}

	heldJSON, err := kit.Execute(ctx, providers.ToolCall{Name: "chat_send", Arguments: `{"room_id":"` + room.ID + `","kind":"text","body":"stale answer","basis_seq":1}`})
	if err != nil {
		t.Fatalf("stale chat_send error = %v", err)
	}
	var held struct {
		Status string              `json:"status"`
		Draft  channels.Draft      `json:"draft"`
		Delta  channels.DraftDelta `json:"delta"`
	}
	if err := json.Unmarshal([]byte(heldJSON), &held); err != nil {
		t.Fatalf("decode held chat_send = %v: %s", err, heldJSON)
	}
	if held.Status != "held" || held.Draft.ID == "" || held.Delta.Count != 1 {
		t.Fatalf("held chat_send result = %#v", held)
	}
	listJSON, err := kit.Execute(ctx, providers.ToolCall{Name: "chat_draft", Arguments: `{"action":"list"}`})
	if err != nil || !strings.Contains(listJSON, held.Draft.ID) {
		t.Fatalf("chat_draft list = %s, err = %v", listJSON, err)
	}
	resolvedJSON, err := kit.Execute(ctx, providers.ToolCall{Name: "chat_draft", Arguments: `{"action":"resolve","draft_id":"` + held.Draft.ID + `","resolution":"silent"}`})
	if err != nil || !strings.Contains(resolvedJSON, `"state":"dropped"`) {
		t.Fatalf("chat_draft silent = %s, err = %v", resolvedJSON, err)
	}

	taskJSON, err := kit.Execute(ctx, providers.ToolCall{Name: "chat_task", Arguments: `{"action":"create","room_id":"` + room.ID + `","title":"Review","owner_id":"` + credential.Agent.ID + `"}`})
	if err != nil {
		t.Fatalf("chat_task create error = %v", err)
	}
	var taskResult struct {
		Task channels.Message `json:"task"`
	}
	if err := json.Unmarshal([]byte(taskJSON), &taskResult); err != nil || taskResult.Task.TaskState != string(channels.TaskStateOpen) {
		t.Fatalf("chat_task create = %s, err %v", taskJSON, err)
	}
	updatedTaskJSON, err := kit.Execute(ctx, providers.ToolCall{Name: "chat_task", Arguments: `{"action":"update","task_id":"` + taskResult.Task.ID + `","state":"done"}`})
	if err != nil || !strings.Contains(updatedTaskJSON, `"task_state":"done"`) {
		t.Fatalf("chat_task update = %s, err %v", updatedTaskJSON, err)
	}
	listedTaskJSON, err := kit.Execute(ctx, providers.ToolCall{Name: "chat_task", Arguments: `{"action":"list","room_id":"` + room.ID + `"}`})
	if err != nil || !strings.Contains(listedTaskJSON, taskResult.Task.ID) {
		t.Fatalf("chat_task list = %s, err %v", listedTaskJSON, err)
	}

	reminderJSON, err := kit.Execute(ctx, providers.ToolCall{Name: "chat_remind", Arguments: `{"action":"set","after":"1m","note":"resume"}`})
	if err != nil {
		t.Fatalf("chat_remind set error = %v", err)
	}
	var reminderResult struct {
		Reminder channels.Reminder `json:"reminder"`
	}
	if err := json.Unmarshal([]byte(reminderJSON), &reminderResult); err != nil || reminderResult.Reminder.State != channels.ReminderPending {
		t.Fatalf("chat_remind set = %s, err %v", reminderJSON, err)
	}
	listedReminderJSON, err := kit.Execute(ctx, providers.ToolCall{Name: "chat_remind", Arguments: `{"action":"list","state":"pending"}`})
	if err != nil || !strings.Contains(listedReminderJSON, reminderResult.Reminder.ID) {
		t.Fatalf("chat_remind list = %s, err %v", listedReminderJSON, err)
	}
	cancelledReminderJSON, err := kit.Execute(ctx, providers.ToolCall{Name: "chat_remind", Arguments: `{"action":"cancel","reminder_id":"` + reminderResult.Reminder.ID + `"}`})
	if err != nil || !strings.Contains(cancelledReminderJSON, `"state":"cancelled"`) {
		t.Fatalf("chat_remind cancel = %s, err %v", cancelledReminderJSON, err)
	}
}

func TestChatVerifyIsAvailableOnlyToHiddenRoomRuntime(t *testing.T) {
	ctx := context.Background()
	service, err := channels.Open(filepath.Join(t.TempDir(), "channels"), nil)
	if err != nil {
		t.Fatalf("channels.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = service.Close() })
	owner, err := service.CreateNamedAgent(ctx, channels.CreateNamedAgentParams{Name: "Andy"})
	if err != nil {
		t.Fatalf("CreateNamedAgent() error = %v", err)
	}
	room, err := service.CreateRoom(ctx, channels.CreateRoomParams{
		Kind: channels.RoomChannel, Name: "Build", CreatedBy: "local-user",
		Members: []channels.RoomMember{
			{MemberType: channels.MemberHuman, MemberID: "local-user"},
			{MemberType: channels.MemberAgent, MemberID: owner.Agent.ID},
		},
	})
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}
	client, err := service.BindAgent(ctx, room.AgentID)
	if err != nil {
		t.Fatalf("BindAgent(room runtime) error = %v", err)
	}
	kit, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("tools.New() error = %v", err)
	}
	kit.SetActiveProfile(modelprofile.Profile{ProviderName: "openai", Model: "test", Family: modelprofile.FamilyCodex}, true)
	kit.SetChatAgent(client)
	assertDefinitionPresent(t, kit.Definitions(), "chat_verify")
	assertDefinitionMissing(t, kit.Definitions(), "chat_send")

	taskJSON, err := kit.Execute(ctx, providers.ToolCall{Name: "chat_task", Arguments: `{"action":"create","room_id":"` + room.ID + `","title":"Fix callback","owner_id":"` + owner.Agent.ID + `","verification_required":true}`})
	if err != nil {
		t.Fatalf("chat_task create error = %v", err)
	}
	var taskResult struct {
		Task channels.Message `json:"task"`
	}
	if err := json.Unmarshal([]byte(taskJSON), &taskResult); err != nil {
		t.Fatalf("decode task result: %v", err)
	}
	revisedJSON, err := kit.Execute(ctx, providers.ToolCall{Name: "chat_task", Arguments: `{"action":"revise","room_id":"` + room.ID + `","task_id":"` + taskResult.Task.ID + `","body":"Reject replayed and expired state"}`})
	if err != nil || !strings.Contains(revisedJSON, `"task_goal_revision":2`) {
		t.Fatalf("chat_task revise = %s, err = %v", revisedJSON, err)
	}
	checking, err := service.UpdateTask(ctx, channels.TaskUpdateParams{
		TaskID: taskResult.Task.ID, State: channels.TaskStateChecking,
		AgentID: owner.Agent.ID, Token: owner.Token,
	})
	if err != nil {
		t.Fatalf("mark task checking: %v", err)
	}
	verifyArgs := fmt.Sprintf(
		`{"room_id":%q,"task_id":%q,"goal_revision":%d,"candidate_revision":%d,"decision":"block","report":"Replay still succeeds."}`,
		room.ID, taskResult.Task.ID, checking.TaskGoalRevision, checking.TaskCandidateRevision,
	)
	verifiedJSON, err := kit.Execute(ctx, providers.ToolCall{Name: "chat_verify", Arguments: verifyArgs})
	if err != nil || !strings.Contains(verifiedJSON, `"decision":"block"`) || !strings.Contains(verifiedJSON, `"kind":"verification_feedback"`) {
		t.Fatalf("chat_verify = %s, err = %v", verifiedJSON, err)
	}
}

func assertDefinitionPresent(t *testing.T, definitions []providers.ToolDefinition, name string) {
	t.Helper()
	for _, definition := range definitions {
		if definition.Name == name {
			return
		}
	}
	t.Fatalf("tool definition %q missing", name)
}

func assertDefinitionMissing(t *testing.T, definitions []providers.ToolDefinition, name string) {
	t.Helper()
	for _, definition := range definitions {
		if definition.Name == name {
			t.Fatalf("tool definition %q unexpectedly present", name)
		}
	}
}
