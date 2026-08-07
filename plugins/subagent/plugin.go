package subagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
)

const (
	hostChildSession = "host.child_session.request"
	capabilityPrompt = "agent.system_prompt.section"
	capabilityClient = "plugin.client.request"
)

var taskNameCleaner = regexp.MustCompile(`[^a-z0-9_]+`)

func Handler() pluginapi.Handler {
	return pluginapi.Handler{
		Definition: pluginapi.Definition{
			Tools: []pluginapi.Tool{
				{ID: "spawn_agent", Description: "Delegate a bounded task when separate context or parallel work materially improves the result. Keep work local when the next step is tightly coupled or simpler to do directly. Set subagent_type for a fresh worker; omit it to fork the current conversation, and use general-purpose for unspecialized fresh work. Treat the child's final message as a deliverable and verify relevant evidence before relying on it.", InputSchema: spawnSchema(), ExecutionScopes: []string{"root"}, Activity: &pluginapi.ToolActivity{ConcurrencySafe: true}},
				{ID: "send_message", Description: "Send a message to an existing child task (queue-or-resume). Address the target by agent_id, agent_path, or task_name. Set trigger_turn=true to drive the target's next turn immediately.", InputSchema: objectSchema(map[string]any{"target": stringField("agent_id, agent_path, or task_name."), "message": stringField("Message to deliver."), "trigger_turn": map[string]any{"type": "boolean"}}, "target", "message"), ExecutionScopes: []string{"root"}, Activity: &pluginapi.ToolActivity{ConcurrencySafe: true}},
				{ID: "close_agent", Description: "Close or cancel an existing child task. Address the target by agent_id, agent_path, or task_name.", InputSchema: objectSchema(map[string]any{"target": stringField("agent_id, agent_path, or task_name.")}, "target"), ExecutionScopes: []string{"root"}, Activity: &pluginapi.ToolActivity{ConcurrencySafe: true}},
				{ID: "agent_report", Description: "Submit an optional structured handoff report for the current child agent. The final message remains the deliverable; use this for durable summary, evidence, changed files, and artifacts.", InputSchema: objectSchema(map[string]any{"outcome": map[string]any{"type": "string", "enum": []string{"completed", "partial", "blocked", "failed"}}, "summary": stringField("Concise result summary."), "constraints": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "work_done": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "blockers": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "changed_files": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "artifacts": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}}, "outcome", "summary"), ExecutionScopes: []string{"child"}},
				{ID: "helpme", Description: "Start a fresh-context recovery when the main agent may be stuck in a wrong direction, polluted context, or repeated failed attempts. The helper runs in the background and resumes you with its result.", InputSchema: helpMeSchema(), ExecutionScopes: []string{"root"}},
			},
			Capabilities: []pluginapi.Capability{
				{ID: capabilityPrompt, Kind: "transform", Version: 1},
				{ID: capabilityClient, Kind: "decision", Version: 1},
			},
			RequiredHostServices: []pluginapi.HostService{{ID: hostChildSession, Required: true}},
		},
		ExecuteTool: executeTool,
		InvokeCapability: func(ctx context.Context, host pluginapi.Host, call pluginapi.CapabilityCall) (json.RawMessage, error) {
			switch call.Capability {
			case capabilityPrompt:
				return json.Marshal(map[string]string{"text": promptSection})
			case capabilityClient:
				var request struct {
					Method string          `json:"method"`
					Input  json.RawMessage `json:"input"`
				}
				if err := json.Unmarshal(call.Input, &request); err != nil {
					return nil, err
				}
				if request.Method != "status.list" {
					return nil, fmt.Errorf("unknown client method %q", request.Method)
				}
				var result json.RawMessage
				if err := host.CallHost(ctx, hostChildSession, map[string]any{"action": "list", "actor_path": "/root", "input": request.Input}, &result); err != nil {
					return nil, err
				}
				return json.Marshal(map[string]json.RawMessage{"result": result})
			default:
				return nil, fmt.Errorf("unknown capability %q", call.Capability)
			}
		},
	}
}

func executeTool(ctx context.Context, host pluginapi.Host, call pluginapi.ToolCall) (pluginapi.ToolResult, error) {
	action := ""
	input := append(json.RawMessage(nil), call.Arguments...)
	switch call.ToolID {
	case "spawn_agent":
		action = "spawn"
		var args struct {
			Description     string `json:"description"`
			Prompt          string `json:"prompt"`
			SubagentType    string `json:"subagent_type"`
			Name            string `json:"name"`
			AgentProfile    string `json:"agent_profile"`
			Model           string `json:"model"`
			RunInBackground bool   `json:"run_in_background"`
			Isolation       string `json:"isolation"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return pluginapi.ToolResult{}, err
		}
		if strings.TrimSpace(args.Description) == "" || strings.TrimSpace(args.Prompt) == "" {
			return pluginapi.ToolResult{}, errors.New("spawn_agent requires description and prompt")
		}
		name := strings.TrimSpace(args.Name)
		if name == "" {
			name = deriveTaskName(args.Description)
		}
		input, _ = json.Marshal(map[string]any{"type": strings.TrimSpace(args.SubagentType), "task_name": name, "agent_profile": strings.TrimSpace(args.AgentProfile), "description": strings.TrimSpace(args.Description), "prompt": workerPrompt(strings.TrimSpace(args.SubagentType), strings.TrimSpace(args.Prompt)), "isolation": strings.TrimSpace(args.Isolation), "model_alias": strings.TrimSpace(args.Model), "run_in_background": args.RunInBackground})
	case "send_message":
		action = "send"
	case "close_agent":
		action = "close"
	case "agent_report":
		action = "report"
	case "helpme":
		if strings.TrimSpace(call.ActorPath) != "" && strings.TrimSpace(call.ActorPath) != "/root" {
			return pluginapi.ToolResult{}, errors.New("helpme is only available to the main agent")
		}
		var args struct {
			Reason               string   `json:"reason"`
			OriginalGoal         string   `json:"original_goal"`
			CurrentUnderstanding string   `json:"current_understanding"`
			Ask                  string   `json:"ask"`
			FailedAttempts       []string `json:"failed_attempts"`
			Constraints          []string `json:"constraints"`
			Evidence             []string `json:"evidence"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return pluginapi.ToolResult{}, err
		}
		action = "spawn"
		input, _ = json.Marshal(map[string]any{"type": "general-purpose", "task_name": "helpme_recovery", "description": "HelpMe recovery", "run_in_background": true, "prompt": buildHelpMePrompt(args.OriginalGoal, args.Reason, args.CurrentUnderstanding, args.Ask, args.FailedAttempts, args.Constraints, args.Evidence)})
	default:
		return pluginapi.ToolResult{}, fmt.Errorf("unknown tool %q", call.ToolID)
	}
	var result json.RawMessage
	err := host.CallHost(ctx, hostChildSession, map[string]any{"action": action, "actor_id": call.ActorID, "actor_path": call.ActorPath, "input": input}, &result)
	if err != nil {
		return pluginapi.ToolResult{}, err
	}
	return pluginapi.TextResult(string(result)), nil
}

