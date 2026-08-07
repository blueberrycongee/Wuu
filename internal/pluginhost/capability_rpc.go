package pluginhost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/blueberrycongee/wuu/internal/providers"
)

// CapabilityRPC defines the typed, bidirectional extension protocol between
// the Wuu host and external plugin processes. This upgrades the existing
// hook-based protocol (wuu-plugin-v1) to a full capability RPC where both
// sides can initiate typed calls.
//
// ## Host → Plugin (capability calls)
//
// The host calls plugin-registered capabilities through the existing
// Invoke path. Each capability has a typed input/output contract.
//
// ## Plugin → Host (host service calls)
//
// Plugins call host services over the same bidirectional channel. The
// host exposes a fixed set of services for storage, settings, sub-agent
// spawning, and other controlled operations. Plugins declare required
// services during initialization; the host rejects activation if an
// unsupported service is required.
//
// ## Protocol version
//
// Protocol version 2 adds capability negotiation. Version 1 clients
// are still supported; they default to hook-only mode.
const (
	CapabilityProtocolVersion = 2
	CapabilityProtocolName    = "wuu-plugin-v2"

	// CapabilityAgentRequestTransform lets a plugin transform the complete,
	// provider-neutral request immediately before provider validation and send.
	CapabilityAgentRequestTransform = "agent.request.transform"
	// CapabilityAgentSystemPromptSection contributes a generation-stable section
	// evaluated before that plugin generation can become active.
	CapabilityAgentSystemPromptSection = "agent.system_prompt.section"
	// CapabilityAgentCompaction selects a plugin-owned conversation compactor.
	CapabilityAgentCompaction = "agent.compaction"
	// CapabilityAgentContinuation lets a plugin request another host-owned turn
	// and provide request-only context for that turn.
	CapabilityAgentContinuation = "agent.turn.continuation"
	// CapabilityAgentTurnCompleted observes a settled model turn after history
	// and usage have been persisted. It cannot alter host turn control flow.
	CapabilityAgentTurnCompleted = "agent.turn.completed"
	// CapabilityPluginClientRequest handles a generation-bound opaque request
	// from a Wuu client. Method names and payload schemas belong to the plugin.
	CapabilityPluginClientRequest = "plugin.client.request"
)

// CapabilityNegotiationError marks an initialize response that cannot be
// activated safely. Runtime generation builders treat this as a hard rejection
// even when ordinary plugin startup failures are allowed in inventory.
type CapabilityNegotiationError struct{ Err error }

func (e *CapabilityNegotiationError) Error() string { return e.Err.Error() }
func (e *CapabilityNegotiationError) Unwrap() error { return e.Err }

func IsCapabilityNegotiationError(err error) bool {
	var target *CapabilityNegotiationError
	return errors.As(err, &target)
}

// CapabilityDescriptor declares one capability a plugin provides.
// Capabilities are typed, versioned, and ordered by priority.
type CapabilityDescriptor struct {
	// ID is a stable dotted identifier, e.g. "agent.tool.execute.around".
	ID string `json:"id"`

	// Kind classifies the dispatch semantics (observe/transform/guard/around/decision).
	Kind string `json:"kind"`

	// Version is the capability contract version. Breaking changes increment
	// the major version.
	Version int `json:"version"`

	// Priority determines ordering when multiple plugins provide the
	// same capability. Higher values execute first.
	Priority int `json:"priority,omitempty"`

	// DependsOn lists capability IDs this capability requires from
	// other plugins or the host.
	DependsOn []string `json:"depends_on,omitempty"`

	// Conflicts lists capability IDs that must NOT be present. Used for
	// mutually exclusive capabilities (e.g., two compaction providers).
	Conflicts []string `json:"conflicts,omitempty"`
}

// HostServiceDescriptor declares a host service the plugin wants to use.
// Host services are the Plugin → Host direction of the capability RPC.
type HostServiceDescriptor struct {
	// ID is the service identifier, e.g. "host.storage.get".
	ID string `json:"id"`

	// Required indicates that the plugin cannot activate without this service.
	// Optional services are used if available; required services cause
	// activation failure if the host doesn't support them.
	Required bool `json:"required,omitempty"`
}

// ---------------------------------------------------------------------------
// Host service protocol (Plugin → Host)
// ---------------------------------------------------------------------------

// HostServiceMethod is a stable identifier for a host-exposed service method.
type HostServiceMethod string

