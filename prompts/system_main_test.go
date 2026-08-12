package prompts

import (
	"strings"
	"testing"
)

func TestSystemMainDoesNotOwnOptionalProductGuidance(t *testing.T) {
	prompt := SystemMain()
	for _, productTerm := range []string{"subagent", "spawn_agent", "agent_report"} {
		if strings.Contains(strings.ToLower(prompt), productTerm) {
			t.Fatalf("SystemMain should leave %q guidance to its plugin:\n%s", productTerm, prompt)
		}
	}
}
