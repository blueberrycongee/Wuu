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

func TestRoomAgentOrientationDefinesSingleEntrypointAndVisibleDelegation(t *testing.T) {
	prompt := namedAgentOrientation(channels.NamedAgent{
		Kind: "room", RoomID: "room-1", MemoryDir: "/agents/room-1/memory",
	})
	for _, want := range []string{
		"You are the room's single collaboration entrypoint",
		"Ordinary room messages and member reports wake you; they do not wake every member",
		"calling chat_task create once per member at the room root",
		"persisted task messages are the room's public assignment facts",
		"Members publish meaningful progress, handoffs, questions, and results in each task's public thread",
		"final synthesis",
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
		"Ordinary shared-room messages, including @mentions, are routed first to the room agent and do not directly wake every member",
		"Do not independently claim work from the shared transcript",
		"Act on room tasks assigned to you",
		"mark it doing when you start",
		"post only meaningful progress, questions, handoffs, and the result in that task's public thread",
		"Use the task message as thread_id and reply_to",
		"A collaboration message is private control traffic",
		"Direct messages remain private conversations with the human",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("named-agent orientation missing room routing rule %q:\n%s", want, prompt)
		}
	}
}
