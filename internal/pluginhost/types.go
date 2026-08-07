package pluginhost

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/blueberrycongee/wuu/internal/providers"
	"github.com/blueberrycongee/wuu/internal/toolresult"
)

const (
	maxToolRegistrations  = 64
	maxToolDescriptionLen = 16 * 1024
	maxToolSchemaLen      = 128 * 1024
	maxToolMetadataLen    = 1024
)

var localToolIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// Hook identifies a typed interception point exposed by the Wuu runtime.
type Hook string

const (
	HookSessionStart      Hook = "session.start"
	HookSessionStop       Hook = "session.stop"
	HookChatMessage       Hook = "chat.message"
	HookChatRequest       Hook = "chat.request"
	HookToolDefinition    Hook = "tool.definition"
	HookToolExecuteBefore Hook = "tool.execute.before"
	HookToolExecuteAfter  Hook = "tool.execute.after"
	HookShellEnv          Hook = "shell.env"
)

var validHooks = map[Hook]struct{}{
	HookSessionStart:      {},
	HookSessionStop:       {},
	HookChatMessage:       {},
	HookChatRequest:       {},
	HookToolDefinition:    {},
	HookToolExecuteBefore: {},
	HookToolExecuteAfter:  {},
	HookShellEnv:          {},
}

// IsValidHook reports whether name is part of the public plugin protocol.
func IsValidHook(name Hook) bool {
	_, ok := validHooks[name]
	return ok
}

// InitializeParams describes the runtime instance offered to a plugin.
type InitializeParams struct {
	ProtocolVersion   int    `json:"protocol_version"`
	PluginID          string `json:"plugin_id"`
	PluginRoot        string `json:"plugin_root"`
	ProjectRoot       string `json:"project_root"`
	WuuHome           string `json:"wuu_home"`
	WorkspaceStateDir string `json:"workspace_state_dir,omitempty"`
}

// InitializeResult declares the interception points and tools implemented by a
// plugin. Tool IDs are local to the declaring plugin; Wuu assigns the public
// model-visible names during registration.
type InitializeResult struct {
	Hooks []Hook             `json:"hooks"`
	Tools []ToolRegistration `json:"tools,omitempty"`
}

// ToolRegistration is one model-visible tool owned by a plugin process.
type ToolRegistration struct {
	ID              string                     `json:"id"`
	Description     string                     `json:"description"`
	InputSchema     map[string]any             `json:"input_schema"`
	ExecutionScopes []string                   `json:"execution_scopes,omitempty"`
	Activity        *ToolActivityMetadata      `json:"activity,omitempty"`
	Display         *providers.ToolCallDisplay `json:"display,omitempty"`
}

// ToolActivityMetadata controls scheduling and activity classification for a
// registered tool. It is host metadata and is not included in provider tool
// definitions.
type ToolActivityMetadata struct {
	ReadOnly        bool   `json:"read_only,omitempty"`
	ConcurrencySafe bool   `json:"concurrency_safe,omitempty"`
	Destructive     bool   `json:"destructive,omitempty"`
	Risk            string `json:"risk,omitempty"`
	Reason          string `json:"reason,omitempty"`
}

// ToolExecuteParams carries a registered tool call over the plugin protocol.
// ToolID is the plugin-local registration ID; Tool in the embedded input is
// the public model-visible name.
type ToolExecuteParams struct {
	ToolExecuteInput
	ToolID string `json:"tool_id"`
}

// ToolExecuteResult is the structured result returned by tool.execute.
type ToolExecuteResult struct {
	Result toolresult.Result `json:"result"`
}

func validateToolRegistrations(tools []ToolRegistration) error {
	if len(tools) > maxToolRegistrations {
		return fmt.Errorf("declares %d tools, limit is %d", len(tools), maxToolRegistrations)
	}
	seen := make(map[string]struct{}, len(tools))
	for index, tool := range tools {
		prefix := fmt.Sprintf("tool[%d]", index)
		if !localToolIDPattern.MatchString(tool.ID) {
			return fmt.Errorf("%s id %q must match ^[a-zA-Z0-9_-]{1,64}$", prefix, tool.ID)
		}
		if _, ok := seen[tool.ID]; ok {
			return fmt.Errorf("duplicate tool id %q", tool.ID)
		}
		seen[tool.ID] = struct{}{}
		description := strings.TrimSpace(tool.Description)
		if description == "" {
			return fmt.Errorf("%s description is required", prefix)
		}
		if len(tool.Description) > maxToolDescriptionLen {
			return fmt.Errorf("%s description exceeds %d bytes", prefix, maxToolDescriptionLen)
		}
		if tool.InputSchema == nil {
			return fmt.Errorf("%s input_schema is required", prefix)
		}
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			return fmt.Errorf("%s input_schema is not valid JSON: %w", prefix, err)
		}
		if len(raw) > maxToolSchemaLen {
			return fmt.Errorf("%s input_schema exceeds %d bytes", prefix, maxToolSchemaLen)
		}
		if schemaType, ok := tool.InputSchema["type"]; !ok || schemaType != "object" {
			return fmt.Errorf("%s input_schema type must be %q", prefix, "object")
		}
		scopes := make(map[string]struct{}, len(tool.ExecutionScopes))
		for _, scope := range tool.ExecutionScopes {
			scope = strings.TrimSpace(scope)
			if scope != "root" && scope != "child" {
				return fmt.Errorf("%s execution scope %q must be root or child", prefix, scope)
			}
			if _, duplicate := scopes[scope]; duplicate {
				return fmt.Errorf("%s repeats execution scope %q", prefix, scope)
			}
			scopes[scope] = struct{}{}
		}
		if tool.Activity != nil {
			if err := validateBoundedMetadata("activity.risk", tool.Activity.Risk); err != nil {
				return fmt.Errorf("%s: %w", prefix, err)
			}
			if err := validateBoundedMetadata("activity.reason", tool.Activity.Reason); err != nil {
				return fmt.Errorf("%s: %w", prefix, err)
			}
		}
		if tool.Display != nil {
			if strings.TrimSpace(tool.Display.Kind) == "" && strings.TrimSpace(tool.Display.Text) == "" && strings.TrimSpace(tool.Display.Capability) == "" {
				return fmt.Errorf("%s display metadata is empty", prefix)
			}
			for field, value := range map[string]string{
				"display.kind":       tool.Display.Kind,
				"display.text":       tool.Display.Text,
				"display.capability": tool.Display.Capability,
			} {
				if err := validateBoundedMetadata(field, value); err != nil {
					return fmt.Errorf("%s: %w", prefix, err)
				}
			}
		}
	}
	return nil
}

