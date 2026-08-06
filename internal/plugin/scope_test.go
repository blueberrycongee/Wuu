package plugin

import (
	"errors"
	"testing"
)

func TestGenerationLifecycle(t *testing.T) {
	gen := NewGeneration("gen-1", "test-plugin", "1.0.0", "abc123")

	if gen.ID != "gen-1" {
		t.Errorf("expected ID gen-1, got %s", gen.ID)
	}
	if gen.PluginID != "test-plugin" {
		t.Errorf("expected PluginID test-plugin, got %s", gen.PluginID)
	}
	if gen.Disposed() {
		t.Error("generation should not be disposed immediately")
	}
	if gen.RegistrationCount() != 0 {
		t.Errorf("expected 0 registrations, got %d", gen.RegistrationCount())
	}
}

func TestGenerationRegisterAndDispose(t *testing.T) {
	gen := NewGeneration("gen-1", "test-plugin", "1.0.0", "abc123")

	var cleanup1, cleanup2 bool
	err := gen.Register(func() error { cleanup1 = true; return nil })
	if err != nil {
		t.Fatalf("unexpected register error: %v", err)
	}
	err = gen.Register(func() error { cleanup2 = true; return nil })
	if err != nil {
		t.Fatalf("unexpected register error: %v", err)
	}

	if gen.RegistrationCount() != 2 {
		t.Errorf("expected 2 registrations, got %d", gen.RegistrationCount())
	}

	err = gen.Dispose()
	if err != nil {
		t.Fatalf("unexpected dispose error: %v", err)
	}
	if !gen.Disposed() {
		t.Error("generation should be disposed")
	}
	if !cleanup1 {
		t.Error("cleanup1 should have been called")
	}
	if !cleanup2 {
		t.Error("cleanup2 should have been called")
	}
}

func TestGenerationDisposeReverseOrder(t *testing.T) {
	gen := NewGeneration("gen-1", "test-plugin", "1.0.0", "abc123")

	var order []int
	gen.Register(func() error { order = append(order, 1); return nil })
	gen.Register(func() error { order = append(order, 2); return nil })
	gen.Register(func() error { order = append(order, 3); return nil })

	gen.Dispose()

	if len(order) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(order))
	}
	// LIFO: 3, 2, 1
	if order[0] != 3 || order[1] != 2 || order[2] != 1 {
		t.Errorf("expected reverse order [3 2 1], got %v", order)
	}
}

func TestGenerationDisposeIdempotent(t *testing.T) {
	gen := NewGeneration("gen-1", "test-plugin", "1.0.0", "abc123")

	var count int
	gen.Register(func() error { count++; return nil })

	err1 := gen.Dispose()
	err2 := gen.Dispose()

	if err1 != nil {
		t.Errorf("first dispose should succeed: %v", err1)
	}
	if err2 != nil {
		t.Errorf("second dispose should succeed (idempotent): %v", err2)
	}
	if count != 1 {
		t.Errorf("cleanup should run exactly once, got %d", count)
	}
}

func TestGenerationDisposeCollectsErrors(t *testing.T) {
	gen := NewGeneration("gen-1", "test-plugin", "1.0.0", "abc123")

	e1 := errors.New("cleanup error 1")
	e2 := errors.New("cleanup error 2")
	gen.Register(func() error { return e1 })
	gen.Register(func() error { return e2 })

	err := gen.Dispose()
	if err == nil {
		t.Fatal("expected joined error")
	}
	if !errors.Is(err, e1) {
		t.Errorf("expected e1 in joined error: %v", err)
	}
	if !errors.Is(err, e2) {
		t.Errorf("expected e2 in joined error: %v", err)
	}
}

func TestGenerationRegisterAfterDisposeFails(t *testing.T) {
	gen := NewGeneration("gen-1", "test-plugin", "1.0.0", "abc123")
	gen.Dispose()

	err := gen.Register(func() error { return nil })
	if err == nil {
		t.Fatal("expected error registering on disposed generation")
	}
}

func TestGenerationFingerprint(t *testing.T) {
	gen := NewGeneration("gen-1", "test-plugin", "1.0.0", "sha256:deadbeef")
	if gen.Fingerprint != "sha256:deadbeef" {
		t.Errorf("expected fingerprint sha256:deadbeef, got %s", gen.Fingerprint)
	}
	if gen.Version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %s", gen.Version)
	}
}

func TestEffectKindClassification(t *testing.T) {
	tests := []struct {
		kind      EffectKind
		isAgent   bool
		isDesktop bool
	}{
		{EffectTool, true, false},
		{EffectSystemPrompt, true, false},
		{EffectContext, true, false},
		{EffectProvider, true, false},
		{EffectCompaction, true, false},
		{EffectContinuation, true, false},
		{EffectSubagent, true, false},
		{EffectPermission, true, false},
		{EffectCommand, true, false},
		{EffectView, false, true},
		{EffectTheme, false, true},
		{EffectSetting, false, true},
		{EffectStorage, false, true},
		{EffectLayout, false, true},
		{EffectRenderer, false, true},
		{EffectShell, false, true},
	}

	for _, tt := range tests {
		if tt.kind.IsAgentRuntime() != tt.isAgent {
			t.Errorf("%s.IsAgentRuntime() = %v, want %v", tt.kind, tt.kind.IsAgentRuntime(), tt.isAgent)
		}
		if tt.kind.IsDesktopWorkbench() != tt.isDesktop {
			t.Errorf("%s.IsDesktopWorkbench() = %v, want %v", tt.kind, tt.kind.IsDesktopWorkbench(), tt.isDesktop)
		}
	}
}

func TestNoopDisposer(t *testing.T) {
	if err := NoopDisposer(); err != nil {
		t.Errorf("NoopDisposer should return nil, got %v", err)
	}
}
