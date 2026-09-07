package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/blueberrycongee/wuu/internal/agent"
	"github.com/blueberrycongee/wuu/internal/pluginhost"
)

func TestWindowProviderKeepsHandoffWithoutNoteInference(t *testing.T) {
	client := &generationClient{id: "windows", capabilities: []pluginhost.CapabilityDescriptor{{ID: pluginhost.CapabilityAgentCompaction, Kind: "decision", Version: 3}},
		invoke: func(params pluginhost.CapabilityInvokeParams) (pluginhost.CapabilityInvokeResult, error) {
			var input pluginhost.CompactionInput
			if err := json.Unmarshal(params.Input, &input); err != nil {
				return pluginhost.CapabilityInvokeResult{}, err
			}
			if input.Operation != "handoff_brief_plan" {
				t.Fatalf("unexpected inference plan %s", input.Operation)
			}
			output, _ := json.Marshal(pluginhost.CompactionOutput{NotePrompt: "handoff", MaxNoteBytes: 24000})
			return pluginhost.CapabilityInvokeResult{Output: output}, nil
		},
	}
	_, registry, err := buildPluginAgentCapabilities(context.Background(), pluginhost.New(client), "byok", "model", "/workspace")
	if err != nil {
		t.Fatal(err)
	}
	provider := registry.Resolve(nil).(*pluginCompactionProvider)
	if !provider.ContextWindowsEnabled() || provider.CompactionNotesEnabled() {
		t.Fatal("incorrect window policy")
	}
	if _, err := provider.PlanCompactionNote(context.Background(), "model", nil, nil, agent.CompactionNote{}); !errors.Is(err, agent.ErrCompactionNoteUnsupported) {
		t.Fatalf("legacy note plan=%v", err)
	}
	if _, err := provider.PlanHandoffBrief(context.Background(), "model", nil, agent.CompactionNote{}, "continue", "source", 12); err != nil {
		t.Fatal(err)
	}
}
