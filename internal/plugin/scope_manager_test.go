package plugin

import (
	"testing"
)

func TestPluginScopeLifecycle(t *testing.T) {
	p := Plugin{
		Manifest: Manifest{
			ID:      "test-plugin",
			Version: "1.0.0",
		},
		Fingerprint: "fp-test",
	}

	scope := NewPluginScope(p)
	if !scope.Active() {
		t.Error("scope should be active after creation")
	}
	if scope.Generation.Disposed() {
		t.Error("generation should not be disposed")
	}
	if scope.Generation.PluginID != "test-plugin" {
		t.Errorf("expected plugin ID test-plugin, got %s", scope.Generation.PluginID)
	}
	if scope.Generation.Version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %s", scope.Generation.Version)
	}

	// Register something via the scope.
	dispose, err := RegisterInScope(scope, EffectTool, "my-tool", "tool-value", nil, 0)
	if err != nil {
		t.Fatalf("unexpected register error: %v", err)
	}
	if dispose == nil {
		t.Fatal("expected non-nil disposer")
	}

	// Verify registration.
	entry, ok := scope.Registries.Tools.Get("my-tool")
	if !ok {
		t.Fatal("expected tool to be registered")
	}
	if entry.Value != "tool-value" {
		t.Errorf("expected tool-value, got %v", entry.Value)
	}

	// Dispose scope.
	err = scope.Dispose()
	if err != nil {
		t.Fatalf("unexpected dispose error: %v", err)
	}
	if scope.Active() {
		t.Error("scope should be inactive after dispose")
	}
	if !scope.Generation.Disposed() {
		t.Error("generation should be disposed")
	}

	// Entry should be gone after generation dispose.
	_, ok = scope.Registries.Tools.Get("my-tool")
	if ok {
		t.Error("tool should be removed after scope dispose")
	}
}

func TestPluginScopeDisposeIdempotent(t *testing.T) {
	p := Plugin{
		Manifest: Manifest{ID: "test-plugin", Version: "1.0.0"},
	}
	scope := NewPluginScope(p)

	err1 := scope.Dispose()
	err2 := scope.Dispose()

	if err1 != nil {
		t.Errorf("first dispose: %v", err1)
	}
	if err2 != nil {
		t.Errorf("second dispose: %v", err2)
	}
	if scope.Active() {
		t.Error("scope should be inactive")
	}
}

func TestScopeManagerActivateAndDeactivate(t *testing.T) {
	sm := NewScopeManager()
	p := Plugin{
		Manifest:    Manifest{ID: "p1", Version: "1.0.0"},
		Fingerprint: "fp1",
	}

	scope := sm.Activate(p)
	if scope == nil {
		t.Fatal("expected non-nil scope")
	}
	if !scope.Active() {
		t.Error("scope should be active")
	}
	if sm.ActiveCount() != 1 {
		t.Errorf("expected 1 active scope, got %d", sm.ActiveCount())
	}

	got := sm.Get("p1")
	if got != scope {
		t.Error("Get should return the same scope")
	}

	err := sm.Deactivate("p1")
	if err != nil {
		t.Fatalf("unexpected deactivate error: %v", err)
	}
	if sm.ActiveCount() != 0 {
		t.Errorf("expected 0 active scopes, got %d", sm.ActiveCount())
	}
	if sm.Get("p1") != nil {
		t.Error("Get should return nil after deactivate")
	}
}

func TestScopeManagerActivateReplacesOld(t *testing.T) {
	sm := NewScopeManager()
	p := Plugin{
		Manifest:    Manifest{ID: "p1", Version: "1.0.0"},
		Fingerprint: "fp1",
	}

	scope1 := sm.Activate(p)
	// Activate same plugin again (e.g., upgrade).
	scope2 := sm.Activate(p)

	if scope1.Active() {
		t.Error("old scope should be disposed after re-activation")
	}
	if !scope2.Active() {
		t.Error("new scope should be active")
	}
	if sm.ActiveCount() != 1 {
		t.Errorf("expected 1 active scope, got %d", sm.ActiveCount())
	}
}

func TestScopeManagerDeactivateMissing(t *testing.T) {
	sm := NewScopeManager()
	err := sm.Deactivate("nonexistent")
	if err != nil {
		t.Errorf("deactivating missing plugin should not error: %v", err)
	}
}

func TestScopeManagerList(t *testing.T) {
	sm := NewScopeManager()
	p1 := Plugin{Manifest: Manifest{ID: "p1", Version: "1.0.0"}}
	p2 := Plugin{Manifest: Manifest{ID: "p2", Version: "1.0.0"}}

	sm.Activate(p1)
	sm.Activate(p2)

	list := sm.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 scopes, got %d", len(list))
	}
}

func TestScopeManagerDisposeAll(t *testing.T) {
	sm := NewScopeManager()
	p1 := Plugin{Manifest: Manifest{ID: "p1", Version: "1.0.0"}}
	p2 := Plugin{Manifest: Manifest{ID: "p2", Version: "1.0.0"}}

	sm.Activate(p1)
	sm.Activate(p2)

	err := sm.DisposeAll()
	if err != nil {
		t.Fatalf("unexpected dispose all error: %v", err)
	}
	if sm.ActiveCount() != 0 {
		t.Errorf("expected 0 active scopes, got %d", sm.ActiveCount())
	}
}

func TestRegisterInScopeUnknownKind(t *testing.T) {
	p := Plugin{
		Manifest: Manifest{ID: "test-plugin", Version: "1.0.0"},
	}
	scope := NewPluginScope(p)

	_, err := RegisterInScope(scope, "nonexistent", "key", "val", nil, 0)
	if err == nil {
		t.Fatal("expected error for unknown effect kind")
	}
}

func TestScopeManagerGetMissing(t *testing.T) {
	sm := NewScopeManager()
	if sm.Get("nonexistent") != nil {
		t.Error("Get should return nil for missing plugin")
	}
}

func TestNewGenerationIDUniqueness(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := newGenerationID("test-plugin")
		if ids[id] {
			t.Errorf("duplicate generation ID: %s", id)
		}
		ids[id] = true
	}
}
