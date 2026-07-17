package modelcatalog

const kimiForCodingProviderID = "kimi-for-coding"

func applyBuiltinCatalogOverrides(data *catalogData) {
	if data == nil {
		return
	}
	for i := range data.Providers {
		provider := &data.Providers[i]
		if provider.ID != kimiForCodingProviderID {
			continue
		}
		provider.API = "https://api.kimi.com/coding/"
		for j := range provider.Models {
			if provider.Models[j].ID == "k3" {
				provider.Models[j] = kimiK3Model(provider.Models[j])
				return
			}
		}
		provider.Models = append(provider.Models, kimiK3Model(Model{}))
		return
	}
}

func kimiK3Model(model Model) Model {
	attachment := false
	toolCall := true
	structuredOutput := true
	temperature := true
	model.ID = "k3"
	model.Name = "Kimi K3"
	model.Family = "kimi-k3"
	model.ReleaseDate = "2026-07-16"
	model.Reasoning = true
	model.ReasoningOptions = []map[string]any{
		{"type": "effort", "values": []any{"max"}},
	}
	model.Attachment = &attachment
	model.ToolCall = &toolCall
	model.StructuredOutput = &structuredOutput
	model.Temperature = &temperature
	model.Modalities = &Modalities{
		Input:  []string{"text", "image", "video"},
		Output: []string{"text"},
	}
	model.Limit = &Limit{Context: 1_048_576, Output: 131_072}
	model.Options = map[string]any{
		"allow_empty_signature": true,
		"thinking_replay":       "full",
	}
	model.Headers = map[string]string{"User-Agent": "KimiCLI/1.5"}
	model.SupportedEfforts = []string{"max"}
	model.DefaultVariant = "max"
	model.Variants = map[string]map[string]any{
		"max": {
			"thinking": map[string]any{
				"type":    "adaptive",
				"display": "summarized",
			},
			"effort": "max",
		},
	}
	return model
}
