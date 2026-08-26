package appserver

import (
	"strings"
	"testing"
	"time"

	"github.com/blueberrycongee/wuu/internal/channels"
)

func TestNamedAgentOrientationDistinguishesHomeFromProjectScope(t *testing.T) {
	agent := channels.NamedAgent{
		ID:        "agent-1",
		Name:      "Andy",
		MemoryDir: "/agents/agent-1/memory",
		CreatedAt: time.Now(),
	}
	prompt := namedAgentOrientation(agent)
	for _, want := range []string{
		"agent home is your private identity and state anchor; it is not the limit",
		"supplied as request-only environment context",
		"Use an absolute file path or set a command's cwd",
		"Do not claim that you can only access your agent home",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("orientation missing %q:\n%s", want, prompt)
		}
	}
}

func TestNamedAgentOrientationExcludesProjectlessConversations(t *testing.T) {
	prompt := namedAgentOrientation(channels.NamedAgent{
		Name: "Andy", MemoryDir: "/agents/agent-1/memory",
	})
	if !strings.Contains(prompt, "Projectless conversation sessions are not project workspaces") {
		t.Fatalf("orientation should explain empty project scope:\n%s", prompt)
	}
}

func TestRoomAgentOrientationDefinesNamedVerifierAndConversationRouting(t *testing.T) {
	prompt := agentRuntimeOrientation(channels.AgentRuntime{
		Kind: channels.PrincipalRoomRuntime, RoomID: "room-1", MemoryDir: "/runtimes/room-1/memory",
	})
	for _, want := range []string{
		"You are the hidden runtime",
		"Never use chat_send",
		"You are the room's single collaboration entrypoint",
		"Ordinary room messages and member reports wake you; they do not wake every member",
		"chat_task create at the room root",
		"Set verification_required=true for every substantive coding task",
		"call chat_task revise",
		"work_id, goal_revision, candidate_revision, and artifact_refs identify the exact candidate",
		"give every current visible member a real opportunity",
		"Use parallel private control messages",
		"Use serial invitations",
		"Select exactly one current Named Agent other than the owner as verifier",
		"profile set to that verifier's member_id",
		"kind=peer_result",
		"first-line PASS, BLOCK, or UNKNOWN",
		"call chat_verify exactly once",
		"Do not create hidden workers or unnamed selectors",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("room orientation missing %q:\n%s", want, prompt)
		}
	}
}

func TestNamedAgentOrientationRoutesSharedRoomWorkThroughRoomAgent(t *testing.T) {
	prompt := namedAgentOrientation(channels.NamedAgent{
		Name: "Andy", MemoryDir: "/agents/agent-1/memory",
	})
	for _, want := range []string{
		"Ordinary shared-room messages, including @mentions, are routed first to the hidden room runtime and do not directly wake every member",
		"Do not independently claim work from the shared transcript",
		"Act on room tasks assigned to you",
		"mark a task doing when you start",
		"do not publish the candidate as final",
		"kind=candidate_ready, source_message_id=task_id, and artifact_refs",
		"returned goal_revision and candidate_revision",
		"kind verification_feedback",
		"on BLOCK",
		"On PASS",
		"Use the task message as thread_id and reply_to",
		"Direct messages remain private conversations with the human",
		"control message asks for a public conversational contribution",
		"kind=peer_result",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("named-agent orientation missing room routing rule %q:\n%s", want, prompt)
		}
	}
}
