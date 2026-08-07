package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	pluginpkg "github.com/blueberrycongee/wuu/internal/plugin"
	"github.com/blueberrycongee/wuu/internal/pluginhost"
	"github.com/blueberrycongee/wuu/internal/pluginsettings"
)

func TestPluginHostServicesSettingsOwnershipDefaultsAndFingerprint(t *testing.T) {
	home, workspace := t.TempDir(), t.TempDir()
	item := serviceTestPlugin("demo", "plugin:user:demo", "active-generation")
	if _, err := pluginsettings.Update(home, workspace, item.SubjectID, pluginsettings.ScopeUser, "pending-generation", func(values map[string]json.RawMessage) error {
		values["enabled"] = json.RawMessage(`false`)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	handler := newPluginHostServices(item, workspace, home, nil)

	getRaw, err := handler.HandleHostService(context.Background(), pluginhost.HostServiceSettingsGet, json.RawMessage(`{"key":"enabled"}`))
	if err != nil {
		t.Fatal(err)
	}
	var get pluginhost.SettingsGetResult
	if err := json.Unmarshal(getRaw, &get); err != nil || string(get.Value) != "false" {
		t.Fatalf("get = %s, err = %v", getRaw, err)
	}
	listRaw, err := handler.HandleHostService(context.Background(), pluginhost.HostServiceSettingsList, json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	var list pluginhost.SettingsListResult
	if err := json.Unmarshal(listRaw, &list); err != nil {
		t.Fatal(err)
	}
	if string(list.Entries["enabled"]) != "false" || string(list.Entries["label"]) != `"default"` || len(list.Entries) != 2 {
		t.Fatalf("list = %s", listRaw)
	}
	for _, key := range []string{"other.enabled", "../enabled", "demo.enabled"} {
		if _, err := handler.HandleHostService(context.Background(), pluginhost.HostServiceSettingsGet, mustJSON(t, pluginhost.SettingsGetParams{Key: key})); err == nil {
			t.Fatalf("cross-owner key %q was accepted", key)
		}
	}
	document, err := pluginsettings.Read(home, workspace, item.SubjectID, pluginsettings.ScopeUser)
	if err != nil {
		t.Fatal(err)
	}
	if document.Fingerprint != "pending-generation" {
		t.Fatalf("read-only runtime rewrote pending fingerprint to %q", document.Fingerprint)
	}
}

func TestPluginHostServicesStorageIsolationScopesLimitsAndClose(t *testing.T) {
	home, workspace := t.TempDir(), t.TempDir()
	alpha := newPluginHostServices(serviceTestPlugin("alpha", "plugin:user:alpha", "one"), workspace, home, nil)
	beta := newPluginHostServices(serviceTestPlugin("beta", "plugin:user:beta", "one"), workspace, home, nil)

	callService(t, alpha, pluginhost.HostServiceStorageSet, pluginhost.StorageSetParams{Scope: "workspace", Key: "panel.mode", Value: "focused"}, nil)
	callService(t, alpha, pluginhost.HostServiceStorageSet, pluginhost.StorageSetParams{Scope: "workspace", Key: "second", Value: "two"}, nil)
	var got pluginhost.StorageGetResult
	callService(t, alpha, pluginhost.HostServiceStorageGet, pluginhost.StorageGetParams{Scope: "workspace", Key: "panel.mode"}, &got)
	if got.Value == nil || *got.Value != "focused" {
		t.Fatalf("alpha value = %+v", got.Value)
	}
	for name, handler := range map[string]*pluginHostServices{"other-plugin": beta, "user-scope": alpha} {
		scope := "workspace"
		if name == "user-scope" {
			scope = "user"
		}
		var isolated pluginhost.StorageGetResult
		callService(t, handler, pluginhost.HostServiceStorageGet, pluginhost.StorageGetParams{Scope: scope, Key: "panel.mode"}, &isolated)
		if isolated.Value != nil {
			t.Fatalf("storage leaked to %s: %q", name, *isolated.Value)
		}
	}
	var keys pluginhost.StorageKeysResult
	callService(t, alpha, pluginhost.HostServiceStorageKeys, pluginhost.StorageKeysParams{Scope: "workspace"}, &keys)
	if strings.Join(keys.Keys, ",") != "panel.mode,second" {
		t.Fatalf("keys = %v", keys.Keys)
	}
	callService(t, alpha, pluginhost.HostServiceStorageDelete, pluginhost.StorageDeleteParams{Scope: "workspace", Key: "panel.mode"}, nil)
	if _, err := alpha.HandleHostService(context.Background(), pluginhost.HostServiceStorageSet, mustJSON(t, pluginhost.StorageSetParams{Scope: "workspace", Key: "../escape", Value: "x"})); err == nil {
		t.Fatal("path-like storage key was accepted")
	}
	if _, err := alpha.HandleHostService(context.Background(), pluginhost.HostServiceStorageSet, mustJSON(t, pluginhost.StorageSetParams{Scope: "workspace", Key: "large", Value: strings.Repeat("x", pluginsettings.MaxStateValueBytes+1)})); err == nil {
		t.Fatal("oversize storage value was accepted")
	}
	if _, err := alpha.HandleHostService(context.Background(), pluginhost.HostServiceStorageKeys, json.RawMessage(`{"scope":"project"}`)); err == nil {
		t.Fatal("implicit storage scope was accepted")
	}
	alpha.CloseHostServices()
	if _, err := alpha.HandleHostService(context.Background(), pluginhost.HostServiceStorageKeys, json.RawMessage(`{"scope":"workspace"}`)); err == nil || !strings.Contains(err.Error(), "no longer active") {
		t.Fatalf("closed generation call error = %v", err)
	}
}

func TestProductionProcessClientNestedSettingsAndStorageCalls(t *testing.T) {
	if os.Getenv("WUU_RUNTIME_HOST_SERVICE_HELPER") == "1" {
		runRuntimeHostServiceHelper()
		return
	}
	home, workspace := t.TempDir(), t.TempDir()
	item := serviceTestPlugin("demo", "plugin:user:demo", "generation")
	item.Runtime = &pluginpkg.RuntimeSpec{
		Protocol: pluginhost.CapabilityProtocolName,
		Command:  os.Args[0],
		Args:     []string{"-test.run=TestProductionProcessClientNestedSettingsAndStorageCalls"},
		Env:      map[string]string{"WUU_RUNTIME_HOST_SERVICE_HELPER": "1"},
		Timeout:  3,
	}
	var liveHandler pluginhost.HostServiceHandler
	host, err := buildPluginHost([]pluginpkg.Plugin{item}, workspace, home, map[string]bool{item.ID: true}, func(ctx context.Context, config pluginhost.ProcessConfig) (pluginhost.Client, error) {
		liveHandler = config.HostServiceHandler
		return startPluginClient(ctx, config)
	}, NewPluginTurnRouter())
	if err != nil {
		t.Fatal(err)
	}
	result := pluginhost.ChatMessageOutput{}
	if err := host.Run(context.Background(), pluginhost.HookChatMessage, pluginhost.ChatMessageInput{}, &result); err != nil {
		t.Fatal(err)
	}
	if result.Content != "stored:default" {
		t.Fatalf("nested result = %+v", result)
	}
	if err := host.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := liveHandler.HandleHostService(context.Background(), pluginhost.HostServiceStorageKeys, json.RawMessage(`{"scope":"workspace"}`)); err == nil || !strings.Contains(err.Error(), "no longer active") {
		t.Fatalf("closed ProcessClient left host services live: %v", err)
	}
	document, err := pluginsettings.ReadState(home, workspace, item.SubjectID, pluginsettings.ScopeWorkspace)
	if err != nil || document.Values["nested"] != "stored" {
		t.Fatalf("nested storage = %+v, err = %v", document.Values, err)
	}
}

func TestProductionHostServicesCloseOnGenerationSwap(t *testing.T) {
	if os.Getenv("WUU_RUNTIME_HOST_SERVICE_HELPER") == "1" {
		runRuntimeHostServiceHelper()
		return
	}
	home, workspace := t.TempDir(), t.TempDir()
	item := serviceTestPlugin("old", "plugin:user:old", "old-generation")
	item.Runtime = &pluginpkg.RuntimeSpec{
		Protocol: pluginhost.CapabilityProtocolName,
		Command:  os.Args[0],
		Args:     []string{"-test.run=TestProductionProcessClientNestedSettingsAndStorageCalls"},
		Env:      map[string]string{"WUU_RUNTIME_HOST_SERVICE_HELPER": "1"},
		Timeout:  3,
	}
	var oldHandler pluginhost.HostServiceHandler
	oldHost, err := buildPluginHost([]pluginpkg.Plugin{item}, workspace, home, map[string]bool{item.ID: true}, func(ctx context.Context, config pluginhost.ProcessConfig) (pluginhost.Client, error) {
		oldHandler = config.HostServiceHandler
		return startPluginClient(ctx, config)
	}, NewPluginTurnRouter())
	if err != nil {
		t.Fatal(err)
	}
	old := testPluginGeneration("old", &generationClient{id: "placeholder"})
	_ = old.host.Close(context.Background())
	old.host = oldHost
	session := testGenerationSession(old)
	candidate := testPluginGeneration("candidate", &generationClient{id: "candidate"})
	if err := session.ActivatePluginGeneration(candidate, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := oldHandler.HandleHostService(context.Background(), pluginhost.HostServiceStorageKeys, json.RawMessage(`{"scope":"workspace"}`)); err == nil || !strings.Contains(err.Error(), "no longer active") {
		t.Fatalf("swapped generation left host services live: %v", err)
	}
}

// runRuntimeHostServiceHelper implements a real protocol-v2 process which
// performs two nested plugin-to-host calls while the host awaits hook.invoke.
func runRuntimeHostServiceHelper() {
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	if !scanner.Scan() {
		os.Exit(2)
	}
	var initialize struct {
		ID     string          `json:"id"`
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	if json.Unmarshal(scanner.Bytes(), &initialize) != nil || initialize.Method != "initialize" {
		os.Exit(3)
	}
	var params pluginhost.CapabilityInitializeParams
	if json.Unmarshal(initialize.Params, &params) != nil || len(params.SupportedHostServices) != 8 {
		os.Exit(4)
	}
	initResult := pluginhost.CapabilityInitializeResult{
		InitializeResult: pluginhost.InitializeResult{Hooks: []pluginhost.Hook{pluginhost.HookChatMessage}},
		ProtocolVersion:  pluginhost.CapabilityProtocolVersion,
		RequiredHostServices: []pluginhost.HostServiceDescriptor{
			{ID: string(pluginhost.HostServiceStorageSet), Required: true},
			{ID: string(pluginhost.HostServiceSettingsGet), Required: true},
			{ID: string(pluginhost.HostServiceWorkspaceList), Required: false},
		},
	}
	encodeHelperResponse(encoder, initialize.ID, initResult)
	if !scanner.Scan() {
		os.Exit(5)
	}
	var invoke struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(scanner.Bytes(), &invoke) != nil {
		os.Exit(6)
	}
	_ = encoder.Encode(pluginhost.HostServiceCall{ID: "storage", Method: pluginhost.HostServiceStorageSet, Params: json.RawMessage(`{"scope":"workspace","key":"nested","value":"stored"}`)})
	if !scanner.Scan() {
		os.Exit(7)
	}
	var storage pluginhost.HostServiceResult
	if json.Unmarshal(scanner.Bytes(), &storage) != nil || storage.Error != nil {
		os.Exit(8)
	}
	_ = encoder.Encode(pluginhost.HostServiceCall{ID: "setting", Method: pluginhost.HostServiceSettingsGet, Params: json.RawMessage(`{"key":"label"}`)})
	if !scanner.Scan() {
		os.Exit(9)
	}
	var setting pluginhost.HostServiceResult
	if json.Unmarshal(scanner.Bytes(), &setting) != nil || setting.Error != nil {
		os.Exit(10)
	}
	var value pluginhost.SettingsGetResult
	if json.Unmarshal(setting.Result, &value) != nil {
		os.Exit(11)
	}
	var label string
	if json.Unmarshal(value.Value, &label) != nil {
		os.Exit(12)
	}
	encodeHelperResponse(encoder, invoke.ID, pluginhost.InvokeResult{Output: mustJSONHelper(pluginhost.ChatMessageOutput{Content: "stored:" + label})})
	time.Sleep(10 * time.Second)
}

func encodeHelperResponse(encoder *json.Encoder, id string, value any) {
	_ = encoder.Encode(struct {
		ID     string          `json:"id"`
		Result json.RawMessage `json:"result"`
	}{ID: id, Result: mustJSONHelper(value)})
}

func mustJSONHelper(value any) json.RawMessage {
	raw, _ := json.Marshal(value)
	return raw
}

func serviceTestPlugin(id, subjectID, fingerprint string) pluginpkg.Plugin {
	return pluginpkg.Plugin{Manifest: pluginpkg.Manifest{
		ID: id,
		Settings: map[string]pluginpkg.SettingDefinition{
			id + ".enabled": {Type: pluginpkg.SettingTypeBoolean, Default: true, Scope: pluginpkg.SettingScopeUser},
			id + ".label":   {Type: pluginpkg.SettingTypeString, Default: "default", Scope: pluginpkg.SettingScopeWorkspace},
		},
	}, SubjectID: subjectID, Fingerprint: fingerprint, Root: "."}
}

func callService(t *testing.T, handler *pluginHostServices, method pluginhost.HostServiceMethod, params, result any) {
	t.Helper()
	raw, err := handler.HandleHostService(context.Background(), method, mustJSON(t, params))
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		if err := json.Unmarshal(raw, result); err != nil {
			t.Fatal(err)
		}
	}
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