const (
	// Storage
	HostServiceStorageGet    HostServiceMethod = "host.storage.get"
	HostServiceStorageSet    HostServiceMethod = "host.storage.set"
	HostServiceStorageDelete HostServiceMethod = "host.storage.delete"
	HostServiceStorageKeys   HostServiceMethod = "host.storage.keys"

	// Settings
	HostServiceSettingsGet  HostServiceMethod = "host.settings.get"
	HostServiceSettingsList HostServiceMethod = "host.settings.list"

	// Child sessions. Product-specific orchestration semantics remain in the
	// calling plugin; the host only exposes a neutral request dispatcher.
	HostServiceChildSessionRequest HostServiceMethod = "host.child_session.request"

	// Session
	HostServiceSessionGetInfo HostServiceMethod = "host.session.info"

	// Workspace
	HostServiceWorkspaceGetRoot HostServiceMethod = "host.workspace.root"
	HostServiceWorkspaceList    HostServiceMethod = "host.workspace.list"

	// Diagnostics
	HostServiceDiagnosticsLog HostServiceMethod = "host.diagnostics.log"
)

type ChildSessionRequestParams struct {
	Action    string          `json:"action"`
	ActorID   string          `json:"actor_id,omitempty"`
	ActorPath string          `json:"actor_path,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
}

// HostServiceCall is a Plugin → Host RPC call.
type HostServiceCall struct {
	// ID is a caller-chosen correlation identifier.
	ID string `json:"id"`

	// Method is the host service being called.
	Method HostServiceMethod `json:"method"`

	// Params carries method-specific arguments.
	Params json.RawMessage `json:"params,omitempty"`
}

// HostServiceResult is the Host → Plugin response.
type HostServiceResult struct {
	// ID matches the originating HostServiceCall.ID.
	ID string `json:"id"`

	// Result carries method-specific return data.
	Result json.RawMessage `json:"result,omitempty"`

	// Error is non-nil when the call failed.
	Error *HostServiceError `json:"error,omitempty"`
}

// PluginClientRequestInput is a product-neutral client-to-plugin envelope.
type PluginClientRequestInput struct {
	Method string          `json:"method"`
	Input  json.RawMessage `json:"input,omitempty"`
}

// PluginClientRequestOutput carries one plugin-owned JSON result.
type PluginClientRequestOutput struct {
	Result json.RawMessage `json:"result,omitempty"`
}

const (
	ContinuationPhaseProbe   = "probe"
	ContinuationPhasePrepare = "prepare"
)

// AgentContinuationInput asks a plugin whether it owns pending continuation
// work for one thread. Prepare is called only after the host holds the thread's
// execution lease and must revalidate the earlier probe.
type AgentContinuationInput struct {
	ThreadID string `json:"thread_id"`
	Phase    string `json:"phase"`
}

// AgentContinuationBlock is request-only model context owned by the plugin.
type AgentContinuationBlock struct {
	Kind    string `json:"kind"`
	Title   string `json:"title,omitempty"`
	Source  string `json:"source,omitempty"`
	Content string `json:"content"`
}

// AgentContinuationOutput requests a turn and, during prepare, supplies the
// context that explains the plugin-owned work to the model.
type AgentContinuationOutput struct {
	Continue bool                      `json:"continue"`
	Blocks   []AgentContinuationBlock  `json:"blocks,omitempty"`
	Display  *AgentContinuationDisplay `json:"display,omitempty"`
}

// AgentContinuationDisplay is an optional read-only user-query presentation
// for the requested turn. The host renders it but never adds it to model context.
type AgentContinuationDisplay struct {
	Text string `json:"text"`
	Name string `json:"name,omitempty"`
}

// AgentTurnCompletedInput is a product-neutral settled-turn observation.
type AgentTurnCompletedInput struct {
	ThreadID     string    `json:"thread_id"`
	TurnID       string    `json:"turn_id"`
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  time.Time `json:"completed_at"`
	Succeeded    bool      `json:"succeeded"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
}

type AgentTurnCompletedOutput struct{}

// HostServiceError describes a host service call failure.
type HostServiceError struct {
	// Code is a machine-readable error code.
	Code string `json:"code"`

	// Message is a human-readable description.
	Message string `json:"message"`
}

