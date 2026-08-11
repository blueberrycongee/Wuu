package subagent

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
)

const (
	capabilityPrompt    = "agent.system_prompt.section"
	capabilityClient    = "plugin.client.request"
	capabilityLifecycle = "agent.turn.lifecycle"
	capabilityInterrupt = "agent.turn.interrupted"
)

var taskNameCleaner = regexp.MustCompile(`[^a-z0-9_]+`)

type taskRecord struct {
	SessionID       string `json:"session_id"`
	ParentSessionID string `json:"parent_session_id"`
	ParentTurnID    string `json:"parent_turn_id,omitempty"`
	Name            string `json:"name"`
	RequestID       string `json:"request_id"`
	TurnID          string `json:"turn_id,omitempty"`
	QueueID         string `json:"queue_id,omitempty"`
	State           string `json:"state"`
}

func Handler() pluginapi.Handler {
	return pluginapi.Handler{
		Definition: pluginapi.Definition{
			Tools: []pluginapi.Tool{
				{ID: "spawn_agent", Description: "Delegate a bounded task when separate context or parallel work materially improves the result. Child sessions start with fresh conversation context unless context is explicitly set to fork. Installed plugins and workspace capabilities remain available in either mode. Completion is delivered back into this session as a normal read-only query bubble.", InputSchema: spawnSchema(), ExecutionScopes: []string{"root"}, Activity: &pluginapi.ToolActivity{ConcurrencySafe: true}},
				{ID: "send_message", Description: "Send a follow-up turn to an existing child task by session id or task name.", InputSchema: objectSchema(map[string]any{"target": stringField("Child session id or task name."), "message": stringField("Message to deliver.")}, "target", "message"), ExecutionScopes: []string{"root"}, Activity: &pluginapi.ToolActivity{ConcurrencySafe: true}},
				{ID: "close_agent", Description: "Cancel an existing child task by session id or task name.", InputSchema: objectSchema(map[string]any{"target": stringField("Child session id or task name.")}, "target"), ExecutionScopes: []string{"root"}, Activity: &pluginapi.ToolActivity{ConcurrencySafe: true}},
			},
			Capabilities: []pluginapi.Capability{
				{ID: capabilityPrompt, Kind: "transform", Version: 1},
				{ID: capabilityClient, Kind: "decision", Version: 1},
				{ID: capabilityLifecycle, Kind: "observe", Version: 1, ErrorPolicy: "isolate"},
				{ID: capabilityInterrupt, Kind: "observe", Version: 1, ErrorPolicy: "isolate"},
			},
			RequiredHostServices: []pluginapi.HostService{
				{ID: pluginapi.HostServiceSessionCreate, Required: true},
				{ID: pluginapi.HostServiceSessionSend, Required: true},
				{ID: pluginapi.HostServiceSessionList, Required: true},
				{ID: pluginapi.HostServiceSessionCancel, Required: true},
				{ID: pluginapi.HostServiceStorageGet, Required: true},
				{ID: pluginapi.HostServiceStorageSet, Required: true},
			},
		},
		ExecuteTool:      executeTool,
		InvokeCapability: invokeCapability,
	}
}

func executeTool(ctx context.Context, host pluginapi.Host, call pluginapi.ToolCall) (pluginapi.ToolResult, error) {
	switch call.ToolID {
	case "spawn_agent":
		return spawnAgent(ctx, host, call)
	case "send_message":
		return sendMessage(ctx, host, call)
	case "close_agent":
		return closeAgent(ctx, host, call)
	default:
		return pluginapi.ToolResult{}, fmt.Errorf("unknown tool %q", call.ToolID)
	}
}