func deriveTaskName(description string) string {
	name := taskNameCleaner.ReplaceAllString(strings.ToLower(strings.TrimSpace(description)), "_")
	name = strings.Trim(name, "_")
	if len(name) > 40 {
		name = strings.TrimRight(name[:40], "_")
	}
	if name == "" {
		return "task"
	}
	return name
}

func stringField(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}
func objectSchema(properties map[string]any, required ...string) map[string]any {
	return map[string]any{"type": "object", "properties": properties, "required": required}
}
func spawnSchema() map[string]any {
	return objectSchema(map[string]any{
		"description":   stringField("Short 3-5 word summary of what the agent will do."),
		"prompt":        stringField("Concrete, self-contained task brief with scope, constraints, acceptance criteria, and deliverable."),
		"subagent_type": stringField("Optional specialized agent type; omit to fork the current conversation."),
		"name":          stringField("Optional addressable task name using lowercase letters, digits, and underscores."),
		"agent_profile": stringField("Optional saved agent profile."), "model": stringField("Optional configured model alias."),
		"run_in_background": map[string]any{"type": "boolean"}, "isolation": map[string]any{"type": "string", "enum": []string{"worktree"}},
	}, "description", "prompt")
}

func helpMeSchema() map[string]any {
	properties := map[string]any{
		"reason": stringField("Why recovery is needed now."), "original_goal": stringField("The user's original intent or latest task contract."),
		"current_understanding": stringField("Current understanding and uncertainty."), "ask": stringField("The exact task for the fresh helper."),
	}
	for _, key := range []string{"failed_attempts", "constraints", "evidence"} {
		properties[key] = map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
	}
	return objectSchema(properties)
}

func buildHelpMePrompt(goal, reason, understanding, ask string, failed, constraints, evidence []string) string {
	brief := fmt.Sprintf("You are a fresh-context recovery helper. Diagnose the task independently and return a bounded, evidence-backed recommendation.\n\nOriginal goal:\n%s\n\nReason for recovery:\n%s\n\nCurrent understanding (treat as potentially wrong):\n%s\n\nExact ask:\n%s\n\nFailed attempts:\n- %s\n\nConstraints:\n- %s\n\nEvidence:\n- %s", strings.TrimSpace(goal), strings.TrimSpace(reason), strings.TrimSpace(understanding), strings.TrimSpace(ask), strings.Join(failed, "\n- "), strings.Join(constraints, "\n- "), strings.Join(evidence, "\n- "))
	return workerPrompt("general-purpose", brief)
}

func workerPrompt(workerType, task string) string {
	instructions := `Work as a bounded child task. Complete only the assigned scope, preserve unrelated work, verify applicable claims, and report exact changed files, commands, blockers, and evidence. Your final message is the deliverable the parent receives; agent_report is an optional structured handoff, not a substitute for a clear final message.`
	if strings.TrimSpace(workerType) == "worker" {
		instructions = `Implement only the assigned scoped change in the provided isolated workspace. Preserve unrelated work, verify locally, and report exact changed files, commands, blockers, and evidence. Your final message is the deliverable the parent receives.`
	}
	return instructions + "\n\n" + strings.TrimSpace(task)
}

const promptSection = `# Subagent results

A completed subagent task does not mean the overall task is complete. Integrate the result and verify the overall work before claiming completion.

# Delegation

The main agent owns the user conversation, final synthesis, and decision about whether delegation is worth the overhead. Keep tightly coupled, trivial, or critical-path work local. Delegate bounded independent research, verification, or disjoint implementation when separate context or parallel execution materially improves the result.

Fresh task prompts must be self-contained: include the task, background, scope, non-goals, starting points, constraints, acceptance criteria, deliverable, and for edits the owned files or modules. Fork prompts may rely on inherited context but still need a concrete directive and scope.

Background completions are internal handoffs, not new user requests. Read and verify them before synthesis. Do not poll; continue meaningful non-overlapping work or end the turn and let completion resume the session. Reconcile changed-file overlap before integrating results.

# Subagent Types

These are the spawn_agent subagent_type values available this session. Set subagent_type to launch a fresh specialized agent; omit it to fork yourself with full conversation context.

- general-purpose: General-purpose agent for researching complex questions, searching for code, and executing multi-step tasks.
- worker: Implement a scoped code change in an isolated worktree when edits are required.`
