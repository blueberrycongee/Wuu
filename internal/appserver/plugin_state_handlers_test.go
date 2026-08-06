package appserver

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blueberrycongee/wuu/internal/extensions"
	pluginpkg "github.com/blueberrycongee/wuu/internal/plugin"
)

func TestPluginSettingsValidateOwnershipTypeScopeAndGeneration(t *testing.T) {
	srv, item, out := newPluginStateTestServer(t)
	callPluginPackageRPC(t, srv, "default", MethodPluginSettingGet, PluginSettingGetParams{ID: item.SubjectID, Fingerprint: item.Fingerprint, Key: "enabled"})
	defaultResponse := responseByID(t, parseOutput(t, out.String()), "default")
	if defaultResponse["error"] != nil {
		t.Fatalf("default response = %+v", defaultResponse)
	}
	defaultResult := remarshal[PluginSettingResult](t, defaultResponse["result"])
	if defaultResult.Scope != PluginValueScopeWorkspace || string(defaultResult.Value) != "true" {
		t.Fatalf("default result = %+v", defaultResult)
	}

	callPluginPackageRPC(t, srv, "set", MethodPluginSettingSet, PluginSettingSetParams{ID: item.SubjectID, Fingerprint: item.Fingerprint, Key: "enabled", Value: json.RawMessage(`false`)})
	setResult := remarshal[PluginSettingResult](t, responseByID(t, parseOutput(t, out.String()), "set")["result"])
	if string(setResult.Value) != "false" {
		t.Fatalf("set result = %+v", setResult)
	}

	for id, params := range map[string]PluginSettingSetParams{
		"owner": {ID: item.SubjectID, Fingerprint: item.Fingerprint, Key: "other.enabled", Value: json.RawMessage(`true`)},
		"type":  {ID: item.SubjectID, Fingerprint: item.Fingerprint, Key: "enabled", Value: json.RawMessage(`"yes"`)},
		"stale": {ID: item.SubjectID, Fingerprint: "stale", Key: "enabled", Value: json.RawMessage(`true`)},
	} {
		callPluginPackageRPC(t, srv, id, MethodPluginSettingSet, params)
		response := responseByID(t, parseOutput(t, out.String()), id)
		if response["error"] == nil {
			t.Fatalf("%s unexpectedly succeeded: %+v", id, response)
		}
	}
}

func TestPluginStorageIsNamespacedAndScopeIsExplicit(t *testing.T) {
	srv, item, out := newPluginStateTestServer(t)
	for _, scope := range []PluginValueScope{PluginValueScopeUser, PluginValueScopeWorkspace} {
		id := "set-" + string(scope)
		callPluginPackageRPC(t, srv, id, MethodPluginStorageSet, PluginStorageSetParams{ID: item.SubjectID, Fingerprint: item.Fingerprint, Scope: scope, Key: "panel.mode", Value: string(scope)})
		if response := responseByID(t, parseOutput(t, out.String()), id); response["error"] != nil {
			t.Fatalf("%s failed: %+v", id, response)
		}
	}
	for _, scope := range []PluginValueScope{PluginValueScopeUser, PluginValueScopeWorkspace} {
		id := "get-" + string(scope)
		callPluginPackageRPC(t, srv, id, MethodPluginStorageGet, PluginStorageGetParams{ID: item.SubjectID, Fingerprint: item.Fingerprint, Scope: scope, Key: "panel.mode"})
		result := remarshal[PluginStorageResult](t, responseByID(t, parseOutput(t, out.String()), id)["result"])
		if result.Value == nil || *result.Value != string(scope) {
			t.Fatalf("%s result = %+v", scope, result)
		}
	}
	callPluginPackageRPC(t, srv, "bad-key", MethodPluginStorageGet, PluginStorageGetParams{ID: item.SubjectID, Fingerprint: item.Fingerprint, Scope: PluginValueScopeUser, Key: "../escape"})
	if responseByID(t, parseOutput(t, out.String()), "bad-key")["error"] == nil {
		t.Fatal("invalid storage key unexpectedly succeeded")
	}
}

func newPluginStateTestServer(t *testing.T) (*Server, pluginpkg.Plugin, *lockedBuffer) {
	t.Helper()
	rt := newTestRuntime(t, &fakeClient{})
	rt.WuuHome = t.TempDir()
	rt.RootDir = t.TempDir()
	manifestPath := filepath.Join(t.TempDir(), "plugin.json")
	writePluginPackageFile(t, manifestPath, `{
  "id":"state-demo",
  "desktop":{"entry":"desktop.js"},
  "contributes":{"settings":{"enabled":{"type":"boolean","title":"Enabled","default":true,"scope":"workspace","apply":"live"}}}
}`)
	writePluginPackageFile(t, filepath.Join(filepath.Dir(manifestPath), "desktop.js"), `export const activate = () => {};`)
	item, err := pluginpkg.LoadManifest(manifestPath, "user")
	if err != nil {
		t.Fatal(err)
	}
	rt.Plugins = []pluginpkg.Plugin{item}
	rt.ExtensionSettings = &extensions.Settings{}
	if err := rt.ExtensionSettings.RecordGrant(extensions.Grant{SubjectID: item.SubjectID, Fingerprint: item.Fingerprint}); err != nil {
		t.Fatal(err)
	}
	out := &lockedBuffer{}
	return New(rt, out), item, out
}

func requireRPCErrorContains(t *testing.T, response map[string]any, text string) {
	t.Helper()
	if response["error"] == nil || !strings.Contains(fmt.Sprint(response["error"]), text) {
		t.Fatalf("response = %+v", response)
	}
}
