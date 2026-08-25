package channels

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestTaskVerificationPersistsAndWakesVisibleOwner(t *testing.T) {
	ctx := context.Background()
	sink := &recordingWakeSink{}
	service := openTestService(t, sink)
	owner := createTestAgent(t, service, "Andy")
	room, err := service.CreateRoom(ctx, CreateRoomParams{
		Kind: RoomChannel, Name: "Build", CreatedBy: "local-user",
		Members: []RoomMember{
			{MemberType: MemberHuman, MemberID: "local-user"},
			{MemberType: MemberAgent, MemberID: owner.Agent.ID},
		},
	})
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}
	roomRuntime, err := service.BindAgent(ctx, room.AgentID)
	if err != nil {
		t.Fatalf("BindAgent(room runtime) error = %v", err)
	}
	task, err := roomRuntime.CreateTask(ctx, TaskCreateParams{
		RoomID: room.ID, Title: "Fix callback", Body: "Reject replayed state", OwnerID: owner.Agent.ID,
		VerificationRequired: true,
	})
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	sink.take() // Assignment wake is not part of the verification assertion.
	if _, err := service.Check(ctx, owner.Agent.ID, owner.Token); err != nil {
		t.Fatalf("Check(owner assignment) error = %v", err)
	}
	if _, err := service.UpdateTask(ctx, TaskUpdateParams{
		TaskID: task.ID, State: TaskStateDone, AgentID: owner.Agent.ID, Token: owner.Token,
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("unverified completion error = %v, want conflict", err)
	}
	checking, err := service.UpdateTask(ctx, TaskUpdateParams{
		TaskID: task.ID, State: TaskStateChecking, AgentID: owner.Agent.ID, Token: owner.Token,
	})
	if err != nil || checking.TaskGoalRevision != 1 || checking.TaskCandidateRevision != 1 {
		t.Fatalf("initial checking task = %#v, err = %v", checking, err)
	}

	blocked, err := roomRuntime.SubmitTaskVerification(ctx, TaskVerificationSubmitParams{
		RoomID: room.ID, TaskID: task.ID, Decision: VerificationBlock,
		Report: "The replay test still creates a second session.", GoalRevision: 1, CandidateRevision: 1,
	})
	if err != nil {
		t.Fatalf("SubmitTaskVerification(block) error = %v", err)
	}
	if blocked.Verification.Attempt != 1 || blocked.Verification.OwnerID != owner.Agent.ID {
		t.Fatalf("blocked verification = %#v", blocked.Verification)
	}
	tasks, err := service.ListTasks(ctx, TaskListParams{RoomID: room.ID, AgentID: owner.Agent.ID, Token: owner.Token})
	if err != nil || len(tasks) != 1 || tasks[0].TaskState != string(TaskStateRevising) {
		t.Fatalf("blocked task projection = %#v, err = %v", tasks, err)
	}
	if blocked.Delivery.Kind != CollaborationVerificationFeedback || blocked.Delivery.SourceMessageID != task.ID {
		t.Fatalf("verification delivery = %#v", blocked.Delivery)
	}
	if got := sink.take(); len(got) != 1 || got[0] != owner.Agent.ID {
		t.Fatalf("verification wakes = %v, want owner", got)
	}

	checked, err := service.Check(ctx, owner.Agent.ID, owner.Token)
	if err != nil {
		t.Fatalf("Check(owner) error = %v", err)
	}
	found := false
	for _, delivery := range checked.Collaboration {
		if delivery.ID != blocked.Delivery.ID {
			continue
		}
		found = delivery.Kind == CollaborationVerificationFeedback &&
			strings.Contains(delivery.Body, "Verification block") &&
			strings.Contains(delivery.Body, "task is now revising")
	}
	if !found {
		t.Fatalf("owner collaboration missing verification feedback: %#v", checked.Collaboration)
	}
	if _, err := service.UpdateTask(ctx, TaskUpdateParams{
		TaskID: task.ID, State: TaskStateDoing, AgentID: owner.Agent.ID, Token: owner.Token,
	}); err != nil {
		t.Fatalf("UpdateTask(repair doing) error = %v", err)
	}
	checking, err = service.UpdateTask(ctx, TaskUpdateParams{
		TaskID: task.ID, State: TaskStateChecking, AgentID: owner.Agent.ID, Token: owner.Token,
	})
	if err != nil || checking.TaskCandidateRevision != 2 {
		t.Fatalf("repair checking task = %#v, err = %v", checking, err)
	}

	passed, err := roomRuntime.SubmitTaskVerification(ctx, TaskVerificationSubmitParams{
		RoomID: room.ID, TaskID: task.ID, Decision: VerificationPass,
		Report: "Replay is rejected and focused tests pass.", GoalRevision: 1, CandidateRevision: 2,
	})
	if err != nil {
		t.Fatalf("SubmitTaskVerification(pass) error = %v", err)
	}
	if passed.Verification.Attempt != 2 || !strings.Contains(passed.Delivery.Body, "Publish the result") {
		t.Fatalf("passed verification = %#v, delivery = %#v", passed.Verification, passed.Delivery)
	}
	tasks, err = service.ListTasks(ctx, TaskListParams{RoomID: room.ID, AgentID: owner.Agent.ID, Token: owner.Token})
	if err != nil || len(tasks) != 1 || tasks[0].TaskState != string(TaskStateChecking) {
		t.Fatalf("passed task projection = %#v, err = %v", tasks, err)
	}
	persisted, err := service.GetTaskVerification(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTaskVerification() error = %v", err)
	}
	if persisted.Decision != VerificationPass || persisted.Attempt != 2 || persisted.GoalRevision != 1 ||
		persisted.CandidateRevision != 2 || persisted.Report != "Replay is rejected and focused tests pass." {
		t.Fatalf("persisted verification = %#v", persisted)
	}
	completed, err := service.UpdateTask(ctx, TaskUpdateParams{
		TaskID: task.ID, State: TaskStateDone, AgentID: owner.Agent.ID, Token: owner.Token,
	})
	if err != nil || completed.TaskState != string(TaskStateDone) {
		t.Fatalf("verified completion = %#v, err = %v", completed, err)
	}
}