func spawnAgent(ctx context.Context, host pluginapi.Host, call pluginapi.ToolCall) (pluginapi.ToolResult, error) {
	var args struct {
		Description  string `json:"description"`
		Prompt       string `json:"prompt"`
		SubagentType string `json:"subagent_type"`
		Name         string `json:"name"`
		AgentProfile string `json:"agent_profile"`
		Model        string `json:"model"`
		Context      string `json:"context"`
		Isolation    string `json:"isolation"`
	}
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		return pluginapi.ToolResult{}, err
	}
	if strings.TrimSpace(args.Description) == "" || strings.TrimSpace(args.Prompt) == "" {
		return pluginapi.ToolResult{}, errors.New("spawn_agent requires description and prompt")
	}
	if strings.TrimSpace(args.AgentProfile) != "" {
		return pluginapi.ToolResult{}, errors.New("agent_profile is no longer part of the Subagent plugin contract; put required memory in the task prompt")
	}
	if strings.TrimSpace(call.SessionID) == "" {
		return pluginapi.ToolResult{}, errors.New("spawn_agent requires a parent session")
	}
	name := strings.TrimSpace(args.Name)
	if name == "" {
		name = deriveTaskName(args.Description)
	}
	requestID, err := newRequestID()
	if err != nil {
		return pluginapi.ToolResult{}, err
	}
	contextSource := "fresh"
	switch strings.TrimSpace(args.Context) {
	case "", "fresh":
	case "fork":
		contextSource = "fork"
	default:
		return pluginapi.ToolResult{}, fmt.Errorf("unsupported subagent context %q", args.Context)
	}
	workspace := "shared"
	if strings.TrimSpace(args.Isolation) == "worktree" {
		workspace = "worktree"
		contextSource = "fork"
	}
	var created pluginapi.SessionCreateResult
	err = host.CallHost(ctx, pluginapi.HostServiceSessionCreate, pluginapi.SessionCreateParams{RequestID: "create-" + requestID, Name: name, Visibility: "plugin", ParentSessionID: call.SessionID, ContextSource: contextSource, Workspace: workspace, ModelAlias: strings.TrimSpace(args.Model)}, &created)
	if err != nil {
		return pluginapi.ToolResult{}, err
	}
	record := taskRecord{SessionID: created.SessionID, ParentSessionID: call.SessionID, ParentTurnID: call.TurnID, Name: name, RequestID: "turn-" + requestID, State: "created"}
	if err := saveRecord(ctx, host, record); err != nil {
		return pluginapi.ToolResult{}, err
	}
	var sent pluginapi.SessionSendResult
	err = host.CallHost(ctx, pluginapi.HostServiceSessionSend, pluginapi.SessionSendParams{RequestID: record.RequestID, SessionID: created.SessionID, Input: pluginapi.SessionInput{Prompt: workerPrompt(strings.TrimSpace(args.SubagentType), strings.TrimSpace(args.Prompt))}, Presentation: &pluginapi.SessionInputPresentation{Kind: "query_bubble", Text: "子任务 " + name + " 已开始", Name: name}, Cause: "subagent.task"}, &sent)
	if err != nil {
		return pluginapi.ToolResult{}, err
	}
	record.State = sent.State
	record.TurnID = sent.TurnID
	record.QueueID = sent.QueueID
	if err := saveRecord(ctx, host, record); err != nil {
		return pluginapi.ToolResult{}, err
	}
	return pluginapi.TextResult(fmt.Sprintf(`{"session_id":%q,"task_name":%q,"state":%q}`, created.SessionID, name, sent.State)), nil
}

