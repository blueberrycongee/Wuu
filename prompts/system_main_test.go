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

func TestSystemMainEncouragesEvidenceBackedIntentInference(t *testing.T) {
	prompt := SystemMain()
	for _, guidance := range []string{
		"infer the user's underlying goal",
		"resolve discoverable uncertainty yourself",
		"best-supported interpretation",
		"workspace as the source of truth for repository behavior",
		"preferably primary sources",
	} {
		if !strings.Contains(prompt, guidance) {
			t.Fatalf("SystemMain missing %q guidance:\n%s", guidance, prompt)
		}
	}
}
