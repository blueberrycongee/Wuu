package modelcatalog

import (
	"strings"

	"github.com/blueberrycongee/wuu/internal/config"
)

const kimiForCodingProviderID = "kimi-for-coding"

// applyProviderCompatibilityDefaults adds request-transport defaults that
// models.dev does not describe. These are deliberately kept separate from
// catalog facts such as modalities, limits, and reasoning efforts. Explicit
// user headers and model options always win over these defaults.
func applyProviderCompatibilityDefaults(providerID string, provider config.ProviderConfig, modelIDs ...string) config.ProviderConfig {
	switch providerID {
	case kimiForCodingProviderID:
		provider.Headers = mergeHeaders(
			map[string]string{"User-Agent": "KimiCLI/1.5"},
			provider.Headers,
		)
		provider = applyModelCompatibilityDefaults(provider, map[string]any{
			"force_adaptive_thinking": true,
			"anthropic_default_betas": false,
		})
		if model, ok := provider.Models["k3"]; ok {
			model.Options = mergeModelOptions(map[string]any{
				"allow_empty_signature": true,
				"thinking_replay":       "full",
			}, model.Options)
			provider.Models["k3"] = model
		}
	case "minimax", "minimax-cn", "minimax-coding-plan", "minimax-cn-coding-plan":
		provider = applyModelCompatibilityDefaults(provider, map[string]any{
			"anthropic_default_betas": false,
		})
	case "deepseek":
		if isAnthropicCompatibleProvider(provider) {
			provider.NPM = "@ai-sdk/anthropic"
			if replaceKnownEndpoint(provider.BaseURL, "https://api.deepseek.com", "https://api.deepseek.com/anthropic") {
				provider.API = "https://api.deepseek.com/anthropic"
				provider.BaseURL = provider.API
			}
		} else if providerUsesModel(provider, "deepseek-v4-pro", modelIDs...) && strings.TrimSpace(provider.WireAPI) == "" {
			provider.WireAPI = "responses"
		}
	case "zai-coding-plan":
		if isAnthropicCompatibleProvider(provider) {
			provider.NPM = "@ai-sdk/anthropic"
			if replaceKnownEndpoint(provider.BaseURL, "https://api.z.ai/api/coding/paas/v4", "https://api.z.ai/api/anthropic") {
				provider.API = "https://api.z.ai/api/anthropic"
				provider.BaseURL = provider.API
			}
		} else if strings.EqualFold(strings.TrimSpace(provider.WireAPI), "responses") &&
			replaceKnownEndpoint(provider.BaseURL, "https://api.z.ai/api/coding/paas/v4", "https://api.z.ai/api/v1") {
			provider.API = "https://api.z.ai/api/v1"
			provider.BaseURL = provider.API
		}
	case "xai":
		if strings.TrimSpace(provider.BaseURL) == "" {
			provider.BaseURL = "https://api.x.ai/v1"
		}
		if providerUsesModel(provider, "grok-4.6", modelIDs...) && strings.TrimSpace(provider.WireAPI) == "" {
			provider.WireAPI = "responses"
		}
	}
	return provider
}

func isAnthropicCompatibleProvider(provider config.ProviderConfig) bool {
	switch normalizeID(provider.Type) {
	case "anthropic", "anthropic-official", "claude":
		return true
	default:
		return false
	}
}

func replaceKnownEndpoint(current string, known ...string) bool {
	current = strings.TrimRight(strings.ToLower(strings.TrimSpace(current)), "/")
	if current == "" {
		return true
	}
	for _, endpoint := range known {
		if current == strings.TrimRight(strings.ToLower(strings.TrimSpace(endpoint)), "/") {
			return true
		}
	}
	return false
}

func providerUsesModel(provider config.ProviderConfig, target string, modelIDs ...string) bool {
	target = strings.TrimSpace(target)
	for _, modelID := range modelIDs {
		if strings.EqualFold(strings.TrimSpace(modelID), target) {
			return true
		}
	}
	if len(modelIDs) == 0 && strings.EqualFold(strings.TrimSpace(provider.Model), target) {
		return true
	}
	return false
}

func applyModelCompatibilityDefaults(provider config.ProviderConfig, defaults map[string]any) config.ProviderConfig {
	for id, model := range provider.Models {
		model.Options = mergeModelOptions(defaults, model.Options)
		provider.Models[id] = model
	}
	return provider
}

