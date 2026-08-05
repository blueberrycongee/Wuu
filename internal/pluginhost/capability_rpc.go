package pluginhost

import (
	"encoding/json"
	"fmt"
	"strings"
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
)

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

	// Sub-agent
	HostServiceSubagentSpawn  HostServiceMethod = "host.subagent.spawn"
	HostServiceSubagentStatus HostServiceMethod = "host.subagent.status"

	// Session
	HostServiceSessionGetInfo HostServiceMethod = "host.session.info"

	// Workspace
	HostServiceWorkspaceGetRoot HostServiceMethod = "host.workspace.root"
	HostServiceWorkspaceList    HostServiceMethod = "host.workspace.list"

	// Diagnostics
	HostServiceDiagnosticsLog HostServiceMethod = "host.diagnostics.log"
)

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

// HostServiceError describes a host service call failure.
type HostServiceError struct {
	// Code is a machine-readable error code.
	Code string `json:"code"`

	// Message is a human-readable description.
	Message string `json:"message"`
}

// ---------------------------------------------------------------------------
// Host service parameter/result types
// ---------------------------------------------------------------------------

// StorageGetParams is the input for host.storage.get.
type StorageGetParams struct {
	Key string `json:"key"`
}

// StorageGetResult is the output of host.storage.get.
type StorageGetResult struct {
	Value *string `json:"value"` // nil when key doesn't exist
}

// StorageSetParams is the input for host.storage.set.
type StorageSetParams struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// StorageDeleteParams is the input for host.storage.delete.
type StorageDeleteParams struct {
	Key string `json:"key"`
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

// SubagentSpawnParams is the input for host.subagent.spawn.
type SubagentSpawnParams struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
	Model       string `json:"model,omitempty"`
}

// SubagentSpawnResult is the output of host.subagent.spawn.
type SubagentSpawnResult struct {
	AgentID string `json:"agent_id"`
}

// SubagentStatusResult is the output of host.subagent.status.
type SubagentStatusResult struct {
	AgentID string `json:"agent_id"`
	Status  string `json:"status"` // "running", "completed", "failed", "cancelled"
	Result  string `json:"result,omitempty"`
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

	// SupportedHostServices lists the host services available to this plugin.
	// Plugins can use this to decide whether to enable optional features.
	SupportedHostServices []HostServiceMethod `json:"supported_host_services,omitempty"`
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

// allowedCapabilityKinds is the closed set of valid capability dispatch kinds.
var allowedCapabilityKinds = map[string]bool{
	"observe":  true,
	"transform": true,
	"guard":    true,
	"around":   true,
	"decision": true,
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
	return nil
}

// ValidateHostServiceMethod checks that a host service method is recognized.
func ValidateHostServiceMethod(m HostServiceMethod) error {
	switch m {
	case HostServiceStorageGet, HostServiceStorageSet, HostServiceStorageDelete, HostServiceStorageKeys,
		HostServiceSettingsGet, HostServiceSettingsList,
		HostServiceSubagentSpawn, HostServiceSubagentStatus,
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
		HostServiceSubagentSpawn, HostServiceSubagentStatus,
		HostServiceSessionGetInfo,
		HostServiceWorkspaceGetRoot, HostServiceWorkspaceList,
		HostServiceDiagnosticsLog,
	}
}
