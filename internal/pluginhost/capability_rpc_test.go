package pluginhost

import (
	"strings"
	"testing"
)

func TestValidateCapabilityDescriptor(t *testing.T) {
	tests := []struct {
		name    string
		desc    CapabilityDescriptor
		wantErr bool
	}{
		{
			name:    "empty id",
			desc:    CapabilityDescriptor{Kind: "transform", Version: 1},
			wantErr: true,
		},
		{
			name:    "no dot in id",
			desc:    CapabilityDescriptor{ID: "tool", Kind: "transform", Version: 1},
			wantErr: true,
		},
		{
			name:    "host-only capability",
			desc:    CapabilityDescriptor{ID: "host.plugin.install", Kind: SeamDecision, Version: 1},
			wantErr: true,
		},
		{
			name:    "empty kind",
			desc:    CapabilityDescriptor{ID: "agent.tool.execute", Version: 1},
			wantErr: true,
		},
		{
			name:    "unknown kind",
			desc:    CapabilityDescriptor{ID: "agent.tool.execute", Kind: "unknown", Version: 1},
			wantErr: true,
		},
		{
			name:    "zero version",
			desc:    CapabilityDescriptor{ID: "agent.tool.execute", Kind: "transform", Version: 0},
			wantErr: true,
		},
		{
			name:    "valid observe",
			desc:    CapabilityDescriptor{ID: "agent.session.lifecycle", Kind: "observe", ErrorPolicy: ErrorPolicyIsolate, Version: 1},
			wantErr: false,
		},
		{
			name:    "valid transform",
			desc:    CapabilityDescriptor{ID: "agent.tool.execute.after", Kind: "transform", Version: 2, Priority: 10},
			wantErr: false,
		},
		{
			name:    "guard is not implemented",
			desc:    CapabilityDescriptor{ID: "agent.permission.policy", Kind: "guard", Version: 1},
			wantErr: true,
		},
		{
			name:    "around is not implemented",
			desc:    CapabilityDescriptor{ID: "agent.tool.execute.around", Kind: "around", Version: 1},
			wantErr: true,
		},
		{
			name:    "valid decision",
			desc:    CapabilityDescriptor{ID: "agent.compaction", Kind: "decision", Version: 1},
			wantErr: false,
		},
		{
			name:    "system prompt requires transform v1",
			desc:    CapabilityDescriptor{ID: CapabilityAgentSystemPromptSection, Kind: "observe", Version: 1},
			wantErr: true,
		},
		{
			name:    "compaction requires decision v1",
			desc:    CapabilityDescriptor{ID: CapabilityAgentCompaction, Kind: "decision", Version: 2},
			wantErr: true,
		},
		{
			name:    "observe cannot propagate",
			desc:    CapabilityDescriptor{ID: "agent.observe.custom", Kind: SeamObserve, ErrorPolicy: ErrorPolicyPropagate, Version: 1},
			wantErr: true,
		},
		{
			name:    "ignore only valid for observe",
			desc:    CapabilityDescriptor{ID: "agent.transform.custom", Kind: SeamTransform, ErrorPolicy: ErrorPolicyIgnore, Version: 1},
			wantErr: true,
		},
		{
			name:    "turn completed defaults to isolate",
			desc:    CapabilityDescriptor{ID: CapabilityAgentTurnCompleted, Kind: SeamObserve, Version: 1},
			wantErr: false,
		},
		{
			name: "with dependencies",
			desc: CapabilityDescriptor{
				ID: "agent.tool.custom", Kind: "transform", Version: 1,
				DependsOn: []string{"agent.tool.register"},
			},
			wantErr: false,
		},
		{
			name: "with conflicts",
			desc: CapabilityDescriptor{
				ID: "agent.compaction.custom", Kind: "decision", Version: 1,
				Conflicts: []string{"agent.compaction.default"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCapabilityDescriptor(tt.desc)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCapabilityDescriptor() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateHostServiceMethod(t *testing.T) {
	validMethods := AllHostServices()
	for _, m := range validMethods {
		if err := ValidateHostServiceMethod(m); err != nil {
			t.Errorf("ValidateHostServiceMethod(%s) unexpected error: %v", m, err)
		}
	}

	invalidMethods := []HostServiceMethod{
		"host.unknown",
		"agent.tool.execute",
		"",
	}
	for _, m := range invalidMethods {
		if err := ValidateHostServiceMethod(m); err == nil {
			t.Errorf("ValidateHostServiceMethod(%s) expected error, got nil", m)
		}
	}
}

func TestAllHostServicesComplete(t *testing.T) {
	services := AllHostServices()
	if len(services) == 0 {
		t.Fatal("expected non-empty host services list")
	}

	// Each host service method should start with "host."
	for _, s := range services {
		if string(s[:5]) != "host." {
			t.Errorf("host service method %s should start with 'host.'", s)
		}
	}

	// Verify no duplicates.
	seen := make(map[HostServiceMethod]bool)
	for _, s := range services {
		if seen[s] {
			t.Errorf("duplicate host service method: %s", s)
		}
		seen[s] = true
	}
}

func TestCapabilityInitializeResultProtocolVersion(t *testing.T) {
	// Protocol v2 should carry capability information.
	result := CapabilityInitializeResult{
		InitializeResult: InitializeResult{},
		Capabilities: []CapabilityDescriptor{
			{ID: "agent.tool.custom", Kind: "transform", Version: 1},
		},
		RequiredHostServices: []HostServiceDescriptor{
			{ID: "host.storage.get", Required: true},
		},
		ProtocolVersion: 2,
	}

	if result.ProtocolVersion != 2 {
		t.Errorf("expected protocol version 2, got %d", result.ProtocolVersion)
	}
	if len(result.Capabilities) != 1 {
		t.Errorf("expected 1 capability, got %d", len(result.Capabilities))
	}
	if len(result.RequiredHostServices) != 1 {
		t.Errorf("expected 1 host service, got %d", len(result.RequiredHostServices))
	}
}

func TestCapabilityInitializeParams(t *testing.T) {
	params := CapabilityInitializeParams{
		InitializeParams: InitializeParams{
			ProtocolVersion: ProtocolVersion,
			PluginID:        "test-plugin",
		},
		CapabilityProtocolVersion: CapabilityProtocolVersion,
		SupportedHostServices: []HostServiceMethod{
			HostServiceStorageGet,
			HostServiceStorageSet,
		},
	}

	if params.ProtocolVersion != ProtocolVersion || params.CapabilityProtocolVersion != CapabilityProtocolVersion {
		t.Errorf("protocol offer = transport %d capability %d", params.ProtocolVersion, params.CapabilityProtocolVersion)
	}
	if len(params.SupportedHostServices) != 2 {
		t.Errorf("expected 2 supported services, got %d", len(params.SupportedHostServices))
	}
}

func TestValidateCapabilityNegotiationHostServices(t *testing.T) {
	base := CapabilityInitializeResult{ProtocolVersion: CapabilityProtocolVersion}
	base.Capabilities = []CapabilityDescriptor{
		{ID: CapabilityAgentRequestTransform, Kind: "transform", Version: 1},
		{ID: CapabilityAgentSystemPromptSection, Kind: "transform", Version: 1},
		{ID: CapabilityAgentCompaction, Kind: "decision", Version: 1},
		{ID: CapabilityPluginClientRequest, Kind: "decision", Version: 1},
	}

	optional := base
	optional.RequiredHostServices = []HostServiceDescriptor{{ID: "host.future.optional"}}
	if err := ValidateCapabilityNegotiation(optional, nil); err != nil {
		t.Fatalf("optional unsupported service rejected: %v", err)
	}

	required := base
	required.RequiredHostServices = []HostServiceDescriptor{{ID: string(HostServiceStorageGet), Required: true}}
	if err := ValidateCapabilityNegotiation(required, nil); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("required unavailable service error = %v", err)
	}
	if err := ValidateCapabilityNegotiation(required, []HostServiceMethod{HostServiceStorageGet}); err != nil {
		t.Fatalf("available required service rejected: %v", err)
	}
}

func TestValidateCapabilityNegotiationRejectsUnsupportedAndDuplicateDeclarations(t *testing.T) {
	tests := []struct {
		name   string
		result CapabilityInitializeResult
		match  string
	}{
		{
			name: "unsupported capability",
			result: CapabilityInitializeResult{ProtocolVersion: 2, Capabilities: []CapabilityDescriptor{{
				ID: "agent.future.transform", Kind: "transform", Version: 1,
			}}},
			match: "not supported",
		},
		{
			name: "duplicate capability",
			result: CapabilityInitializeResult{ProtocolVersion: 2, Capabilities: []CapabilityDescriptor{
				{ID: CapabilityAgentRequestTransform, Kind: "transform", Version: 1},
				{ID: CapabilityAgentRequestTransform, Kind: "transform", Version: 1},
			}},
			match: "duplicate capability",
		},
		{
			name: "v1 declaration",
			result: CapabilityInitializeResult{ProtocolVersion: 1, Capabilities: []CapabilityDescriptor{{
				ID: CapabilityAgentRequestTransform, Kind: "transform", Version: 1,
			}}},
			match: "protocol v1",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateCapabilityNegotiation(test.result, nil)
			if err == nil || !strings.Contains(err.Error(), test.match) {
				t.Fatalf("error = %v, want %q", err, test.match)
			}
		})
	}
}

func TestHostServiceError(t *testing.T) {
	err := HostServiceError{
		Code:    "NOT_FOUND",
		Message: "key does not exist",
	}
	if err.Code != "NOT_FOUND" {
		t.Errorf("expected code NOT_FOUND, got %s", err.Code)
	}
	if err.Message != "key does not exist" {
		t.Errorf("unexpected message: %s", err.Message)
	}
}

func TestStorageParams(t *testing.T) {
	// Verify storage param types are well-formed.
	getParams := StorageGetParams{Scope: "workspace", Key: "my-key"}
	if getParams.Scope != "workspace" || getParams.Key != "my-key" {
		t.Errorf("unexpected get params: %+v", getParams)
	}

	setParams := StorageSetParams{Scope: "user", Key: "k", Value: "v"}
	if setParams.Scope != "user" || setParams.Key != "k" || setParams.Value != "v" {
		t.Error("storage set params mismatch")
	}

	deleteParams := StorageDeleteParams{Scope: "workspace", Key: "k"}
	if deleteParams.Scope != "workspace" || deleteParams.Key != "k" {
		t.Error("storage delete params mismatch")
	}
	keysParams := StorageKeysParams{Scope: "user"}
	if keysParams.Scope != "user" {
		t.Error("storage keys scope mismatch")
	}

	result := StorageGetResult{Value: nil}
	if result.Value != nil {
		t.Error("expected nil value for missing key")
	}

	val := "hello"
	result.Value = &val
	if *result.Value != "hello" {
		t.Error("unexpected value")
	}
}
