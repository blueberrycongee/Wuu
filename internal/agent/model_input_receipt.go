package agent

import (
	"context"
	"time"

	"github.com/blueberrycongee/wuu/internal/providers"
)

const ModelInputReceiptContractVersion = 1

// ModelInputReceipt is the provider-neutral, durable reconstruction record for
// one final request shape after context assembly and plugin transforms.
type ModelInputReceipt struct {
	ContractVersion  int                        `json:"contract_version"`
	OperationID      string                     `json:"operation_id"`
	SessionID        string                     `json:"session_id,omitempty"`
	ExecutionID      string                     `json:"execution_id,omitempty"`
	DriverID         string                     `json:"driver_id"`
	DriverVersion    string                     `json:"driver_version"`
	Provider         string                     `json:"provider,omitempty"`
	Model            string                     `json:"model"`
	StepIndex        int                        `json:"step_index"`
	InputFactSeqs    []int                      `json:"input_fact_seqs,omitempty"`
	Messages         []providers.ChatMessage    `json:"messages"`
	Tools            []ModelInputToolDefinition `json:"tools,omitempty"`
	ToolSurfaceHash  string                     `json:"tool_surface_hash,omitempty"`
	SystemSections   []ModelInputSystemSection  `json:"system_sections,omitempty"`
	PromptCacheKey   string                     `json:"prompt_cache_key,omitempty"`
	ForceToolName    string                     `json:"force_tool_name,omitempty"`
	Temperature      float64                    `json:"temperature,omitempty"`
	Effort           string                     `json:"effort,omitempty"`
	HistoryRewritten bool                       `json:"history_rewritten,omitempty"`
	CreatedAt        time.Time                  `json:"created_at"`
}

type ModelInputToolDefinition struct {
	Name         string         `json:"name"`
	Description  string         `json:"description,omitempty"`
	InputSchema  map[string]any `json:"input_schema"`
	DeferLoading bool           `json:"defer_loading,omitempty"`
	CacheStable  bool           `json:"cache_stable,omitempty"`
}

type ModelInputSystemSection struct {
	Key    string `json:"key"`
	Static bool   `json:"static"`
	Bytes  int    `json:"bytes"`
	Hash   string `json:"hash"`
}

type ModelInputReceiptStore interface {
	SaveModelInputReceipt(context.Context, ModelInputReceipt) error
}

func durableInputFactSeqs(messages []providers.ChatMessage) []int {
	seen := make(map[int]struct{})
	seqs := make([]int, 0)
	for _, message := range messages {
		if message.Seq < 1 {
			continue
		}
		if _, ok := seen[message.Seq]; ok {
			continue
		}
		seen[message.Seq] = struct{}{}
		seqs = append(seqs, message.Seq)
	}
	return seqs
}

func modelInputTools(definitions []providers.ToolDefinition) []ModelInputToolDefinition {
	tools := make([]ModelInputToolDefinition, 0, len(definitions))
	for _, definition := range definitions {
		tools = append(tools, ModelInputToolDefinition{
			Name:         definition.Name,
			Description:  definition.Description,
			InputSchema:  definition.InputSchema,
			DeferLoading: definition.DeferLoading,
			CacheStable:  definition.CacheStable,
		})
	}
	return tools
}

func modelInputSystemSections(sections []SystemPromptSectionInfo) []ModelInputSystemSection {
	out := make([]ModelInputSystemSection, 0, len(sections))
	for _, section := range sections {
		out = append(out, ModelInputSystemSection{
			Key:    section.Key,
			Static: section.Static,
			Bytes:  section.Bytes,
			Hash:   section.Hash,
		})
	}
	return out
}
