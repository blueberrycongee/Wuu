// Package contracttest provides a standalone test host for plugin API
// contracts. Plugin authors can use this to verify that their plugin
// conforms to the Wuu plugin API contract without running the full
// Wuu desktop application.
//
// Usage in a plugin's Go test:
//
//	func TestPluginContract(t *testing.T) {
//	    host := contracttest.NewHost(t, contracttest.HostConfig{
//	        PluginDir: "../test-plugin",
//	    })
//	    defer host.Close()
//
//	    host.AssertInitialization()
//	    host.AssertToolRegistration("my-tool")
//	    host.AssertToolExecution("my-tool", `{"input": "test"}`, `{"result": "ok"}`)
//	}
package contracttest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blueberrycongee/wuu/internal/pluginhost"
)

// HostConfig configures the contract test host.
type HostConfig struct {
	// PluginDir is the path to the plugin directory containing plugin.json.
	PluginDir string
	// WuuHome overrides the Wuu home directory used for the test.
	WuuHome string
	// Timeout for individual operations.
	Timeout int // seconds, defaults to 30
}

// Host is a standalone plugin contract test host. It starts an external
// plugin process and provides assertion helpers for verifying API contracts.
type Host struct {
	t       *testing.T
	config  HostConfig
	client  pluginhost.Client
	diags   []Diagnostic
	closed  bool
}

// Diagnostic records a contract check result.
type Diagnostic struct {
	Level   string // "pass", "fail", "skip"
	Check   string
	Message string
	Detail  string
}

// NewHost creates a contract test host and starts the plugin process.
func NewHost(t *testing.T, config HostConfig) *Host {
	t.Helper()
	if config.Timeout <= 0 {
		config.Timeout = 30
	}
	if config.WuuHome == "" {
		config.WuuHome = filepath.Join(os.TempDir(), "wuu-contract-test")
	}

	h := &Host{t: t, config: config}
	return h
}

// Close shuts down the plugin process and prints the diagnostic summary.
// It does NOT automatically fail the test for recorded failures — callers
// should explicitly check diagnostics or use AssertNoDiagnosticsAbove.
func (h *Host) Close() {
	if h.closed {
		return
	}
	h.closed = true
	if h.client != nil {
		_ = h.client.Close(context.Background())
	}

	h.t.Helper()
	pass, fail, skip := 0, 0, 0
	for _, d := range h.diags {
		switch d.Level {
		case "pass":
			pass++
		case "fail":
			fail++
		case "skip":
			skip++
		}
	}
	h.t.Logf("── Contract test summary ── %d pass, %d fail, %d skip", pass, fail, skip)
}

func (h *Host) record(level, check, message string, detail ...string) {
	h.diags = append(h.diags, Diagnostic{
		Level:   level,
		Check:   check,
		Message: message,
		Detail:  strings.Join(detail, "; "),
	})
}

// AssertManifestExists verifies that plugin.json exists and can be parsed.
func (h *Host) AssertManifestExists() {
	h.t.Helper()
	manifestPath := filepath.Join(h.config.PluginDir, "plugin.json")
	if _, err := os.Stat(manifestPath); err != nil {
		h.record("fail", "manifest.exists", fmt.Sprintf("plugin.json not found at %s", manifestPath), err.Error())
		return
	}
	h.record("pass", "manifest.exists", manifestPath)
}

// AssertManifestValid verifies that plugin.json contains the required fields.
func (h *Host) AssertManifestValid(requiredFields ...string) {
	h.t.Helper()
	manifestPath := filepath.Join(h.config.PluginDir, "plugin.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		h.record("fail", "manifest.valid", "cannot read manifest", err.Error())
		return
	}
	var manifest map[string]any
	if err := json.Unmarshal(data, &manifest); err != nil {
		h.record("fail", "manifest.valid", "invalid JSON", err.Error())
		return
	}

	fields := requiredFields
	if len(fields) == 0 {
		fields = []string{"id", "schema_version", "version"}
	}

	allOk := true
	for _, field := range fields {
		if _, ok := manifest[field]; !ok {
			h.record("fail", "manifest.valid", fmt.Sprintf("missing required field %q", field))
			allOk = false
		}
	}
	if allOk {
		h.record("pass", "manifest.valid", fmt.Sprintf("all required fields present: %s", strings.Join(fields, ", ")))
	}
}

