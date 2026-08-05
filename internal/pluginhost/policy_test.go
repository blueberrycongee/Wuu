package pluginhost

import (
	"reflect"
	"testing"

	"github.com/blueberrycongee/wuu/internal/extensions"
)

// The host-owned mapping is the authority: interception-capable hooks require
// closed-catalog permissions, lifecycle metadata hooks stay free.
func TestRequiredPermissionsForProtocolHook(t *testing.T) {
	cases := map[Hook][]string{
		HookChatMessage:       {extensions.PermSessionRead},
		HookChatRequest:       {extensions.PermSessionRead, extensions.PermSessionWrite},
		HookToolDefinition:    {extensions.PermToolsDefine},
		HookToolExecuteBefore: {extensions.PermToolsIntercept},
		HookToolExecuteAfter:  {extensions.PermToolsIntercept},
		HookShellEnv:          {extensions.PermShellEnv},
		HookSessionStart:      nil,
		HookSessionStop:       nil,
	}
	for hook, want := range cases {
		if got := RequiredPermissionsForProtocolHook(hook); !reflect.DeepEqual(got, want) {
			t.Fatalf("RequiredPermissionsForProtocolHook(%q) = %v, want %v", hook, got, want)
		}
	}
}

func TestFilterHooksByGrantedPermissions(t *testing.T) {
	declared := []Hook{HookSessionStart, HookChatMessage, HookChatRequest, HookShellEnv}

	kept, stripped := FilterHooksByGrantedPermissions(declared, nil)
	if !reflect.DeepEqual(kept, []Hook{HookSessionStart}) {
		t.Fatalf("nil grant kept = %v, want only session.start", kept)
	}
	if !reflect.DeepEqual(stripped, []Hook{HookChatMessage, HookChatRequest, HookShellEnv}) {
		t.Fatalf("nil grant stripped = %v", stripped)
	}

	// Read without write: observation passes, transform is stripped.
	kept, stripped = FilterHooksByGrantedPermissions(declared, []string{extensions.PermSessionRead})
	if !reflect.DeepEqual(kept, []Hook{HookSessionStart, HookChatMessage}) {
		t.Fatalf("read-only kept = %v", kept)
	}
	if !reflect.DeepEqual(stripped, []Hook{HookChatRequest, HookShellEnv}) {
		t.Fatalf("read-only stripped = %v", stripped)
	}

	kept, stripped = FilterHooksByGrantedPermissions(declared, extensions.CatalogPermissions())
	if !reflect.DeepEqual(kept, declared) {
		t.Fatalf("full catalog kept = %v, want %v", kept, declared)
	}
	if len(stripped) != 0 {
		t.Fatalf("full catalog stripped = %v, want none", stripped)
	}
}
