package agentcontrol

import (
	"fmt"
	"sort"
	"strings"
)

// IsolationMode controls whether a worker runs in its own git
// worktree or shares the parent's working directory.
type IsolationMode string

const (
	IsolationInplace  IsolationMode = "inplace"
	IsolationWorktree IsolationMode = "worktree"
)

// WorkerType defines a role a worker can adopt.
type WorkerType struct {
	Name             string
	Role             string
	Description      string
	SystemPrompt     string
	AllowedTools     []string
	DisallowedTools  []string
	ContextScope     string
	OutputSchema     string
	SuccessCriteria  []string
	OneShot          bool
	Background       bool
	DefaultIsolation IsolationMode
	// RequiresReport marks verdict-class workers whose value depends on a
	// structured agent_report handoff. When such a worker completes without
	// filing one, the runtime issues a single mechanical closing turn with
	// tool_choice pinned to agent_report before results are delivered. It
	// never becomes a lifecycle state: if the closing turn still files
	// nothing, the run completes with a synthesized final_text report.
	RequiresReport bool
}

const DefaultSubagentType = "general-purpose"

// HelpMeRecoveryWorkerType is the worker type the helpme tool spawns for a
// fresh-context recovery. Its tool surface matches general-purpose; unlike
// general-purpose it carries RequiresReport because the parent-side history
// rewrite is built from the helper's structured agent_report handoff.
const HelpMeRecoveryWorkerType = "helpme_recovery"

var builtinWorkerTypes = map[string]WorkerType{
	DefaultSubagentType: {
		Name:             DefaultSubagentType,
		Role:             "Generalist",
		Description:      "General-purpose agent for researching complex questions, searching for code, and executing multi-step tasks.",
		AllowedTools:     nil,
		OneShot:          false,
		DefaultIsolation: IsolationInplace,
		ContextScope:     "Self-contained task prompt plus repository memory and skills.",
		OutputSchema:     "Final message is the deliverable: outcome, what was done, what changed, blockers, and verifiable evidence. agent_report is an optional structured handoff on top.",
		SuccessCriteria: []string{
			"Task scope is completed or clearly blocked.",
			"Changed files and verification are reported with evidence.",
		},
		SystemPrompt: "",
	},
	HelpMeRecoveryWorkerType: {
		Name:             HelpMeRecoveryWorkerType,
		Role:             "HelpMe Recovery",
		Description:      "Fresh-context recovery helper launched by the helpme tool after the parent got stuck; prefer calling helpme instead of spawning this type directly.",
		AllowedTools:     nil,
		OneShot:          false,
		DefaultIsolation: IsolationInplace,
		ContextScope:     "Self-contained HelpMe handoff brief (goal, ask, reason, constraints, failed attempts, evidence) plus repository memory and skills.",
		OutputSchema:     "Final message is the deliverable. A structured agent_report handoff is additionally required — submitted at close, requested automatically if missing — because the parent's context rewrite is built from it.",
		SuccessCriteria: []string{
			"Recovery ask is completed or clearly blocked.",
			"Changed files and verification are reported with evidence.",
		},
		SystemPrompt:   "",
		RequiresReport: true,
	},
	"worker": {
		Name:         "worker",
		Role:         "Worker",
		Description:  "Implement a scoped code change in an isolated worktree when edits are required.",
		SystemPrompt: "",
		AllowedTools: nil,
		ContextScope: "Concrete task brief, plan step, allowed write set, current worktree status, and verification command.",
		OutputSchema: "Implementation report with changed files, commands run, blockers, risks, and evidence.",
		SuccessCriteria: []string{
			"Only assigned files or behavior are changed.",
			"Targeted verification ran or a blocker explains why it could not run.",
		},
		OneShot:          false,
		Background:       false,
		DefaultIsolation: IsolationWorktree,
	},
}

