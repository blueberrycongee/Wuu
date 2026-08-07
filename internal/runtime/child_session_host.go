package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/blueberrycongee/wuu/internal/agent"
	"github.com/blueberrycongee/wuu/internal/agentcontrol"
	"github.com/blueberrycongee/wuu/internal/agentthread"
	"github.com/blueberrycongee/wuu/internal/pluginhost"
	"github.com/blueberrycongee/wuu/internal/toolctx"
)

// dispatchChildSessionRequest implements the neutral child-session host seam.
// Plugins own product vocabulary, schemas, prompting, and presentation; the
// host owns admission, execution leases, persistence, and child lifecycle.
func dispatchChildSessionRequest(control *agentcontrol.AgentControl, ctx context.Context, request pluginhost.ChildSessionRequestParams) (json.RawMessage, error) {
	if control == nil {
		return nil, errors.New("child-session service is unavailable")
	}
	actorPath := strings.TrimSpace(request.ActorPath)
	if actorPath == "" {
		actorPath = agentthread.RootPath
	}
	switch strings.TrimSpace(request.Action) {
	case "spawn":
		var input struct {
			Type            string `json:"type"`
			TaskName        string `json:"task_name"`
			AgentProfile    string `json:"agent_profile"`
			Description     string `json:"description"`
			Prompt          string `json:"prompt"`
			Isolation       string `json:"isolation"`
			ModelAlias      string `json:"model_alias"`
			RunInBackground bool   `json:"run_in_background"`
		}
		if err := json.Unmarshal(request.Input, &input); err != nil {
			return nil, err
		}
		if strings.TrimSpace(input.Type) == "" {
			history := agent.HistoryFromContext(ctx)
			if len(history) == 0 {
				return nil, errors.New("fork requires parent history")
			}
			result, err := control.Fork(ctx, agentcontrol.ForkRequest{
				TaskName: input.TaskName, AgentProfile: input.AgentProfile,
				Description: input.Description, Prompt: input.Prompt, ForkMode: "all",
				ParentID: strings.TrimSpace(request.ActorID), ParentPath: actorPath,
				Isolation: input.Isolation, ModelAlias: input.ModelAlias, Synchronous: false,
			}, history)
			return marshalChildSessionResult(result, err)
		}
		workerType, err := agentcontrol.LookupPublicWorkerType(input.Type)
		if err != nil {
			return nil, err
		}
		result, err := control.Spawn(ctx, agentcontrol.SpawnRequest{
			Type: input.Type, TaskName: input.TaskName, AgentProfile: input.AgentProfile,
			Description: input.Description, Prompt: input.Prompt,
			ParentID: strings.TrimSpace(request.ActorID), ParentPath: actorPath,
			Isolation: input.Isolation, ModelAlias: input.ModelAlias,
			Synchronous:   !input.RunInBackground && !workerType.Background,
			WaitInterrupt: toolctx.WaitInterrupt(ctx),
		})
		return marshalChildSessionResult(result, err)
	case "send":
		var input struct {
			Target      string `json:"target"`
			Message     string `json:"message"`
			TriggerTurn bool   `json:"trigger_turn"`
		}
		if err := json.Unmarshal(request.Input, &input); err != nil {
			return nil, err
		}
		if !input.TriggerTurn {
			if err := control.SendMessageFrom(actorPath, ctx, input.Target, input.Message); err != nil {
				return nil, err
			}
			return json.RawMessage(`{"status":"sent"}`), nil
		}
		result, err := control.FollowupTaskFrom(actorPath, ctx, input.Target, input.Message)
		return marshalChildSessionResult(result, err)
	case "close":
		var input struct {
			Target string `json:"target"`
		}
		if err := json.Unmarshal(request.Input, &input); err != nil {
			return nil, err
		}
		if !control.StopFrom(actorPath, input.Target) {
			return nil, fmt.Errorf("child session %q not found", input.Target)
		}
		return json.RawMessage(`{"status":"closed"}`), nil
	case "list":
		var input struct {
			PathPrefix string `json:"path_prefix"`
		}
		if len(request.Input) != 0 {
			if err := json.Unmarshal(request.Input, &input); err != nil {
				return nil, err
			}
		}
		return marshalChildSessionResult(control.ListFrom(actorPath, input.PathPrefix), nil)
	case "await":
		var input struct {
			Targets []string `json:"targets"`
		}
		if err := json.Unmarshal(request.Input, &input); err != nil {
			return nil, err
		}
		result, err := control.AwaitFrom(actorPath, ctx, input.Targets)
		return marshalChildSessionResult(result, err)
	case "report":
		if strings.TrimSpace(request.ActorID) == "" || actorPath == agentthread.RootPath {
			return nil, errors.New("child-session reports require a child execution actor")
		}
		var input agentcontrol.AgentReportRequest
		if err := json.Unmarshal(request.Input, &input); err != nil {
			return nil, err
		}
		result, err := control.RecordAgentReport(strings.TrimSpace(request.ActorID), actorPath, input)
		return marshalChildSessionResult(result, err)
	default:
		return nil, fmt.Errorf("unsupported child-session action %q", request.Action)
	}
}

func marshalChildSessionResult(value any, err error) (json.RawMessage, error) {
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(value)
	return json.RawMessage(encoded), err
}