func sendMessage(ctx context.Context, host pluginapi.Host, call pluginapi.ToolCall) (pluginapi.ToolResult, error) {
	var args struct {
		Target  string `json:"target"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		return pluginapi.ToolResult{}, err
	}
	record, err := resolveTask(ctx, host, call.SessionID, args.Target)
	if err != nil {
		return pluginapi.ToolResult{}, err
	}
	requestID, err := newRequestID()
	if err != nil {
		return pluginapi.ToolResult{}, err
	}
	nextRequestID := "turn-" + requestID
	previousRequestID := record.RequestID
	record.RequestID = nextRequestID
	record.ParentSessionID = call.SessionID
	record.ParentTurnID = call.TurnID
	if err := saveRecord(ctx, host, record); err != nil {
		return pluginapi.ToolResult{}, err
	}
	var sent pluginapi.SessionSendResult
	if err := host.CallHost(ctx, pluginapi.HostServiceSessionSend, pluginapi.SessionSendParams{RequestID: nextRequestID, SessionID: record.SessionID, Input: pluginapi.SessionInput{Prompt: strings.TrimSpace(args.Message)}, Presentation: &pluginapi.SessionInputPresentation{Kind: "query_bubble", Text: "父任务已更新要求", Name: record.Name}, Cause: "subagent.followup", IfRunning: pluginapi.SessionIfRunningSteer}, &sent); err != nil {
		return pluginapi.ToolResult{}, err
	}
	if sent.Steered {
		record.RequestID = previousRequestID
	}
	record.State = sent.State
	record.TurnID = sent.TurnID
	record.QueueID = sent.QueueID
	if err := saveRecord(ctx, host, record); err != nil {
		return pluginapi.ToolResult{}, err
	}
	return pluginapi.TextResult(fmt.Sprintf(`{"session_id":%q,"state":%q,"steered":%t}`, record.SessionID, sent.State, sent.Steered)), nil
}

func closeAgent(ctx context.Context, host pluginapi.Host, call pluginapi.ToolCall) (pluginapi.ToolResult, error) {
	var args struct {
		Target string `json:"target"`
	}
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		return pluginapi.ToolResult{}, err
	}
	record, err := resolveTask(ctx, host, call.SessionID, args.Target)
	if err != nil {
		return pluginapi.ToolResult{}, err
	}
	var result pluginapi.SessionCancelResult
	if err := host.CallHost(ctx, pluginapi.HostServiceSessionCancel, pluginapi.SessionCancelParams{SessionID: record.SessionID, TurnID: record.TurnID, QueueID: record.QueueID}, &result); err != nil {
		return pluginapi.ToolResult{}, err
	}
	return pluginapi.TextResult(fmt.Sprintf(`{"session_id":%q,"cancelled":%t}`, result.SessionID, result.Cancelled)), nil
}

func invokeCapability(ctx context.Context, host pluginapi.Host, call pluginapi.CapabilityCall) (json.RawMessage, error) {
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
		switch request.Method {
		case "status.list":
			var params pluginapi.SessionListParams
			if len(request.Input) != 0 {
				if err := json.Unmarshal(request.Input, &params); err != nil {
					return nil, err
				}
			}
			var result pluginapi.SessionListResult
			if err := host.CallHost(ctx, pluginapi.HostServiceSessionList, params, &result); err != nil {
				return nil, err
			}
			return json.Marshal(map[string]any{"result": result})
		default:
			return nil, fmt.Errorf("unknown client method %q", request.Method)
		}
	case capabilityLifecycle:
		var input pluginapi.TurnLifecycleInput
		if err := json.Unmarshal(call.Input, &input); err != nil {
			return nil, err
		}
		if input.State != "completed" && input.State != "failed" && input.State != "interrupted" && input.State != "discarded" {
			return json.RawMessage(`{}`), nil
		}
		record, ok, err := loadRecord(ctx, host, input.RequestID)
		if err != nil || !ok {
			return json.RawMessage(`{}`), err
		}
		record.State = input.State
		if err := saveRecord(ctx, host, record); err != nil {
			return nil, err
		}
		output := strings.TrimSpace(input.FinalOutput)
		if output == "" {
			output = strings.TrimSpace(input.Error)
		}
		if output == "" {
			output = "子任务已结束，但没有返回文本结果。"
		}
		deliveryRequestID := completionDeliveryRequestID(input.RequestID)
		continuation, tracksContinuation, err := parentContinuationRecord(ctx, host, record, deliveryRequestID)
		if err != nil {
			return nil, err
		}
		if tracksContinuation {
			if err := saveRecord(ctx, host, continuation); err != nil {
				return nil, err
			}
		}
		prompt := fmt.Sprintf("子任务 %s（session %s）已%s。请检查并整合以下交接结果：\n\n%s", record.Name, record.SessionID, lifecycleLabel(input.State), output)
		var sent pluginapi.SessionSendResult
		if err := host.CallHost(ctx, pluginapi.HostServiceSessionSend, pluginapi.SessionSendParams{RequestID: deliveryRequestID, SessionID: record.ParentSessionID, Input: pluginapi.SessionInput{Prompt: prompt}, Presentation: &pluginapi.SessionInputPresentation{Kind: "query_bubble", Text: "子任务 " + record.Name + " 已更新", Name: record.Name}, Cause: "subagent.completion"}, &sent); err != nil {
			return nil, err
		}
		if tracksContinuation {
			continuation.State = sent.State
			if err := saveRecord(ctx, host, continuation); err != nil {
				return nil, err
			}
		}
		return json.RawMessage(`{}`), nil
	case capabilityInterrupt:
		var input pluginapi.AgentTurnInterruptedInput
		if err := json.Unmarshal(call.Input, &input); err != nil {
			return nil, err
		}
		if strings.TrimSpace(input.ThreadID) == "" || strings.TrimSpace(input.TurnID) == "" {
			return json.RawMessage(`{}`), nil
		}
		var listed pluginapi.SessionListResult
		if err := host.CallHost(ctx, pluginapi.HostServiceSessionList, pluginapi.SessionListParams{ParentSessionID: input.ThreadID}, &listed); err != nil {
			return nil, err
		}
		for _, child := range listed.Sessions {
			record, ok, err := loadRecordBySession(ctx, host, child.SessionID)
			if err != nil {
				return nil, err
			}
			if !ok || strings.TrimSpace(record.ParentTurnID) != strings.TrimSpace(input.TurnID) {
				continue
			}
			var result pluginapi.SessionCancelResult
			if err := host.CallHost(ctx, pluginapi.HostServiceSessionCancel, pluginapi.SessionCancelParams{SessionID: record.SessionID, TurnID: record.TurnID, QueueID: record.QueueID}, &result); err != nil {
				return nil, err
			}
			if result.Cancelled {
				record.State = "interrupted"
				if err := saveRecord(ctx, host, record); err != nil {
					return nil, err
				}
			}
		}
		return json.RawMessage(`{}`), nil
	default:
		return nil, fmt.Errorf("unknown capability %q", call.Capability)
	}
}

func resolveTask(ctx context.Context, host pluginapi.Host, parentID, target string) (taskRecord, error) {
	target = strings.TrimSpace(target)
	var listed pluginapi.SessionListResult
	if err := host.CallHost(ctx, pluginapi.HostServiceSessionList, pluginapi.SessionListParams{ParentSessionID: strings.TrimSpace(parentID)}, &listed); err != nil {
		return taskRecord{}, err
	}
	var match *pluginapi.SessionSummary
	for index := range listed.Sessions {
		item := &listed.Sessions[index]
		if item.SessionID != target && item.Name != target {
			continue
		}
		if match != nil {
			return taskRecord{}, fmt.Errorf("task name %q is ambiguous; use the session id", target)
		}
		match = item
	}
	if match == nil {
		return taskRecord{}, fmt.Errorf("child session %q not found", target)
	}
	record, ok, err := loadRecordBySession(ctx, host, match.SessionID)
	if err != nil {
		return taskRecord{}, err
	}
	if ok {
		return record, nil
	}
	return taskRecord{SessionID: match.SessionID, ParentSessionID: match.ParentSessionID, Name: match.Name, State: match.State}, nil
}

func saveRecord(ctx context.Context, host pluginapi.Host, record taskRecord) error {
	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}
	for _, key := range []string{"request." + record.RequestID, "session." + record.SessionID} {
		if err := host.CallHost(ctx, pluginapi.HostServiceStorageSet, map[string]any{"scope": "workspace", "key": key, "value": string(encoded)}, &struct{}{}); err != nil {
			return err
		}
	}
	return nil
}

func loadRecord(ctx context.Context, host pluginapi.Host, requestID string) (taskRecord, bool, error) {
	return loadStoredRecord(ctx, host, "request."+strings.TrimSpace(requestID))
}
func loadRecordBySession(ctx context.Context, host pluginapi.Host, sessionID string) (taskRecord, bool, error) {
	return loadStoredRecord(ctx, host, "session."+strings.TrimSpace(sessionID))
}
func loadStoredRecord(ctx context.Context, host pluginapi.Host, key string) (taskRecord, bool, error) {
	var result struct {
		Value *string `json:"value"`
	}
	if err := host.CallHost(ctx, pluginapi.HostServiceStorageGet, map[string]any{"scope": "workspace", "key": key}, &result); err != nil {
		return taskRecord{}, false, err
	}
	if result.Value == nil {
		return taskRecord{}, false, nil
	}
	var record taskRecord
	if err := json.Unmarshal([]byte(*result.Value), &record); err != nil {
		return taskRecord{}, false, err
	}
	return record, true, nil
}

func parentContinuationRecord(ctx context.Context, host pluginapi.Host, child taskRecord, requestID string) (taskRecord, bool, error) {
	parentID := strings.TrimSpace(child.ParentSessionID)
	if parentID == "" {
		return taskRecord{}, false, nil
	}
	parent, ok, err := loadRecordBySession(ctx, host, parentID)
	if err != nil {
		return taskRecord{}, false, err
	}
	if !ok || strings.TrimSpace(parent.SessionID) != parentID || strings.TrimSpace(parent.ParentSessionID) == "" {
		return taskRecord{}, false, nil
	}
	return taskRecord{
		SessionID:       parentID,
		ParentSessionID: parent.ParentSessionID,
		Name:            parent.Name,
		RequestID:       requestID,
		State:           "created",
	}, true, nil
}

func completionDeliveryRequestID(requestID string) string {
	digest := sha256.Sum256([]byte("subagent.completion\x00" + strings.TrimSpace(requestID)))
	return "deliver-" + hex.EncodeToString(digest[:12])
}

func newRequestID() (string, error) {
	var raw [12]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}
func lifecycleLabel(state string) string {
	if state == "completed" {
		return "完成"
	}
	if state == "failed" {
		return "失败"
	}
	if state == "interrupted" {
		return "中断"
	}
	return "取消"
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
	return objectSchema(map[string]any{"description": stringField("Short 3-5 word summary of what the agent will do."), "prompt": stringField("Concrete, self-contained task brief with scope, constraints, acceptance criteria, and deliverable."), "subagent_type": stringField("Optional specialized agent type."), "name": stringField("Optional addressable task name using lowercase letters, digits, and underscores."), "model": stringField("Optional configured model alias."), "context": map[string]any{"type": "string", "enum": []string{"fresh", "fork"}, "description": "Optional conversation context source. Defaults to fresh; fork inherits the parent conversation."}, "isolation": map[string]any{"type": "string", "enum": []string{"worktree"}}}, "description", "prompt")
}

func workerPrompt(workerType, task string) string {
	instructions := `Work as a bounded child task. Complete only the assigned scope, preserve unrelated work, verify applicable claims, and report exact changed files, commands, blockers, and evidence. Your final message is the deliverable the parent receives.`
	if strings.TrimSpace(workerType) == "worker" {
		instructions = `Implement only the assigned scoped change in the provided isolated workspace. Preserve unrelated work, verify locally, and report exact changed files, commands, blockers, and evidence. Your final message is the deliverable the parent receives.`
	}
	return instructions + "\n\n" + strings.TrimSpace(task)
}

const promptSection = `# Subagent results

A completed subagent task does not mean the overall task is complete. Integrate the result and verify the overall work before claiming completion.

# Delegation

The main agent owns the user conversation, final synthesis, and decision about whether delegation is worth the overhead. Keep tightly coupled, trivial, or critical-path work local. Delegate bounded independent research, verification, or disjoint implementation when separate context or parallel execution materially improves the result.

The Subagent plugin owns its task records, cancellation propagation, concurrency policy, and recovery. Use the public Session services to build whatever task topology fits the work; the host does not impose a delegation tree or a global child limit.

Child sessions use fresh conversation context by default while retaining installed plugins and workspace capabilities. Fresh task prompts must be self-contained. Use context=fork only when inherited conversation history is materially necessary, and still provide a concrete directive and scope. Child completion is delivered as a read-only query bubble; do not poll while work is running.`
