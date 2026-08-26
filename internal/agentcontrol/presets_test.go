package agentcontrol

import (
	"strings"
	"testing"
)

// These tests pin the load-bearing lines of the model-facing prompt
// constants so a casual edit can't accidentally weaken them.

func TestComposeWorkerSystemPrompt_ContainsWorkerOverride(t *testing.T) {
	wt, err := LookupWorkerType(DefaultSubagentType)
	if err != nil {
		t.Fatalf("LookupWorkerType(general-purpose): %v", err)
	}
	got := composeWorkerSystemPrompt("You are wuu, a pragmatic CLI coding assistant.", wt, "/tmp/repo", IsolationInplace)
	if !strings.Contains(got, "Worker override:") {
		t.Fatalf("worker system prompt missing override marker: %q", got)
	}
	if !strings.Contains(got, "If a tool is in your tool list") {
		t.Fatalf("worker system prompt must restore access to worker tools: %q", got)
	}
	if !strings.Contains(got, "cannot spawn or manage other agents") {
		t.Fatalf("worker system prompt must match worker tool filtering: %q", got)
	}
	if strings.Contains(got, "You may spawn further sub-agents") {
		t.Fatalf("worker system prompt must not promise recursive delegation: %q", got)
	}
}