func validateBoundedMetadata(field, value string) error {
	if len(value) > maxToolMetadataLen {
		return fmt.Errorf("%s exceeds %d bytes", field, maxToolMetadataLen)
	}
	if strings.ContainsRune(value, '\x00') {
		return errors.New(field + " contains a NUL byte")
	}
	return nil
}

// InvokeParams carries one interception through the external plugin protocol.
// Input is immutable event context. Output is the mutable value produced by
// earlier plugins in the chain.
type InvokeParams struct {
	Hook   Hook            `json:"hook"`
	Input  json.RawMessage `json:"input"`
	Output json.RawMessage `json:"output"`
}

// InvokeResult returns the next output value in the deterministic plugin chain.
type InvokeResult struct {
	Output json.RawMessage `json:"output"`
}

// ChatRequestInput identifies one provider-neutral model request without
// duplicating the mutable request payload carried in InvokeParams.Output.
type ChatRequestInput struct {
	SessionID string `json:"session_id,omitempty"`
	ThreadID  string `json:"thread_id,omitempty"`
	CWD       string `json:"cwd"`
	Provider  string `json:"provider"`
	StepIndex int    `json:"step_index"`
}

// ChatRequestOutput is the public, provider-neutral model request surface.
// Transport-owned cache and correlation fields stay under Wuu's control.
type ChatRequestOutput struct {
	Model                       string                     `json:"model"`
	Messages                    []providers.ChatMessage    `json:"messages"`
	Tools                       []providers.ToolDefinition `json:"tools,omitempty"`
	Temperature                 float64                    `json:"temperature,omitempty"`
	MaxTokens                   int                        `json:"max_tokens,omitempty"`
	Effort                      string                     `json:"effort,omitempty"`
	ProviderOptions             map[string]any             `json:"provider_options,omitempty"`
	NativeDeferredToolDiscovery bool                       `json:"native_deferred_tool_discovery,omitempty"`
	ForceToolName               string                     `json:"force_tool_name,omitempty"`
}

type ChatMessageInput struct {
	SessionID string `json:"session_id,omitempty"`
	ThreadID  string `json:"thread_id,omitempty"`
	CWD       string `json:"cwd"`
}

type ChatMessageOutput struct {
	Content        string                 `json:"content"`
	DisplayContent string                 `json:"display_content,omitempty"`
	Images         []providers.InputImage `json:"images,omitempty"`
	Files          []providers.InputFile  `json:"files,omitempty"`
}

type ToolExecuteInput struct {
	SessionID string          `json:"session_id,omitempty"`
	ThreadID  string          `json:"thread_id,omitempty"`
	ActorID   string          `json:"actor_id,omitempty"`
	ActorPath string          `json:"actor_path,omitempty"`
	CWD       string          `json:"cwd"`
	StepIndex int             `json:"step_index,omitempty"`
	CallID    string          `json:"call_id"`
	Tool      string          `json:"tool"`
	Arguments json.RawMessage `json:"arguments"`
}

type ToolExecuteBeforeOutput struct {
	Arguments json.RawMessage `json:"arguments"`
}

type ToolExecuteAfterOutput struct {
	Result toolresult.Result `json:"result"`
	Error  string            `json:"error,omitempty"`
}

type State string

const (
	StateStarting State = "starting"
	StateActive   State = "active"
	StateFailed   State = "failed"
	StateStopped  State = "stopped"
)

// Status is a user-safe snapshot of one plugin runtime.
type Status struct {
	ID        string    `json:"id"`
	State     State     `json:"state"`
	Hooks     []Hook    `json:"hooks,omitempty"`
	Error     string    `json:"error,omitempty"`
	StartedAt time.Time `json:"started_at,omitempty"`
}
