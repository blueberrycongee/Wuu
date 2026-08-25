package channels

import (
	"context"
	"errors"
	"testing"
)

func TestDurableWorkTracksDebtRunsArtifactsAndVerification(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	service, err := Open(dir, nil)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	owner, _ := service.CreateNamedAgent(ctx, CreateNamedAgentParams{Name: "Owner"})
	room, _ := service.CreateRoom(ctx, CreateRoomParams{
		Kind: RoomChannel, Name: "Work", CreatedBy: "local-user",
		Members: []RoomMember{{MemberType: MemberAgent, MemberID: owner.Agent.ID}},
	})
	source, err := service.SendHuman(ctx, HumanSendParams{RoomID: room.ID, HumanID: "local-user", Body: "Fix the callback"})
	if err != nil {
		t.Fatalf("SendHuman() error = %v", err)
	}
	runtime, _ := service.BindRuntime(ctx, room.RuntimeID)
	ownerClient, _ := service.BindAgent(ctx, owner.Agent.ID)
	task, err := runtime.CreateTask(ctx, TaskCreateParams{
		RoomID: room.ID, SourceMessageID: source.Message.ID, Title: "Fix callback",
		Body: "Reject replayed state", OwnerID: owner.Agent.ID, VerificationRequired: true,
	})
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	work, err := ownerClient.GetWork(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetWork() error = %v", err)
	}
	if work.SourceMessageID != source.Message.ID || work.OwnerNamedAgentID != owner.Agent.ID || work.State != WorkOpen || work.VerificationState != WorkVerificationPending || len(work.PendingDeliveryRefs) != 1 {
		t.Fatalf("initial work = %#v", work)
	}
	check, err := ownerClient.Check(ctx)
	if err != nil || len(check.Collaboration) != 1 || check.Collaboration[0].Kind != CollaborationAssignment || check.Collaboration[0].WorkID != task.ID {
		t.Fatalf("assignment delivery = %#v, err = %v", check.Collaboration, err)
	}
	if check.Collaboration[0].FromID != "" || check.Collaboration[0].RecipientNamedAgentID != owner.Agent.ID {
		t.Fatalf("assignment leaked hidden sender or lost recipient: %#v", check.Collaboration[0])
	}
	if _, err := ownerClient.UpdateTask(ctx, TaskUpdateParams{TaskID: task.ID, State: TaskStateDoing}); err != nil {
		t.Fatalf("start task: %v", err)
	}
	checking, err := ownerClient.UpdateTask(ctx, TaskUpdateParams{TaskID: task.ID, State: TaskStateChecking})
	if err != nil {
		t.Fatalf("check task: %v", err)
	}
	artifact, err := ownerClient.AddWorkArtifact(ctx, WorkArtifactAddParams{
		WorkID: task.ID, Kind: WorkArtifactDiff, URI: "artifact://diff-1",
		Summary: "2 files changed", WorkspaceRevision: "git:abc",
	})
	if err != nil {
		t.Fatalf("AddWorkArtifact() error = %v", err)
	}
	run, err := runtime.StartWorkRun(ctx, WorkRunStartParams{
		WorkID: task.ID, Kind: WorkRunVerifier, Profile: "coding",
		SessionRef: "session-verifier-1", WorkspaceRevision: "git:abc",
	})
	if err != nil {
		t.Fatalf("StartWorkRun() error = %v", err)
	}
	if err := service.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	service, err = Open(dir, nil)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer service.Close()
	runtime, _ = service.BindRuntime(ctx, room.RuntimeID)
	ownerClient, _ = service.BindAgent(ctx, owner.Agent.ID)
	restored, err := ownerClient.GetWork(ctx, task.ID)
	if err != nil || restored.CurrentRunRef != run.ID || len(restored.Runs) != 1 || len(restored.Artifacts) != 1 || restored.CandidateArtifactRef != artifact.ID {
		t.Fatalf("restored work = %#v, err = %v", restored, err)
	}
	if _, err := runtime.FinishWorkRun(ctx, WorkRunFinishParams{
		WorkID: task.ID, RunID: run.ID, State: WorkRunCompleted, Outcome: "pass",
		Provider: "openai", Model: "reviewer", InputTokens: 100, OutputTokens: 20, ChecksRerun: 1,
	}); err != nil {
		t.Fatalf("FinishWorkRun() error = %v", err)
	}
	result, err := runtime.SubmitTaskVerification(ctx, TaskVerificationSubmitParams{
		TaskID: task.ID, RoomID: room.ID, GoalRevision: checking.TaskGoalRevision,
		CandidateRevision: checking.TaskCandidateRevision, Decision: VerificationPass,
		Report: "Focused replay check passed.", EvidenceRefs: []string{artifact.ID}, RunRef: run.ID,
	})
	if err != nil {
		t.Fatalf("SubmitTaskVerification() error = %v", err)
	}
	if result.Delivery.WorkID != task.ID || len(result.Delivery.ArtifactRefs) != 1 || result.Delivery.FromID != "" {
		t.Fatalf("verification delivery = %#v", result.Delivery)
	}
	verified, err := ownerClient.GetWork(ctx, task.ID)
	if err != nil || verified.VerificationState != WorkVerificationPass || verified.Verification == nil || verified.Verification.RunRef != run.ID || len(verified.Verification.EvidenceRefs) != 1 {
		t.Fatalf("verified work = %#v, err = %v", verified, err)
	}
}

