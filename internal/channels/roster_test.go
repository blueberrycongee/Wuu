package channels

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRoomRuntimeCanInviteAndProposePersistentNamedAgents(t *testing.T) {
	ctx := context.Background()
	wake := &recordingWakeSink{}
	service := openTestService(t, wake)
	andy := createTestAgent(t, service, "Andy")
	reviewer := createTestAgent(t, service, "Reviewer")
	if err := os.WriteFile(filepath.Join(andy.Agent.MemoryDir, agentMemoryIndexFile), []byte("- [Sessions](sessions.md) - Recovers interrupted work safely\n"), 0o600); err != nil {
		t.Fatalf("write member memory index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(reviewer.Agent.MemoryDir, agentMemoryIndexFile), []byte("- [Private](private.md) - Outside-room memory\n"), 0o600); err != nil {
		t.Fatalf("write available agent memory index: %v", err)
	}
	room := createTestRoom(t, service, andy)
	client, err := service.BindRuntime(ctx, room.RuntimeID)
	if err != nil {
		t.Fatalf("BindRuntime() error = %v", err)
	}
	bound, err := client.BindCollaborationSession(ctx, CollaborationSessionBindParams{
		SessionRef: "andy-room-session", PrincipalID: andy.Agent.ID, RoomID: room.ID,
		Purpose: CollaborationSessionCoordination, State: CollaborationSessionRunning,
	})
	if err != nil {
		t.Fatalf("BindCollaborationSession() error = %v", err)
	}

	roster, err := client.RoomRoster(ctx, room.ID)
	if err != nil {
		t.Fatalf("RoomRoster() error = %v", err)
	}
	if roster.MembershipRevision != 1 || len(roster.Members) != 1 || len(roster.AvailableAgents) != 1 || roster.AvailableAgents[0].Name != "Reviewer" {
		t.Fatalf("initial roster = %#v", roster)
	}
	if len(roster.Members[0].Sessions) != 1 || roster.Members[0].Sessions[0].SessionRef != bound.SessionRef ||
		roster.Members[0].Sessions[0].Purpose != CollaborationSessionCoordination ||
		roster.Members[0].Sessions[0].State != CollaborationSessionRunning {
		t.Fatalf("member sessions = %#v", roster.Members[0].Sessions)
	}
	if roster.Members[0].MemoryIndex != "- [Sessions](sessions.md) - Recovers interrupted work safely" {
		t.Fatalf("member memory index = %q", roster.Members[0].MemoryIndex)
	}
	if roster.AvailableAgents[0].MemoryIndex != "" {
		t.Fatalf("available agent memory index = %q, want hidden", roster.AvailableAgents[0].MemoryIndex)
	}
	if roster.AvailableAgents[0].Sessions == nil || len(roster.AvailableAgents[0].Sessions) != 0 {
		t.Fatalf("available agent sessions = %#v, want an empty current-room inventory", roster.AvailableAgents[0].Sessions)
	}

	invited, err := client.InviteRoomAgent(ctx, room.ID, reviewer.Agent.ID)
	if err != nil {
		t.Fatalf("InviteRoomAgent() error = %v", err)
	}
	if invited.MembershipRevision != 2 || len(invited.Members) != 3 {
		t.Fatalf("invited room = %#v", invited)
	}

	proposal, err := client.ProposeRoomAgent(ctx, room.ID, "Designer", "界面设计与交互评审")
	if err != nil {
		t.Fatalf("ProposeRoomAgent() error = %v", err)
	}
	if proposal.Name != "Designer" || proposal.Role != "界面设计与交互评审" || proposal.State != AgentCreationPending {
		t.Fatalf("proposal = %#v", proposal)
	}
	unchanged, err := service.GetRoom(ctx, room.ID)
	if err != nil || unchanged.MembershipRevision != 2 || len(unchanged.Members) != 3 {
		t.Fatalf("proposal changed membership: room = %#v, err = %v", unchanged, err)
	}
	messages, err := service.ListMessages(ctx, room.ID, 0, 100)
	if err != nil || len(messages) != 2 || messages[1].AgentCreationProposal == nil {
		t.Fatalf("proposal message = %#v, err = %v", messages, err)
	}

	approved, err := service.ResolveAgentCreationProposal(ctx, ResolveAgentCreationProposalParams{
		ProposalID: proposal.ID, HumanID: "human-1", Approve: true, Provider: "openai", Model: "gpt-5",
	})
	if err != nil {
		t.Fatalf("ResolveAgentCreationProposal() error = %v", err)
	}
	if approved.State != AgentCreationApproved || approved.CreatedAgentID == "" || approved.Provider != "openai" || approved.Model != "gpt-5" {
		t.Fatalf("approved proposal = %#v", approved)
	}
	persisted, err := service.GetNamedAgent(ctx, approved.CreatedAgentID)
	if err != nil || !persisted.Autostart || persisted.ProviderOverride != "openai" || persisted.ModelOverride != "gpt-5" {
		t.Fatalf("approved named agent = %#v, err = %v", persisted, err)
	}

	cancelProposal, err := client.ProposeRoomAgent(ctx, room.ID, "Writer", "Documentation")
	if err != nil {
		t.Fatalf("ProposeRoomAgent(cancel) error = %v", err)
	}
	cancelled, err := service.ResolveAgentCreationProposal(ctx, ResolveAgentCreationProposalParams{
		ProposalID: cancelProposal.ID, HumanID: "human-1", Approve: false,
	})
	if err != nil || cancelled.State != AgentCreationCancelled {
		t.Fatalf("cancelled proposal = %#v, err = %v", cancelled, err)
	}
	messages, err = service.ListMessages(ctx, room.ID, 0, 100)
	if err != nil || messages[len(messages)-1].Body != "用户取消了创建新角色：Writer" {
		t.Fatalf("cancel notification = %#v, err = %v", messages, err)
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