// applyOfficialCatalogCorrections carries model facts that have been published
// by the vendors but have not yet reached every models.dev snapshot. Keep these
// entries narrow and remove them after the upstream catalog contains equivalent
// or newer metadata.
func applyOfficialCatalogCorrections(data *catalogData) {
	if data == nil {
		return
	}
	for i := range data.Providers {
		provider := &data.Providers[i]
		switch normalizeID(provider.ID) {
		case "deepseek":
			upsertOfficialModel(provider, Model{
				ID:               "deepseek-v4-pro",
				Name:             "DeepSeek V4 Pro",
				Family:           "deepseek-thinking",
				Reasoning:        true,
				ReasoningOptions: officialEffortOptions(true, "low", "high", "max"),
				Attachment:       officialBool(false),
				ToolCall:         officialBool(true),
				StructuredOutput: officialBool(true),
				Temperature:      officialBool(true),
				Interleaved:      map[string]any{"field": "reasoning_content"},
				Modalities:       &Modalities{Input: []string{"text"}, Output: []string{"text"}},
				Limit:            &Limit{Context: 1_000_000, Output: 384_000},
				SupportedEfforts: []string{"none", "low", "high", "max"},
				DefaultVariant:   "high",
			})
		case "zai", "zai-coding-plan":
			upsertOfficialModel(provider, Model{
				ID:               "glm-5.3",
				Name:             "GLM-5.3",
				Family:           "glm",
				Reasoning:        true,
				ReasoningOptions: officialEffortOptions(false, "low", "high", "max"),
				Attachment:       officialBool(false),
				ToolCall:         officialBool(true),
				StructuredOutput: officialBool(true),
				Temperature:      officialBool(true),
				Interleaved:      map[string]any{"field": "reasoning_content"},
				Modalities:       &Modalities{Input: []string{"text"}, Output: []string{"text"}},
				Limit:            &Limit{Context: 1_000_000, Output: 131_072},
				SupportedEfforts: []string{"low", "high", "max"},
				DefaultVariant:   "max",
			})
		case "xai":
			provider.API = "https://api.x.ai/v1"
			upsertOfficialModel(provider, Model{
				ID:               "grok-4.5",
				Name:             "Grok 4.5",
				Family:           "grok",
				Reasoning:        true,
				ReasoningOptions: officialEffortOptions(false, "low", "medium", "high"),
				Attachment:       officialBool(true),
				ToolCall:         officialBool(true),
				StructuredOutput: officialBool(true),
				Temperature:      officialBool(true),
				Modalities:       &Modalities{Input: []string{"text", "image"}, Output: []string{"text"}},
				Limit:            &Limit{Context: 500_000},
				SupportedEfforts: []string{"low", "medium", "high"},
				DefaultVariant:   "high",
			})
			upsertOfficialModel(provider, Model{
				ID:               "grok-4.6",
				Name:             "Grok 4.6 (Early Access)",
				Family:           "grok",
				Reasoning:        true,
				ReasoningOptions: officialEffortOptions(false, "low", "medium", "high", "xhigh"),
				Attachment:       officialBool(true),
				ToolCall:         officialBool(true),
				StructuredOutput: officialBool(true),
				Temperature:      officialBool(true),
				Modalities:       &Modalities{Input: []string{"text", "image"}, Output: []string{"text"}},
				Limit:            &Limit{Context: 500_000},
				SupportedEfforts: []string{"low", "medium", "high", "xhigh"},
				DefaultVariant:   "high",
			})
		}
	}
}

func upsertOfficialModel(provider *Provider, correction Model) {
	if provider == nil {
		return
	}
	for i := range provider.Models {
		if !strings.EqualFold(strings.TrimSpace(provider.Models[i].ID), correction.ID) {
			continue
		}
		existing := provider.Models[i]
		// Pricing and transport-specific overrides can remain fresher upstream;
		// the fields above are the official capability correction.
		correction.Cost = existing.Cost
		correction.Provider = existing.Provider
		correction.Options = existing.Options
		correction.Headers = existing.Headers
		if correction.Status == "" {
			correction.Status = existing.Status
		}
		if correction.ReleaseDate == "" {
			correction.ReleaseDate = existing.ReleaseDate
		}
		provider.Models[i] = correction
		return
	}
	provider.Models = append(provider.Models, correction)
}

func officialEffortOptions(toggle bool, efforts ...string) []map[string]any {
	options := make([]map[string]any, 0, 2)
	if toggle {
		options = append(options, map[string]any{"type": "toggle"})
	}
	options = append(options, map[string]any{"type": "effort", "values": append([]string(nil), efforts...)})
	return options
}

func officialBool(value bool) *bool {
	return &value
}