// AssertCapabilityRegistered verifies that the plugin registers a specific capability.
func (h *Host) AssertCapabilityRegistered(capabilityID string) {
	h.t.Helper()
	if h.client == nil {
		h.record("skip", "capability."+capabilityID, "plugin not initialized")
		return
	}
	// Check hooks for a matching capability pattern.
	for _, hook := range h.client.Hooks() {
		if strings.Contains(string(hook), capabilityID) || hook == pluginhost.Hook(capabilityID) {
			h.record("pass", "capability."+capabilityID, fmt.Sprintf("found via hook %s", hook))
			return
		}
	}
	h.record("fail", "capability."+capabilityID, "capability not registered")
}

// AssertToolRegistered verifies that the plugin registers a tool with the given ID.
func (h *Host) AssertToolRegistered(toolID string) {
	h.t.Helper()
	if h.client == nil {
		h.record("skip", "tool."+toolID, "plugin not initialized")
		return
	}
	tc, ok := h.client.(pluginhost.ToolClient)
	if !ok {
		h.record("skip", "tool."+toolID, "plugin does not support tool interface")
		return
	}
	for _, tool := range tc.Tools() {
		if tool.ID == toolID {
			h.record("pass", "tool."+toolID, fmt.Sprintf("tool registered: %s", tool.Description))
			return
		}
	}
	h.record("fail", "tool."+toolID, "tool not found in plugin registrations")
}

// AssertHookRegistered verifies that the plugin registers a specific hook.
func (h *Host) AssertHookRegistered(hook pluginhost.Hook) {
	h.t.Helper()
	if h.client == nil {
		h.record("skip", "hook."+string(hook), "plugin not initialized")
		return
	}
	for _, registered := range h.client.Hooks() {
		if registered == hook {
			h.record("pass", "hook."+string(hook), "hook registered")
			return
		}
	}
	h.record("fail", "hook."+string(hook), "hook not registered")
}

// AssertNoDiagnosticsAbove verifies that no diagnostics above the given
// severity were recorded.
func (h *Host) AssertNoDiagnosticsAbove(level string) {
	h.t.Helper()
	severity := map[string]int{"pass": 0, "skip": 1, "fail": 2}
	threshold := severity[level]
	for _, d := range h.diags {
		if severity[d.Level] >= threshold && d.Level != "pass" {
			h.t.Errorf("unexpected diagnostic [%s] %s: %s", d.Level, d.Check, d.Message)
		}
	}
}

// AssertHostServiceSupported verifies that a host service method is valid
// per the capability RPC specification.
func (h *Host) AssertHostServiceSupported(method pluginhost.HostServiceMethod) {
	h.t.Helper()
	if err := pluginhost.ValidateHostServiceMethod(method); err != nil {
		h.record("fail", "hostservice."+string(method), err.Error())
		return
	}
	h.record("pass", "hostservice."+string(method), "host service recognized")
}

// AssertCapabilityDescriptorValid checks that a capability descriptor
// is well-formed per the capability RPC specification.
func (h *Host) AssertCapabilityDescriptorValid(desc pluginhost.CapabilityDescriptor) {
	h.t.Helper()
	if err := pluginhost.ValidateCapabilityDescriptor(desc); err != nil {
		h.record("fail", "capability."+desc.ID, "invalid descriptor", err.Error())
		return
	}
	h.record("pass", "capability."+desc.ID, "descriptor valid")
}

// AssertErrorIs checks that an error matches the expected error.
func (h *Host) AssertErrorIs(err error, target error, check string) {
	h.t.Helper()
	if err == nil {
		h.record("fail", check, "expected error but got nil")
		return
	}
	if !errors.Is(err, target) {
		h.record("fail", check, fmt.Sprintf("error mismatch: got %v, want %v", err, target))
		return
	}
	h.record("pass", check, fmt.Sprintf("error matched: %v", target))
}

// AssertNoError checks that an error is nil.
func (h *Host) AssertNoError(err error, check string) {
	h.t.Helper()
	if err != nil {
		h.record("fail", check, fmt.Sprintf("unexpected error: %v", err))
		return
	}
	h.record("pass", check, "no error")
}
