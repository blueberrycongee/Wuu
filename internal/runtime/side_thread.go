package runtime

import (
	"errors"
	"fmt"
	"strings"

	"github.com/blueberrycongee/wuu/internal/agent"
	"github.com/blueberrycongee/wuu/internal/modelcatalog"
	"github.com/blueberrycongee/wuu/internal/modelvariant"
	"github.com/blueberrycongee/wuu/internal/providerfactory"
	"github.com/blueberrycongee/wuu/internal/providers"
)

const sideThreadSystemPrompt = `You are a read-only side conversation attached to a coding task. Answer the user's questions from the supplied snapshot of the main task and the side-conversation history.

The snapshot may have been captured while the main task was still running. Describe only what the snapshot proves, distinguish completed work from in-progress work, and say when the current outcome is not yet known. Do not claim to have inspected state newer than the snapshot.

You have no tools and must not modify files, run commands, start agents, or steer the main task. Keep the answer direct and useful.`

// NewSideThreadRunner clones the attached conversation's model configuration
// into an isolated, tool-free runner. selected is the main conversation's
// pinned model selection, so a side chat answers with the same model the
// conversation is locked to rather than the workspace default (which drifts
// whenever another session switches model). It shares no mutable callbacks,
// tool ledger, request hooks, or usage tracker with the main thread.
func (s *Session) NewSideThreadRunner(sideThreadID string, selected ThreadModelSelection) (*agent.StreamRunner, error) {
	if s == nil || s.StreamRunner == nil {
		return nil, errors.New("stream runner is required")
	}
	id := strings.TrimSpace(sideThreadID)
	if id == "" {
		return nil, errors.New("side thread id is required")
	}

	runner, err := s.newSideThreadBaseRunner(selected)
	if err != nil {
		return nil, err
	}
	runner.Tools = nil
	runner.ToolLedger = nil
	runner.SystemPrompt = sideThreadSystemPrompt
	runner.SystemPromptSections = nil
	runner.MaxSteps = 1
	runner.OnEvent = nil
	runner.OnUsage = nil
	runner.OnTokenUsage = nil
	runner.StreamingToolExecution = false
	runner.BeforeStep = nil
	runner.BeforeRequestContext = nil
	runner.BeforeRequest = nil
	runner.OnRequestContext = nil
	runner.OnCompactAttempt = nil
	runner.OnToolBatchRejected = nil
	runner.AfterTurn = nil
	runner.ForceToolFirstStep = ""
	runner.ForceInitialCompact = false
	runner.CompactOnly = false
	runner.PromptCacheKey = "side-thread:" + id
	runner.InferenceJournal = s.InferenceJournalForOwner("side-thread:" + id)
	return runner, nil
}

// newSideThreadBaseRunner clones the workspace stream runner and repins it to
// the main conversation's model selection. An empty selection, a selection that
// already equals the workspace defaults, or a pinned provider that is no longer
// configured all fall back to the plain workspace clone. This mirrors the model
// pinning in NewThreadRuntimeForRootModel, minus the toolkit/worker/permission
// wiring a read-only side chat has no use for.
func (s *Session) newSideThreadBaseRunner(selected ThreadModelSelection) (*agent.StreamRunner, error) {
	providerName := strings.TrimSpace(selected.Provider)
	model := strings.TrimSpace(selected.Model)
	currentVariant := strings.TrimSpace(s.StreamRunner.Variant)
	currentEffort := strings.TrimSpace(s.StreamRunner.Effort)
	if providerName == "" || model == "" ||
		(providerName == s.ProviderName && model == s.Model &&
			strings.TrimSpace(selected.Variant) == currentVariant &&
			strings.TrimSpace(selected.Effort) == currentEffort) {
		return cloneStreamRunnerForThread(s.StreamRunner, nil), nil
	}

	cfg, _, err := s.LoadEffectiveConfig()
	if err != nil {
		return nil, err
	}
	providerCfg, resolvedName, err := cfg.ResolveProvider(providerName)
	if err != nil {
		// The conversation pins a provider that was since removed from config.
		// The main thread self-heals to the workspace defaults on its next
		// turn, so the side chat follows the same fallback instead of failing.
		providers.DebugLogf("side thread base runner falling back to workspace model (%q unavailable): %v", providerName, err)
		return cloneStreamRunnerForThread(s.StreamRunner, nil), nil
	}
	ruleProviderName, ruleProviderCfg := modelcatalog.EnrichProvider(resolvedName, providerCfg, model)
	selection := modelvariant.ResolveForProvider(ruleProviderName, ruleProviderCfg, model, strings.TrimSpace(selected.Variant), strings.TrimSpace(selected.Effort))
	client, err := providerfactory.BuildStreamClient(ruleProviderCfg, resolvedName)
	if err != nil {
		return nil, fmt.Errorf("build side thread model client: %w", err)
	}
	budget := ResolveModelBudget(model, ruleProviderCfg, cfg.Agent.MaxContextTokens)
	apiModel := modelcatalog.APIModel(ruleProviderCfg, model)

	runner := cloneStreamRunnerForThread(s.StreamRunner, nil)
	runner.Client = client
	runner.ProviderName = resolvedName
	runner.Model = model
	runner.APIModel = apiModel
	runner.Effort = selection.LegacyEffort
	runner.Variant = selection.Variant
	runner.ProviderOptions = modelvariant.CloneOptions(selection.ProviderOptions)
	runner.ContextWindowOverride = budget.ContextWindowTokens
	runner.MaxInputTokens = budget.InputLimitTokens
	runner.OutputReserveTokens = budget.OutputReserveTokens
	runner.CompactThresholdTokens = budget.CompactThresholdTokens
	return runner, nil
}
