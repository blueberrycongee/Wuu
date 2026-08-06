package runtime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/blueberrycongee/wuu/internal/pluginhost"
	"github.com/blueberrycongee/wuu/internal/providers"
)

func TestPluginCompactionRejectsInvalidToolCallHistory(t *testing.T) {
	client := &generationClient{
		id: "invalid-compactor",
		capabilities: []pluginhost.CapabilityDescriptor{{
			ID: pluginhost.CapabilityAgentCompaction, Kind: "decision", Version: 1,
		}},
		invoke: func(pluginhost.CapabilityInvokeParams) (pluginhost.CapabilityInvokeResult, error) {
			output, err := json.Marshal(pluginhost.CompactionOutput{Messages: []providers.ChatMessage{{
				Role: "tool", ToolCallID: "orphan", Content: "unpaired result",
			}}})
			return pluginhost.CapabilityInvokeResult{Output: output}, err
		},
	}
	host := pluginhost.New(client)
	_, compactions, err := buildPluginAgentCapabilities(context.Background(), host, "openai", "model", "/workspace")
	if err != nil {
		t.Fatal(err)
	}

	_, err = compactions.Resolve(nil).Compact(context.Background(), "model", []providers.ChatMessage{{Role: "user", Content: "history"}})
	if err == nil || !strings.Contains(err.Error(), "invalid tool-call history") {
		t.Fatalf("compaction error = %v", err)
	}
}
