package pluginhost

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"
)

// A plugin that declares a transform hook without the granted write permission
// keeps running with the hook stripped and the strip recorded for diagnostics;
// it must not become an active interception point (threat model control #7).
func TestProcessClientStripsHooksBeyondGrantedPermissions(t *testing.T) {
	if os.Getenv("WUU_PLUGINHOST_STRIP_HELPER") == "1" {
		enc := json.NewEncoder(os.Stdout)
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			var req rpcRequest
			_ = json.Unmarshal(scanner.Bytes(), &req)
			_ = enc.Encode(map[string]any{"id": req.ID, "result": InitializeResult{
				Hooks: []Hook{HookSessionStart, HookChatMessage, HookChatRequest, HookShellEnv},
			}})
		}
		return
	}
	client, err := Start(context.Background(), ProcessConfig{
		ID:                 "strip-plugin",
		Command:            os.Args[0],
		Args:               []string{"-test.run=TestProcessClientStripsHooksBeyondGrantedPermissions"},
		Env:                map[string]string{"WUU_PLUGINHOST_STRIP_HELPER": "1"},
		PluginRoot:         t.TempDir(),
		Timeout:            2 * time.Second,
		GrantedPermissions: []string{"session.read"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close(context.Background()) }()

	wantHooks := []Hook{HookSessionStart, HookChatMessage}
	if got := client.Hooks(); !reflect.DeepEqual(got, wantHooks) {
		t.Fatalf("active hooks = %v, want %v", got, wantHooks)
	}
	wantStripped := []Hook{HookChatRequest, HookShellEnv}
	if got := client.Status().StrippedHooks; !reflect.DeepEqual(got, wantStripped) {
		t.Fatalf("stripped hooks = %v, want %v", got, wantStripped)
	}
}

// With no grant at all the plugin still starts, but every permission-gated
// hook is stripped (fail closed does not mean fail to run).
func TestProcessClientWithoutGrantKeepsOnlyMetadataHooks(t *testing.T) {
	if os.Getenv("WUU_PLUGINHOST_STRIP_HELPER") == "1" {
		enc := json.NewEncoder(os.Stdout)
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			var req rpcRequest
			_ = json.Unmarshal(scanner.Bytes(), &req)
			_ = enc.Encode(map[string]any{"id": req.ID, "result": InitializeResult{
				Hooks: []Hook{HookSessionStart, HookChatRequest},
			}})
		}
		return
	}
	client, err := Start(context.Background(), ProcessConfig{
		ID:         "ungranted-plugin",
		Command:    os.Args[0],
		Args:       []string{"-test.run=TestProcessClientWithoutGrantKeepsOnlyMetadataHooks"},
		Env:        map[string]string{"WUU_PLUGINHOST_STRIP_HELPER": "1"},
		PluginRoot: t.TempDir(),
		Timeout:    2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close(context.Background()) }()

	if got := client.Hooks(); !reflect.DeepEqual(got, []Hook{HookSessionStart}) {
		t.Fatalf("active hooks = %v, want only session.start", got)
	}
	if got := client.Status().StrippedHooks; !reflect.DeepEqual(got, []Hook{HookChatRequest}) {
		t.Fatalf("stripped hooks = %v, want chat.request", got)
	}
}