func (e *HostServiceError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

// HostServiceHandler dispatches the host services that are live for one
// plugin process. SupportedHostServices is both the dispatcher's declaration
// and the source of the services advertised during initialization.
// HandleHostService must return promptly when ctx is canceled.
type HostServiceHandler interface {
	SupportedHostServices() []HostServiceMethod
	HandleHostService(ctx context.Context, method HostServiceMethod, params json.RawMessage) (json.RawMessage, error)
}

// HostServiceLifecycle is implemented by generation-owned dispatchers that
// must stop accepting calls when their plugin process closes.
type HostServiceLifecycle interface {
	CloseHostServices()
}

// ---------------------------------------------------------------------------
// Host service parameter/result types
// ---------------------------------------------------------------------------

// StorageGetParams is the input for host.storage.get.
type StorageGetParams struct {
	Scope string `json:"scope"`
	Key   string `json:"key"`
}

// StorageGetResult is the output of host.storage.get.
type StorageGetResult struct {
	Value *string `json:"value"` // nil when key doesn't exist
}

// StorageSetParams is the input for host.storage.set.
type StorageSetParams struct {
	Scope string `json:"scope"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

// StorageDeleteParams is the input for host.storage.delete.
type StorageDeleteParams struct {
	Scope string `json:"scope"`
	Key   string `json:"key"`
}

// StorageKeysParams is the input for host.storage.keys.
type StorageKeysParams struct {
	Scope string `json:"scope"`
}

// StorageKeysResult is the output of host.storage.keys.
type StorageKeysResult struct {
	Keys []string `json:"keys"`
}

// SettingsGetParams is the input for host.settings.get.
type SettingsGetParams struct {
	Key string `json:"key"`
}

// SettingsGetResult is the output of host.settings.get.
type SettingsGetResult struct {
	Value json.RawMessage `json:"value"`
}

// SettingsListResult is the output of host.settings.list.
type SettingsListResult struct {
	Entries map[string]json.RawMessage `json:"entries"`
}

// SessionInfoResult is the output of host.session.info.
type SessionInfoResult struct {
	SessionID string `json:"session_id"`
	ThreadID  string `json:"thread_id,omitempty"`
	CWD       string `json:"cwd"`
	Model     string `json:"model"`
}

// WorkspaceRootResult is the output of host.workspace.root.
type WorkspaceRootResult struct {
	Root string `json:"root"`
}

// WorkspaceListResult is the output of host.workspace.list.
type WorkspaceListResult struct {
	Workspaces []WorkspaceInfo `json:"workspaces"`
}

// WorkspaceInfo describes one registered workspace.
type WorkspaceInfo struct {
	ID   string `json:"id"`
	Root string `json:"root"`
	Name string `json:"name,omitempty"`
}

// ---------------------------------------------------------------------------
// Capability negotiation
// ---------------------------------------------------------------------------

// CapabilityInitializeResult extends InitializeResult with capability
// negotiation for protocol v2+. Plugins declare which capabilities they
// provide and which host services they require.
type CapabilityInitializeResult struct {
	InitializeResult

	// Capabilities lists the capabilities this plugin provides.
	Capabilities []CapabilityDescriptor `json:"capabilities,omitempty"`

	// RequiredHostServices lists host services this plugin needs.
	// Activation fails if any required service is unavailable.
	RequiredHostServices []HostServiceDescriptor `json:"required_host_services,omitempty"`

	// ProtocolVersion is the capability protocol version the plugin
	// requests. The host may downgrade to v1 if v2 is unsupported.
	ProtocolVersion int `json:"protocol_version,omitempty"`
}

// CapabilityInitializeParams extends InitializeParams with the host's
// capability RPC support declaration.
type CapabilityInitializeParams struct {
	InitializeParams

	// CapabilityProtocolVersion offers the capability layer independently of
	// the v1 transport version. Legacy plugins keep seeing protocol_version=1
	// and ignore this additive field.
	CapabilityProtocolVersion int `json:"capability_protocol_version,omitempty"`

	// SupportedHostServices lists the host services available to this plugin.
	// Plugins can use this to decide whether to enable optional features.
	SupportedHostServices []HostServiceMethod `json:"supported_host_services,omitempty"`
}

// CapabilityInvokeParams carries one typed capability invocation. Input and
// output are both present so transform capabilities can build on the value
// produced by an earlier, higher-priority plugin.
type CapabilityInvokeParams struct {
	Capability string          `json:"capability"`
	Input      json.RawMessage `json:"input"`
	Output     json.RawMessage `json:"output"`
}

// CapabilityInvokeResult is the transformed value returned by a plugin.
type CapabilityInvokeResult struct {
	Output json.RawMessage `json:"output"`
}

// RequestTransformInput is immutable context for agent.request.transform.
type RequestTransformInput struct {
	SessionID string `json:"session_id,omitempty"`
	ThreadID  string `json:"thread_id,omitempty"`
	CWD       string `json:"cwd,omitempty"`
	Provider  string `json:"provider,omitempty"`
	StepIndex int    `json:"step_index"`
}

// RequestTransformOutput is the mutable provider-neutral request contract.
type RequestTransformOutput struct {
	Request providers.ChatRequest `json:"request"`
}

// SystemPromptSectionInput is immutable session context for the v1
// agent.system_prompt.section contract.
type SystemPromptSectionInput struct {
	CWD      string `json:"cwd"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// SystemPromptSectionOutput is one generation-stable prompt section.
type SystemPromptSectionOutput struct {
	Text string `json:"text"`
}

// CompactionInput is immutable request context for the v1 agent.compaction contract.
type CompactionInput struct {
	Model    string                  `json:"model"`
	Messages []providers.ChatMessage `json:"messages"`
}

// CompactionOutput is the replacement transcript returned by a compactor.
type CompactionOutput struct {
	Messages []providers.ChatMessage `json:"messages"`
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

// allowedCapabilityKinds is the closed set of valid capability dispatch kinds.
var allowedCapabilityKinds = map[string]bool{
	"observe":   true,
	"transform": true,
	"guard":     true,
	"around":    true,
	"decision":  true,
}

// ValidateCapabilityDescriptor checks that a capability descriptor is well-formed.
func ValidateCapabilityDescriptor(c CapabilityDescriptor) error {
	id := strings.TrimSpace(c.ID)
	if id == "" {
		return fmt.Errorf("capability id is required")
	}
	if !strings.Contains(id, ".") {
		return fmt.Errorf("capability id %q must be a dotted identifier", id)
	}
	kind := strings.TrimSpace(c.Kind)
	if kind == "" {
		return fmt.Errorf("capability %s: kind is required", id)
	}
	if !allowedCapabilityKinds[kind] {
		return fmt.Errorf("capability %s: unknown kind %q (valid: observe, transform, guard, around, decision)", id, kind)
	}
	if c.Version < 1 {
		return fmt.Errorf("capability %s: version must be >= 1, got %d", id, c.Version)
	}
	requiredKind := ""
	switch c.ID {
	case CapabilityAgentRequestTransform:
		requiredKind = "transform"
	case CapabilityAgentSystemPromptSection:
		requiredKind = "transform"
	case CapabilityAgentCompaction:
		requiredKind = "decision"
	case CapabilityAgentContinuation:
		requiredKind = "decision"
	case CapabilityAgentTurnCompleted:
		requiredKind = "observe"
	case CapabilityPluginClientRequest:
		requiredKind = "decision"
	}
	if requiredKind != "" && (c.Kind != requiredKind || c.Version != 1) {
		return fmt.Errorf("capability %s requires kind %q and version 1", id, requiredKind)
	}
	if err := validateCapabilityRelations(c); err != nil {
		return err
	}
	return nil
}

func validateCapabilityRelations(c CapabilityDescriptor) error {
	dependencies := make(map[string]struct{}, len(c.DependsOn))
	for _, dependency := range c.DependsOn {
		dependency = strings.TrimSpace(dependency)
		if dependency == "" || !strings.Contains(dependency, ".") {
			return fmt.Errorf("capability %s: dependency %q must be a dotted identifier", c.ID, dependency)
		}
		if dependency == c.ID {
			return fmt.Errorf("capability %s cannot depend on itself", c.ID)
		}
		if _, exists := dependencies[dependency]; exists {
			return fmt.Errorf("capability %s: duplicate dependency %q", c.ID, dependency)
		}
		dependencies[dependency] = struct{}{}
	}
	conflicts := make(map[string]struct{}, len(c.Conflicts))
	for _, conflict := range c.Conflicts {
		conflict = strings.TrimSpace(conflict)
		if conflict == "" || !strings.Contains(conflict, ".") {
			return fmt.Errorf("capability %s: conflict %q must be a dotted identifier", c.ID, conflict)
		}
		if conflict == c.ID {
			return fmt.Errorf("capability %s cannot conflict with itself", c.ID)
		}
		if _, exists := conflicts[conflict]; exists {
			return fmt.Errorf("capability %s: duplicate conflict %q", c.ID, conflict)
		}
		if _, exists := dependencies[conflict]; exists {
			return fmt.Errorf("capability %s cannot both depend on and conflict with %q", c.ID, conflict)
		}
		conflicts[conflict] = struct{}{}
	}
	return nil
}

// ValidateCapabilityNegotiation validates one initialize response against the
// services actually implemented by the host process.
func ValidateCapabilityNegotiation(result CapabilityInitializeResult, supported []HostServiceMethod) error {
	version := result.ProtocolVersion
	if version == 0 {
		version = ProtocolVersion
	}
	if version < ProtocolVersion || version > CapabilityProtocolVersion {
		return fmt.Errorf("unsupported negotiated protocol version %d", version)
	}
	if version == ProtocolVersion {
		if len(result.Capabilities) != 0 || len(result.RequiredHostServices) != 0 {
			return errors.New("protocol v1 cannot declare capabilities or host services")
		}
		return nil
	}

	seenCapabilities := make(map[string]struct{}, len(result.Capabilities))
	for _, capability := range result.Capabilities {
		if err := ValidateCapabilityDescriptor(capability); err != nil {
			return err
		}
		switch capability.ID {
		case CapabilityAgentRequestTransform, CapabilityAgentSystemPromptSection, CapabilityAgentCompaction, CapabilityAgentContinuation, CapabilityAgentTurnCompleted, CapabilityPluginClientRequest:
		default:
			return fmt.Errorf("capability %s is not supported by this host", capability.ID)
		}
		if _, exists := seenCapabilities[capability.ID]; exists {
			return fmt.Errorf("duplicate capability %q", capability.ID)
		}
		seenCapabilities[capability.ID] = struct{}{}
	}

	available := make(map[HostServiceMethod]struct{}, len(supported))
	for _, service := range supported {
		if err := ValidateHostServiceMethod(service); err != nil {
			return fmt.Errorf("host configured invalid service: %w", err)
		}
		available[service] = struct{}{}
	}
	seenServices := make(map[HostServiceMethod]struct{}, len(result.RequiredHostServices))
	for _, descriptor := range result.RequiredHostServices {
		service := HostServiceMethod(strings.TrimSpace(descriptor.ID))
		if service == "" {
			return errors.New("host service id is required")
		}
		if _, exists := seenServices[service]; exists {
			return fmt.Errorf("duplicate host service %q", service)
		}
		seenServices[service] = struct{}{}
		_, availableNow := available[service]
		if err := ValidateHostServiceMethod(service); err != nil {
			if descriptor.Required {
				return fmt.Errorf("required host service %q is unsupported", service)
			}
			continue
		}
		if descriptor.Required && !availableNow {
			return fmt.Errorf("required host service %q is unavailable", service)
		}
	}
	return nil
}

// ValidateHostServiceMethod checks that a host service method is recognized.
func ValidateHostServiceMethod(m HostServiceMethod) error {
	switch m {
	case HostServiceStorageGet, HostServiceStorageSet, HostServiceStorageDelete, HostServiceStorageKeys,
		HostServiceSettingsGet, HostServiceSettingsList,
		HostServiceChildSessionRequest,
		HostServiceSessionGetInfo,
		HostServiceWorkspaceGetRoot, HostServiceWorkspaceList,
		HostServiceDiagnosticsLog:
		return nil
	default:
		return fmt.Errorf("unknown host service method %q", m)
	}
}

// AllHostServices returns the complete list of supported host services.
func AllHostServices() []HostServiceMethod {
	return []HostServiceMethod{
		HostServiceStorageGet, HostServiceStorageSet, HostServiceStorageDelete, HostServiceStorageKeys,
		HostServiceSettingsGet, HostServiceSettingsList,
		HostServiceChildSessionRequest,
		HostServiceSessionGetInfo,
		HostServiceWorkspaceGetRoot, HostServiceWorkspaceList,
		HostServiceDiagnosticsLog,
	}
}

func cloneCapabilityDescriptors(capabilities []CapabilityDescriptor) []CapabilityDescriptor {
	cloned := make([]CapabilityDescriptor, len(capabilities))
	for index, capability := range capabilities {
		cloned[index] = capability
		cloned[index].DependsOn = append([]string(nil), capability.DependsOn...)
		cloned[index].Conflicts = append([]string(nil), capability.Conflicts...)
	}
	return cloned
}
