package extensions

import (
	"reflect"
	"strings"
	"testing"
)

// Unknown permission values must fail closed: a manifest requesting a
// permission outside the catalog is rejected rather than silently granted
// (threat model control #5).
func TestNormalizePermissionsRejectsUnknownValues(t *testing.T) {
	for _, input := range [][]string{
		{"session.read", "session.root"},
		{"totally-made-up"},
	} {
		if _, err := NormalizePermissions(input); err == nil || !strings.Contains(err.Error(), "unknown permission") {
			t.Fatalf("NormalizePermissions(%v) = %v, want unknown permission error", input, err)
		}
	}
	if err := ValidatePermissions([]string{"screen.capture", "nope"}); err == nil {
		t.Fatal("ValidatePermissions accepted unknown value")
	}
}

// Legacy aliases canonicalize to catalog names so approvals display and grants
// match on canonical values only.
func TestNormalizePermissionsCanonicalizesAliasesAndDedupes(t *testing.T) {
	got, err := NormalizePermissions([]string{"network", "filesystem.read", " filesystem.write ", "", "network.connect"})
	if err != nil {
		t.Fatalf("NormalizePermissions: %v", err)
	}
	want := []string{PermFilesRead, PermFilesWrite, PermNetworkConnect}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizePermissions = %v, want %v", got, want)
	}
}

func TestIsKnownPermission(t *testing.T) {
	if !IsKnownPermission(PermToolsIntercept) {
		t.Fatal("catalog permission reported unknown")
	}
	if IsKnownPermission("tools.intercept.evil") || IsKnownPermission("") {
		t.Fatal("non-catalog permission reported known")
	}
}

func TestRequiredPermissionsForRuntime(t *testing.T) {
	if got := RequiredPermissionsForRuntime(nil); got != nil {
		t.Fatalf("nil runtime = %v, want nil", got)
	}
	if got := RequiredPermissionsForRuntime(&RuntimeSpec{}); got != nil {
		t.Fatalf("runtime without protocol = %v, want nil", got)
	}
	got := RequiredPermissionsForRuntime(&RuntimeSpec{Protocol: "wuu-plugin-v1", Command: "/bin/x"})
	if !reflect.DeepEqual(got, []string{PermProcessSpawn}) {
		t.Fatalf("runtime permissions = %v", got)
	}
}

func TestRequiredPermissionsForMCPServer(t *testing.T) {
	if got := RequiredPermissionsForMCPServer(nil); got != nil {
		t.Fatalf("nil server = %v, want nil", got)
	}
	if got := RequiredPermissionsForMCPServer(&MCPServerSpec{Command: "/bin/x"}); !reflect.DeepEqual(got, []string{PermProcessSpawn}) {
		t.Fatalf("command server = %v", got)
	}
	if got := RequiredPermissionsForMCPServer(&MCPServerSpec{URL: "http://x"}); !reflect.DeepEqual(got, []string{PermNetworkConnect}) {
		t.Fatalf("url server = %v", got)
	}
	got := RequiredPermissionsForMCPServer(&MCPServerSpec{Command: "/bin/x", URL: "http://x"})
	if !reflect.DeepEqual(got, []string{PermProcessSpawn, PermNetworkConnect}) {
		t.Fatalf("command+url server = %v", got)
	}
}

// Prompt hooks observe and rewrite payloads; command hooks spawn processes.
// The mapping is host-owned — manifest text cannot shrink it.
func TestRequiredPermissionsForHook(t *testing.T) {
	if got := RequiredPermissionsForHook(nil); got != nil {
		t.Fatalf("nil entry = %v, want nil", got)
	}
	if got := RequiredPermissionsForHook(&HookEntry{Type: "command", Command: "/bin/x"}); !reflect.DeepEqual(got, []string{PermProcessSpawn}) {
		t.Fatalf("command hook = %v", got)
	}
	got := RequiredPermissionsForHook(&HookEntry{Type: "prompt", Prompt: "rewrite this"})
	want := []string{PermSessionRead, PermSessionWrite}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("prompt hook = %v, want %v", got, want)
	}
	if got := RequiredPermissionsForHook(&HookEntry{Type: "prompt"}); got != nil {
		t.Fatalf("prompt hook without prompt = %v, want nil", got)
	}
}

func TestRequiredPermissionsForCommand(t *testing.T) {
	if got := RequiredPermissionsForCommand(nil); got != nil {
		t.Fatalf("nil command = %v, want nil", got)
	}
	if got := RequiredPermissionsForCommand(&CommandSpec{Kind: CommandKindRuntimeAction}); !reflect.DeepEqual(got, []string{PermCommandsExecute}) {
		t.Fatalf("runtime_action = %v", got)
	}
	if got := RequiredPermissionsForCommand(&CommandSpec{Kind: CommandKindPromptTemplate}); got != nil {
		t.Fatalf("prompt_template = %v, want nil (host-rendered)", got)
	}
}

// The package union is what the grant must cover: declared permissions plus
// every derived surface requirement, deduplicated and sorted.
func TestRequiredPermissionsForPackageSpecUnion(t *testing.T) {
	spec := PackageSpec{
		Runtime:    &RuntimeSpec{Protocol: "wuu-plugin-v1", Command: "/bin/x"},
		MCPServers: map[string]MCPServerSpec{"web": {URL: "http://x"}},
		Hooks: map[string][]HookEntry{
			"PreToolUse": {{Type: "prompt", Prompt: "p"}},
		},
		Commands: []CommandSpec{{ID: "a", Kind: CommandKindPromptTemplate}},
	}
	got := RequiredPermissionsForPackageSpec(spec)
	want := []string{PermNetworkConnect, PermProcessSpawn, PermSessionRead, PermSessionWrite}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("package union = %v, want %v", got, want)
	}

	spec.RequestedPermissions = []string{PermScreenCapture}
	got = RequestedPermissionUnion(spec)
	want = []string{PermNetworkConnect, PermProcessSpawn, PermScreenCapture, PermSessionRead, PermSessionWrite}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("requested union = %v, want %v", got, want)
	}
}

// Skill directories are declarative data read by the host; they must never
// imply execution authority by themselves.
func TestRequiredPermissionsForSkillsIsAlwaysEmpty(t *testing.T) {
	if got := RequiredPermissionsForSkills([]string{"skills", "more"}); got != nil {
		t.Fatalf("skills = %v, want nil", got)
	}
}
