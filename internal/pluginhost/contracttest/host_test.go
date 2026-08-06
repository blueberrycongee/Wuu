package contracttest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/blueberrycongee/wuu/internal/pluginhost"
)

func TestHostManifestChecks(t *testing.T) {
	// Create a temporary plugin directory with a valid manifest.
	dir := t.TempDir()

	manifest := `{
  "schema_version": 1,
  "id": "test-plugin",
  "name": "Test Plugin",
  "version": "0.1.0",
  "description": "A test plugin for contract testing"
}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	host := NewHost(t, HostConfig{PluginDir: dir})
	defer host.Close()

	host.AssertManifestExists()
	host.AssertManifestValid()
	host.AssertNoDiagnosticsAbove("fail")
}

func TestHostManifestMissing(t *testing.T) {
	dir := t.TempDir()

	host := NewHost(t, HostConfig{PluginDir: dir, AllowFailures: true})
	defer host.Close()

	host.AssertManifestExists()
	// We expect a failure diagnostic since manifest is missing.
	found := false
	for _, d := range host.diags {
		if d.Level == "fail" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected failure diagnostic for missing manifest")
	}
}

func TestHostManifestMissingField(t *testing.T) {
	dir := t.TempDir()

	manifest := `{"schema_version": 1, "name": "No ID"}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	host := NewHost(t, HostConfig{PluginDir: dir, AllowFailures: true})
	defer host.Close()

	host.AssertManifestValid() // default required: id, schema_version, version
	found := false
	for _, d := range host.diags {
		if d.Level == "fail" && d.Check == "manifest.valid" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected failure diagnostic for missing id field")
	}
}

func TestHostCapabilityDescriptorValidation(t *testing.T) {
	dir := t.TempDir()
	host := NewHost(t, HostConfig{PluginDir: dir, AllowFailures: true})
	defer host.Close()

	valid := pluginhost.CapabilityDescriptor{
		ID: "agent.tool.custom", Kind: "transform", Version: 1,
	}
	host.AssertCapabilityDescriptorValid(valid)

	invalid := pluginhost.CapabilityDescriptor{
		ID: "bad", Kind: "unknown", Version: 0,
	}
	host.AssertCapabilityDescriptorValid(invalid)
}

func TestHostServiceValidation(t *testing.T) {
	dir := t.TempDir()
	host := NewHost(t, HostConfig{PluginDir: dir, AllowFailures: true})
	defer host.Close()

	host.AssertHostServiceSupported(pluginhost.HostServiceStorageGet)
	host.AssertHostServiceSupported(pluginhost.HostServiceSubagentSpawn)
	host.AssertHostServiceSupported("host.unknown.service")
}

func TestHostAssertNoError(t *testing.T) {
	dir := t.TempDir()
	host := NewHost(t, HostConfig{PluginDir: dir, AllowFailures: true})
	defer host.Close()

	host.AssertNoError(nil, "should-pass")
	host.AssertNoError(os.ErrNotExist, "should-fail")
}

func TestHostNoCrashOnNilClient(t *testing.T) {
	dir := t.TempDir()
	host := NewHost(t, HostConfig{PluginDir: dir})
	defer host.Close()

	// These should skip gracefully when client is nil.
	host.AssertCapabilityRegistered("agent.tool.execute")
	host.AssertToolRegistered("my-tool")
	host.AssertHookRegistered(pluginhost.HookSessionStart)
}