func TestTaskVerificationRejectsVisibleAgentAndInvalidDecision(t *testing.T) {
	ctx := context.Background()
	service := openTestService(t, nil)
	owner := createTestAgent(t, service, "Andy")
	room, err := service.CreateRoom(ctx, CreateRoomParams{
		Kind: RoomChannel, Name: "Build", CreatedBy: "local-user",
		Members: []RoomMember{
			{MemberType: MemberHuman, MemberID: "local-user"},
			{MemberType: MemberAgent, MemberID: owner.Agent.ID},
		},
	})
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}
	roomRuntime, _ := service.BindAgent(ctx, room.AgentID)
	task, err := roomRuntime.CreateTask(ctx, TaskCreateParams{
		RoomID: room.ID, Title: "Fix", OwnerID: owner.Agent.ID,
	})
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	ownerClient, _ := service.BindAgent(ctx, owner.Agent.ID)
	_, err = ownerClient.SubmitTaskVerification(ctx, TaskVerificationSubmitParams{
		RoomID: room.ID, TaskID: task.ID, Decision: VerificationPass, Report: "looks good",
	})
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("visible owner verification error = %v, want unauthorized", err)
	}
	_, err = roomRuntime.SubmitTaskVerification(ctx, TaskVerificationSubmitParams{
		RoomID: room.ID, TaskID: task.ID, Decision: "maybe", Report: "unclear",
	})
	if err == nil || !strings.Contains(err.Error(), "invalid verification decision") {
		t.Fatalf("invalid decision error = %v", err)
	}
}

func TestTaskVerificationRejectsStaleGoalAndCandidateRevisions(t *testing.T) {
	ctx := context.Background()
	service := openTestService(t, nil)
	owner := createTestAgent(t, service, "Andy")
	room, err := service.CreateRoom(ctx, CreateRoomParams{
		Kind: RoomChannel, Name: "Build", CreatedBy: "local-user",
		Members: []RoomMember{
			{MemberType: MemberHuman, MemberID: "local-user"},
			{MemberType: MemberAgent, MemberID: owner.Agent.ID},
		},
	})
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}
	roomRuntime, _ := service.BindAgent(ctx, room.AgentID)
	task, err := roomRuntime.CreateTask(ctx, TaskCreateParams{
		RoomID: room.ID, Title: "Fix callback", Body: "Reject replayed state",
		OwnerID: owner.Agent.ID, VerificationRequired: true,
	})
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	ownerClient, _ := service.BindAgent(ctx, owner.Agent.ID)
	checking, err := ownerClient.UpdateTask(ctx, TaskUpdateParams{TaskID: task.ID, State: TaskStateChecking})
	if err != nil {
		t.Fatalf("UpdateTask(checking) error = %v", err)
	}
	if _, err := ownerClient.UpdateTask(ctx, TaskUpdateParams{TaskID: task.ID, GoalCorrection: "owner rewrite"}); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("owner goal correction error = %v, want unauthorized", err)
	}
	revised, err := roomRuntime.UpdateTask(ctx, TaskUpdateParams{
		TaskID: task.ID, GoalCorrection: "Reject replayed and expired state",
	})
	if err != nil {
		t.Fatalf("UpdateTask(goal correction) error = %v", err)
	}
	if revised.TaskGoalRevision != 2 || revised.TaskState != string(TaskStateOpen) {
		t.Fatalf("revised task = %#v", revised)
	}
	_, err = roomRuntime.SubmitTaskVerification(ctx, TaskVerificationSubmitParams{
		RoomID: room.ID, TaskID: task.ID, Decision: VerificationPass, Report: "old goal passed",
		GoalRevision: checking.TaskGoalRevision, CandidateRevision: checking.TaskCandidateRevision,
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("stale goal verification error = %v, want conflict", err)
	}

	checking, err = ownerClient.UpdateTask(ctx, TaskUpdateParams{TaskID: task.ID, State: TaskStateChecking})
	if err != nil {
		t.Fatalf("UpdateTask(revised checking) error = %v", err)
	}
	if _, err := roomRuntime.SubmitTaskVerification(ctx, TaskVerificationSubmitParams{
		RoomID: room.ID, TaskID: task.ID, Decision: VerificationPass, Report: "new goal passed",
		GoalRevision: checking.TaskGoalRevision, CandidateRevision: checking.TaskCandidateRevision,
	}); err != nil {
		t.Fatalf("SubmitTaskVerification(new goal) error = %v", err)
	}
	if _, err := ownerClient.UpdateTask(ctx, TaskUpdateParams{TaskID: task.ID, State: TaskStateDoing}); err != nil {
		t.Fatalf("UpdateTask(post-pass doing) error = %v", err)
	}
	if _, err := ownerClient.UpdateTask(ctx, TaskUpdateParams{TaskID: task.ID, State: TaskStateDone}); !errors.Is(err, ErrConflict) {
		t.Fatalf("completion after candidate changed error = %v, want conflict", err)
	}
}

