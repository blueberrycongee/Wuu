package plugin

import (
	"fmt"
	"sort"
	"strings"
)

type ActivationIssueKind string

const (
	ActivationIssueMissingRequirement ActivationIssueKind = "missing_requirement"
	ActivationIssueConflict           ActivationIssueKind = "conflict"
)

type ActivationIssue struct {
	Kind            ActivationIssueKind `json:"kind"`
	RelatedPluginID string              `json:"related_plugin_id"`
}

type ActivationPlan struct {
	Plugins []Plugin
	Issues  map[string][]ActivationIssue
}

// BuildActivationPlan applies the manifest's simple package relationships.
// Hard incompatibilities and dependency cycles reject the generation. Missing
// requirements leave only the dependent package inactive. Soft conflicts are
// reported without changing activation.
func BuildActivationPlan(candidates []Plugin) (ActivationPlan, error) {
	byID := make(map[string]Plugin, len(candidates))
	ids := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		id := strings.TrimSpace(candidate.ID)
		if id == "" {
			continue
		}
		if _, exists := byID[id]; exists {
			return ActivationPlan{}, fmt.Errorf("plugin activation contains duplicate id %q", id)
		}
		byID[id] = candidate
		ids = append(ids, id)
	}
	sort.Strings(ids)

	available := make(map[string]bool, len(byID))
	for _, id := range ids {
		available[id] = true
	}
	issues := make(map[string][]ActivationIssue)
	for changed := true; changed; {
		changed = false
		for _, id := range ids {
			if !available[id] {
				continue
			}
			for _, requiredID := range byID[id].Requires {
				if available[requiredID] {
					continue
				}
				available[id] = false
				issues[id] = append(issues[id], ActivationIssue{
					Kind:            ActivationIssueMissingRequirement,
					RelatedPluginID: requiredID,
				})
				changed = true
				break
			}
		}
	}
	for _, id := range ids {
		if !available[id] {
			continue
		}
		for _, brokenID := range byID[id].Breaks {
			if available[brokenID] {
				return ActivationPlan{}, fmt.Errorf("plugin %q breaks plugin %q; disable one before activation", id, brokenID)
			}
		}
	}

	for _, id := range ids {
		if !available[id] {
			continue
		}
		for _, conflictID := range byID[id].Conflicts {
			if available[conflictID] {
				issues[id] = append(issues[id], ActivationIssue{
					Kind:            ActivationIssueConflict,
					RelatedPluginID: conflictID,
				})
			}
		}
	}

	ordered, err := topologicalActivationOrder(byID, available, ids)
	if err != nil {
		return ActivationPlan{}, err
	}
	return ActivationPlan{Plugins: ordered, Issues: issues}, nil
}

func topologicalActivationOrder(byID map[string]Plugin, available map[string]bool, ids []string) ([]Plugin, error) {
	remaining := make(map[string]int)
	dependents := make(map[string][]string)
	for _, id := range ids {
		if !available[id] {
			continue
		}
		remaining[id] = 0
		for _, requiredID := range byID[id].Requires {
			if !available[requiredID] {
				continue
			}
			remaining[id]++
			dependents[requiredID] = append(dependents[requiredID], id)
		}
	}
	ready := make([]string, 0, len(remaining))
	for _, id := range ids {
		if available[id] && remaining[id] == 0 {
			ready = append(ready, id)
		}
	}
	ordered := make([]Plugin, 0, len(remaining))
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		ordered = append(ordered, byID[id])
		for _, dependentID := range dependents[id] {
			remaining[dependentID]--
			if remaining[dependentID] == 0 {
				ready = append(ready, dependentID)
				sort.Strings(ready)
			}
		}
	}
	if len(ordered) != len(remaining) {
		cycle := make([]string, 0)
		for _, id := range ids {
			if available[id] && remaining[id] > 0 {
				cycle = append(cycle, id)
			}
		}
		return nil, fmt.Errorf("plugin requires cycle: %s", strings.Join(cycle, ", "))
	}
	return ordered, nil
}