func TestGoalRevisionInvalidatesPendingDeliveriesAndRunningHandles(t *testing.T) {
	ctx := context.Background()
	service, err := Open(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer service.Close()
	owner, _ := service.CreateNamedAgent(ctx, CreateNamedAgentParams{Name: "Owner"})
	room, _ := service.CreateRoom(ctx, CreateRoomParams{Kind: RoomChannel, Name: "Work", CreatedBy: "local-user", Members: []RoomMember{{MemberType: MemberAgent, MemberID: owner.Agent.ID}}})
	runtime, _ := service.BindRuntime(ctx, room.RuntimeID)
	ownerClient, _ := service.BindAgent(ctx, owner.Agent.ID)
	task, _ := runtime.CreateTask(ctx, TaskCreateParams{RoomID: room.ID, Title: "Fix", OwnerID: owner.Agent.ID, VerificationRequired: true})
	_, _ = ownerClient.Check(ctx)
	_, _ = ownerClient.UpdateTask(ctx, TaskUpdateParams{TaskID: task.ID, State: TaskStateDoing})
	checking, _ := ownerClient.UpdateTask(ctx, TaskUpdateParams{TaskID: task.ID, State: TaskStateChecking})
	run, err := runtime.StartWorkRun(ctx, WorkRunStartParams{WorkID: task.ID, Kind: WorkRunVerifier, SessionRef: "session-stale"})
	if err != nil {
		t.Fatalf("StartWorkRun() error = %v", err)
	}
	revised, err := runtime.UpdateTask(ctx, TaskUpdateParams{TaskID: task.ID, GoalCorrection: "Fix and preserve audit events"})
	if err != nil {
		t.Fatalf("revise task: %v", err)
	}
	if revised.TaskGoalRevision != checking.TaskGoalRevision+1 {
		t.Fatalf("goal revision = %d", revised.TaskGoalRevision)
	}
	work, err := runtime.GetWork(ctx, task.ID)
	if err != nil || work.CurrentRunRef != "" || work.State != WorkOpen || work.VerificationState != WorkVerificationPending {
		t.Fatalf("revised work = %#v, err = %v", work, err)
	}
	if len(work.Runs) != 1 || work.Runs[0].ID != run.ID || work.Runs[0].State != WorkRunInterrupted {
		t.Fatalf("stale run not interrupted: %#v", work.Runs)
	}
	if _, err := runtime.FinishWorkRun(ctx, WorkRunFinishParams{WorkID: task.ID, RunID: run.ID, State: WorkRunCompleted}); !errors.Is(err, ErrConflict) {
		t.Fatalf("FinishWorkRun(stale) error = %v, want conflict", err)
	}
}