// AvailableWorkerTypes returns user-selectable built-in worker roles sorted by
// name. Internal worker types remain available to trusted runtime paths through
// LookupWorkerType but are not advertised to models or accepted by public
// orchestration tools.
func AvailableWorkerTypes() []WorkerType {
	keys := make([]string, 0, len(builtinWorkerTypes))
	for key := range builtinWorkerTypes {
		if key == HelpMeRecoveryWorkerType {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]WorkerType, 0, len(keys))
	for _, key := range keys {
		out = append(out, builtinWorkerTypes[key])
	}
	return out
}

// LookupPublicWorkerType resolves only worker types exposed through
// spawn_agent. HelpMe recovery is an internal implementation detail launched
// by the gated helpme tool.
func LookupPublicWorkerType(name string) (WorkerType, error) {
	wt, err := LookupWorkerType(name)
	if err != nil {
		return WorkerType{}, err
	}
	if wt.Name == HelpMeRecoveryWorkerType {
		return WorkerType{}, fmt.Errorf("worker type %q is internal (available: %s)", name, strings.Join(AvailableWorkerTypeNames(), ", "))
	}
	return wt, nil
}

// AvailableWorkerTypeNames returns all built-in worker role names sorted by name.
func AvailableWorkerTypeNames() []string {
	items := AvailableWorkerTypes()
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Name)
	}
	return out
}

// LookupWorkerType resolves a worker type name to its definition.
func LookupWorkerType(name string) (WorkerType, error) {
	if name == "" {
		name = DefaultSubagentType
	}
	wt, ok := builtinWorkerTypes[name]
	if !ok {
		return WorkerType{}, fmt.Errorf("unknown worker type %q (available: %s)", name, strings.Join(AvailableWorkerTypeNames(), ", "))
	}
	return wt, nil
}

// alwaysBlockedTools is the set of tools that no anonymous worker may use.
// Ultra unlocks recursive orchestration, but helpme remains a root-only
// recovery mechanism.
var alwaysBlockedTools = map[string]struct{}{
	"helpme": {},
}

var defaultWorkerBlockedTools = map[string]struct{}{
	"spawn_agent":  {},
	"send_message": {},
	"close_agent":  {},
}

// FilterToolsForWorker returns the subset of fullList that this worker
// type is allowed to call. ultraMode is optional so existing default-mode
// callers retain byte-for-byte behavior without supplying a new argument.
func FilterToolsForWorker(wt WorkerType, fullList []string, ultraMode ...bool) []string {
	ultra := len(ultraMode) > 0 && ultraMode[0]
	out := make([]string, 0, len(fullList))
	allowSet := map[string]struct{}{}
	for _, t := range wt.AllowedTools {
		allowSet[t] = struct{}{}
	}
	denySet := map[string]struct{}{}
	for _, t := range wt.DisallowedTools {
		denySet[t] = struct{}{}
	}
	for _, name := range fullList {
		if _, blocked := alwaysBlockedTools[name]; blocked {
			continue
		}
		if !ultra {
			if _, blocked := defaultWorkerBlockedTools[name]; blocked {
				continue
			}
		}
		if _, denied := denySet[name]; denied {
			continue
		}
		if len(wt.AllowedTools) == 0 {
			// nil means all non-orchestration tools allowed
			out = append(out, name)
			continue
		}
		if _, ok := allowSet[name]; ok {
			out = append(out, name)
		}
	}
	return out
}

// NormalizeIsolation resolves the effective isolation mode for a spawn request.
func NormalizeIsolation(reqIsolation string, wt WorkerType) (IsolationMode, error) {
	if reqIsolation == "" {
		if wt.DefaultIsolation != "" {
			return wt.DefaultIsolation, nil
		}
		return IsolationInplace, nil
	}
	switch strings.ToLower(reqIsolation) {
	case "inplace":
		return IsolationInplace, nil
	case "worktree":
		return IsolationWorktree, nil
	default:
		return "", fmt.Errorf("invalid isolation %q (valid: inplace, worktree)", reqIsolation)
	}
}