func TestCandidateAndFeedbackDeliveriesRecoverAfterRestart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	service, err := Open(dir, nil)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	owner := createTestAgent(t, service, "Andy")
	room, err := service.CreateRoom(ctx, CreateRoomParams{
		Kind: RoomChannel, Name: "Build", CreatedBy: "local-user",
		Members: []RoomMember{
			{MemberType: MemberHuman, MemberID: "local-user"},
			{MemberType: MemberAgent, MemberID: owner.Agent.ID},
		},
	})
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}
	roomRuntime, _ := service.BindAgent(ctx, room.AgentID)
	ownerClient, _ := service.BindAgent(ctx, owner.Agent.ID)
	task, err := roomRuntime.CreateTask(ctx, TaskCreateParams{
		RoomID: room.ID, Title: "Fix callback", OwnerID: owner.Agent.ID, VerificationRequired: true,
	})
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if _, err := ownerClient.Check(ctx); err != nil {
		t.Fatalf("Check(owner assignment) error = %v", err)
	}
	checking, err := ownerClient.UpdateTask(ctx, TaskUpdateParams{TaskID: task.ID, State: TaskStateChecking})
	if err != nil {
		t.Fatalf("UpdateTask(checking) error = %v", err)
	}
	delivery, err := ownerClient.SendCollaboration(ctx, CollaborationSendParams{
		RoomID: room.ID, ToAgentID: room.AgentID, Kind: CollaborationCandidateReady,
		SourceMessageID: task.ID, Body: "Candidate is ready; focused tests pass.",
	})
	if err != nil {
		t.Fatalf("SendCollaboration(candidate ready) error = %v", err)
	}
	if delivery.GoalRevision != checking.TaskGoalRevision || delivery.CandidateRevision != checking.TaskCandidateRevision {
		t.Fatalf("candidate delivery = %#v", delivery)
	}
	if _, err := roomRuntime.Check(ctx); err != nil {
		t.Fatalf("Check(room candidate) error = %v", err)
	}
	if err := service.Close(); err != nil {
		t.Fatalf("Close(before candidate recovery) error = %v", err)
	}

	service, err = Open(dir, nil)
	if err != nil {
		t.Fatalf("Open(candidate recovery) error = %v", err)
	}
	roomRuntime, _ = service.BindAgent(ctx, room.AgentID)
	recovered, err := roomRuntime.Check(ctx)
	if err != nil || len(recovered.Collaboration) != 1 || recovered.Collaboration[0].ID != delivery.ID {
		t.Fatalf("recovered candidate = %#v, err = %v", recovered.Collaboration, err)
	}
	blocked, err := roomRuntime.SubmitTaskVerification(ctx, TaskVerificationSubmitParams{
		RoomID: room.ID, TaskID: task.ID, Decision: VerificationBlock, Report: "Replay still succeeds.",
		GoalRevision: checking.TaskGoalRevision, CandidateRevision: checking.TaskCandidateRevision,
	})
	if err != nil {
		t.Fatalf("SubmitTaskVerification(block) error = %v", err)
	}
	ownerClient, _ = service.BindAgent(ctx, owner.Agent.ID)
	if _, err := ownerClient.Check(ctx); err != nil {
		t.Fatalf("Check(owner feedback) error = %v", err)
	}
	if err := service.Close(); err != nil {
		t.Fatalf("Close(before feedback recovery) error = %v", err)
	}

	service, err = Open(dir, nil)
	if err != nil {
		t.Fatalf("Open(feedback recovery) error = %v", err)
	}
	t.Cleanup(func() { _ = service.Close() })
	ownerClient, _ = service.BindAgent(ctx, owner.Agent.ID)
	recovered, err = ownerClient.Check(ctx)
	if err != nil || len(recovered.Collaboration) != 1 || recovered.Collaboration[0].ID != blocked.Delivery.ID {
		t.Fatalf("recovered feedback = %#v, err = %v", recovered.Collaboration, err)
	}
}
